package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// AuditWriter is the subset of the store the audit middleware needs.
type AuditWriter interface {
	InsertAuditEvent(ctx context.Context, e store.AuditEvent) error
}

const (
	// auditWriteTimeout bounds each background audit insert.
	auditWriteTimeout = 5 * time.Second
	// auditMaxInFlight caps concurrent background inserts. When the cap is
	// hit (e.g. the database is stalled) new rows are dropped with a warning
	// rather than queueing goroutines without bound.
	auditMaxInFlight = 64
	// auditMaxBodyDrain caps how much of an unread request body the
	// middleware reads after the handler returns to finish the body hash.
	auditMaxBodyDrain = 1 << 20
)

// Audit returns middleware that records one audit row for every mutating
// (non GET/HEAD/OPTIONS) request whose path starts with pathPrefix, once the
// handler has returned (issue #122). Install it outside auth, content-type
// and rate-limit middleware so their rejections are audited too.
//
//   - Failed requests are audited too: the row carries the response status,
//     and a panicking handler is recorded as 500 before the panic propagates
//     to Recoverer.
//   - Only a SHA-256 of the request body is stored, never the body.
//   - The write happens in the background with its own timeout, so a slow or
//     failing audit store never delays or fails the request.
func Audit(writer AuditWriter, logger *slog.Logger, pathPrefix string) func(http.Handler) http.Handler {
	inFlight := make(chan struct{}, auditMaxInFlight)

	record := func(e store.AuditEvent) {
		select {
		case inFlight <- struct{}{}:
		default:
			if logger != nil {
				logger.Warn("audit: too many pending writes, dropping row", "action", e.Action)
			}
			return
		}
		go func() {
			defer func() { <-inFlight }()
			ctx, cancel := context.WithTimeout(context.Background(), auditWriteTimeout)
			defer cancel()
			if err := writer.InsertAuditEvent(ctx, e); err != nil && logger != nil {
				logger.Error("audit: write failed", "err", err, "action", e.Action)
			}
		}()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isMutating(r.Method) || !strings.HasPrefix(r.URL.Path, pathPrefix) {
				next.ServeHTTP(w, r)
				return
			}

			body := &hashingReader{h: sha256.New()}
			if r.Body != nil && r.Body != http.NoBody {
				body.r = r.Body
				r.Body = body
			}
			ww := chiMiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			at := time.Now().UTC()

			panicked := true
			defer func() {
				status := ww.Status()
				if panicked {
					status = http.StatusInternalServerError
				} else if status == 0 {
					status = http.StatusOK
				}
				body.drain()
				record(auditEventFor(r, status, body.sum(), at))
			}()

			next.ServeHTTP(ww, r)
			panicked = false
		})
	}
}

func isMutating(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

// auditEventFor builds the audit row. It runs after routing, so the chi
// route pattern and URL params are resolved. When a middleware rejected the
// request before it reached a leaf route (rate limit, content type), the
// pattern is empty or a mount wildcard, so the concrete path is used.
func auditEventFor(r *http.Request, status int, bodyHash string, at time.Time) store.AuditEvent {
	pattern := r.URL.Path
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if p := rctx.RoutePattern(); p != "" && !strings.HasSuffix(p, "*") {
			pattern = normalizePattern(p)
		}
	}
	return store.AuditEvent{
		Actor:           Actor(r),
		Action:          r.Method + " " + pattern,
		ResourceType:    resourceType(pattern),
		ResourceID:      resourceID(r),
		IP:              clientIP(r),
		UserAgent:       r.UserAgent(),
		RequestBodyHash: bodyHash,
		Status:          status,
		At:              at,
	}
}

// resourceType is the first path segment after the API prefix, skipping the
// admin namespace: "/api/v1/contracts/{id}" -> "contracts",
// "/api/v1/admin/keys/{id}" -> "keys".
func resourceType(pattern string) string {
	p := strings.TrimPrefix(pattern, "/api/v1/")
	p = strings.TrimPrefix(p, "admin/")
	p = strings.TrimPrefix(p, "/")
	if i := strings.IndexByte(p, '/'); i >= 0 {
		p = p[:i]
	}
	return p
}

// resourceID returns the first route parameter that identifies the target
// resource. Create requests carry the ID in the body, which is never read
// here, so they leave it empty.
func resourceID(r *http.Request) string {
	for _, key := range []string{"id", "contractId"} {
		if v := chi.URLParam(r, key); v != "" {
			return v
		}
	}
	return ""
}

// Actor names the caller of a request for attribution (audit rows, the
// added_by of watched accounts). It prefers explicit user identity headers
// and falls back to a non-reversible API key fingerprint, then "anonymous".
func Actor(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-User-ID")); v != "" {
		return v
	}
	if v := strings.TrimSpace(r.Header.Get("X-GitHub-Login")); v != "" {
		return "github:" + v
	}
	if v := strings.TrimSpace(r.Header.Get("X-GitHub-ID")); v != "" {
		return "github:" + v
	}
	if v := strings.TrimSpace(r.Header.Get("X-User-Sub")); v != "" {
		return v
	}
	if token := extractAPIKey(r); token != "" {
		return "apikey:" + store.HashKey(token)[:12]
	}
	return "anonymous"
}

// hashingReader feeds everything the handler reads from the request body
// into a SHA-256 so the body is hashed without being buffered or stored.
type hashingReader struct {
	r io.ReadCloser
	h hash.Hash
}

func (b *hashingReader) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.h.Write(p[:n])
	return n, err
}

func (b *hashingReader) Close() error { return b.r.Close() }

// drain reads (and hashes) whatever the handler left unread, up to a cap.
func (b *hashingReader) drain() {
	if b.r == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(b, auditMaxBodyDrain))
}

func (b *hashingReader) sum() string {
	return hex.EncodeToString(b.h.Sum(nil))
}
