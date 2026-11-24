package middleware

// Idempotency-Key middleware (issue #784).
//
// A POST request that carries an Idempotency-Key header is processed at most
// once. The first request claims the key, executes normally and stores the
// response. Any subsequent request with the same key and identical body
// replays the stored response without executing the handler. A retry with a
// different body returns 422 so the client knows something changed.
//
// The key is scoped to the authenticated API key: two callers with different
// API keys cannot interfere with each other.
//
// Keys expire after 24 hours.  The store's DeleteExpiredIdempotencyKeys
// method must be called periodically to clean up.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// IdempotencyKeyHeader is the HTTP header clients send to enable idempotency.
const IdempotencyKeyHeader = "Idempotency-Key"

// idempotencyKeyMaxLen is the maximum accepted key length.
const idempotencyKeyMaxLen = 255

// secretReplayStatus and secretReplayBody are what a replay of a response that
// carried a secret (API key, signing secret, ...) returns instead of the
// original body. The original body is never stored.
const secretReplayStatus = http.StatusConflict
const secretReplayBody = `{"error":{"code":"IDEMPOTENT_REPLAY_UNAVAILABLE","message":"the original request succeeded, but its response contained a secret that is shown only once and cannot be replayed; create a new resource with a new Idempotency-Key if you lost it"}}`

type idempotencyStateKey struct{}

// idempotencyState lets a handler opt out of response persistence.
type idempotencyState struct{ redact bool }

// RedactIdempotentResponse tells the Idempotency middleware that the response
// of the current request contains a secret. A successful response is then
// neither stored nor replayed: only a fixed 409 message is persisted. Handlers
// that return an API key, token or signing secret must call this. It is a
// no-op when the request is not under Idempotency-Key handling.
func RedactIdempotentResponse(r *http.Request) {
	if st, ok := r.Context().Value(idempotencyStateKey{}).(*idempotencyState); ok {
		st.redact = true
	}
}

// Idempotency returns a middleware that enforces Idempotency-Key semantics on
// POST routes. It reads the authenticated API key ID from the request context
// (set by the Identity middleware) and uses store to store/replay responses.
func Idempotency(s store.IdempotencyStore, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handleIdempotency(s, logger, next, w, r)
		})
	}
}

func handleIdempotency(s store.IdempotencyStore, logger *slog.Logger, next http.Handler, w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.Header.Get(IdempotencyKeyHeader))
	if key == "" {
		next.ServeHTTP(w, r)
		return
	}
	if len(key) > idempotencyKeyMaxLen {
		http.Error(w, `{"error":"Idempotency-Key too long (max 255 characters)"}`, http.StatusUnprocessableEntity)
		return
	}

	// Require an authenticated API key; skip idempotency for anonymous/session callers.
	identity, ok := IdentityFromRequest(r)
	if !ok || identity.APIKey == nil {
		next.ServeHTTP(w, r)
		return
	}
	apiKeyID := identity.APIKey.ID

	// Buffer the body so we can fingerprint it and also pass it to the handler.
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB cap
	if err != nil {
		http.Error(w, `{"error":"failed to read request body"}`, http.StatusInternalServerError)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	sum := sha256.Sum256(body)
	fingerprint := hex.EncodeToString(sum[:])

	rec, fresh, err := s.ClaimIdempotencyKey(r.Context(), apiKeyID, key, fingerprint)
	if err != nil {
		logger.Error("claim idempotency key", "err", err)
		next.ServeHTTP(w, r) // fail open
		return
	}

	if !fresh {
		if rec.BodyFingerprint != fingerprint {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"error":"idempotency key reused with a different request body"}`))
			return
		}
		if rec.InFlight {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"a request with this idempotency key is already in progress"}`))
			return
		}
		// Replay the stored response.
		for k, v := range rec.ResponseHeaders {
			w.Header().Set(k, v)
		}
		w.Header().Set("Idempotent-Replayed", "true")
		w.WriteHeader(rec.ResponseStatus)
		_, _ = w.Write([]byte(rec.ResponseBody))
		return
	}

	// Fresh claim: execute the handler and capture its response.
	state := &idempotencyState{}
	r = r.WithContext(context.WithValue(r.Context(), idempotencyStateKey{}, state))

	rw := &responseCapture{ResponseWriter: w, status: http.StatusOK}
	next.ServeHTTP(rw, r)

	status := rw.status
	headers := map[string]string{"Content-Type": rw.Header().Get("Content-Type")}
	bodyToStore := rw.buf.String()
	if state.redact && status >= 200 && status < 300 {
		// Never persist a secret: keep only a fixed message for replays.
		status = secretReplayStatus
		headers = map[string]string{"Content-Type": "application/json"}
		bodyToStore = secretReplayBody
	}
	if err := s.CompleteIdempotencyKey(context.WithoutCancel(r.Context()), apiKeyID, key, status, headers, bodyToStore); err != nil {
		logger.Error("complete idempotency key", "err", err)
	}
}

// responseCapture wraps http.ResponseWriter, capturing the status and body.
type responseCapture struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
}

func (rc *responseCapture) WriteHeader(code int) {
	rc.status = code
	rc.ResponseWriter.WriteHeader(code)
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	rc.buf.Write(b)
	return rc.ResponseWriter.Write(b)
}
