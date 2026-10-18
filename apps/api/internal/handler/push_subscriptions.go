package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Web Push subscription persistence (issue #592).
//
// The browser-side flow (apps/web) posts a PushSubscription JSON
// ({endpoint, keys:{auth, p256dh}}) to the Next route, which proxies it here.
// The API owns the durable store; subscriptions survive server restarts and
// the endpoint column is the idempotency key — re-subscribing refreshes the
// existing row instead of duplicating it.
//
// Security notes: endpoint, p256dh and auth are push-channel credentials.
// They are never written to logs (the logger only receives ids and counts),
// and the list endpoint that returns the key material for alert fan-out is
// scope+role gated in the router.

// ---- limits -----------------------------------------------------------------

const (
	// maxPushEndpointLen matches the push_subscriptions CHECK constraint.
	// Push-service endpoints (FCM, Mozilla autopush, ...) are long URLs but
	// well under this bound.
	maxPushEndpointLen = 2048
	// maxPushUserIDLen bounds the optional owner binding.
	maxPushUserIDLen = 128
	// maxPushFiltersBytes bounds the subscriber's filter document so one
	// browser cannot park megabytes of JSON in our row (the overall request
	// is additionally capped by the global BodyLimit middleware).
	maxPushFiltersBytes = 4096
)

// ---- request/response types -------------------------------------------------

type createPushSubscriptionRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		Auth   string `json:"auth"`
		P256dh string `json:"p256dh"`
	} `json:"keys"`
	// UserID optionally binds the subscription to a dashboard user.
	UserID string `json:"user_id"`
	// Filters is an opaque JSON object of delivery preferences, e.g.
	// {"contract_ids":[...],"severities":["Critical"]}.
	Filters json.RawMessage `json:"filters"`
}

type pushSubscriptionResponse struct {
	ID        string    `json:"id"`
	Endpoint  string    `json:"endpoint"`
	UserID    string    `json:"user_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// pushSubscriptionFull is the shape returned only by the authenticated list
// endpoint: the sender needs the key material, so it is the sole surface that
// carries p256dh/auth, and the router gates it behind
// ScopeReadWatchdog + contributor.
type pushSubscriptionFull struct {
	pushSubscriptionResponse
	P256dhKey string          `json:"p256dh_key"`
	AuthKey   string          `json:"auth_key"`
	Filters   json.RawMessage `json:"filters,omitempty"`
}

// ---- validation ---------------------------------------------------------------

// validatePushSubscription checks every field of a browser PushSubscription
// and returns the normalised store row. It is a separate function so the
// unit tests exercise the rules directly.
//
// Rules enforced (mirrored by CHECK constraints in migration 000039):
//   - endpoint: absolute https URL, non-empty host, <= 2048 chars.
//   - keys.p256dh: base64url of a 65-byte uncompressed P-256 point (0x04
//     prefix), exactly what PushSubscription.toJSON() produces.
//   - keys.auth: base64url of a 16..32-byte auth secret (RFC 8291 defines
//     16; browsers may lengthen it).
//   - user_id: <= 128 chars.
//   - filters: a JSON object of at most 4096 serialised bytes, when present.
func validatePushSubscription(req createPushSubscriptionRequest) (store.PushSubscription, error) {
	var sub store.PushSubscription

	if len(req.Endpoint) == 0 || len(req.Endpoint) > maxPushEndpointLen {
		return sub, errors.New("endpoint must be a non-empty https URL of at most 2048 characters")
	}
	u, err := url.Parse(req.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return sub, errors.New("endpoint must be a valid https URL")
	}

	p256, err := decodePushKey(req.Keys.P256dh)
	if err != nil || len(p256) != 65 || p256[0] != 0x04 {
		return sub, errors.New("keys.p256dh must be the base64url-encoded 65-byte subscriber public key")
	}
	auth, err := decodePushKey(req.Keys.Auth)
	if err != nil || len(auth) < 16 || len(auth) > 32 {
		return sub, errors.New("keys.auth must be a base64url-encoded auth secret of 16 to 32 bytes")
	}

	if len(req.UserID) > maxPushUserIDLen {
		return sub, fmt.Errorf("user_id must be at most %d characters", maxPushUserIDLen)
	}

	filters := []byte(nil)
	if len(req.Filters) > 0 {
		if len(req.Filters) > maxPushFiltersBytes {
			return sub, fmt.Errorf("filters must be at most %d bytes", maxPushFiltersBytes)
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(req.Filters, &obj); err != nil {
			return sub, errors.New("filters must be a JSON object")
		}
		filters = req.Filters
	}

	return store.PushSubscription{
		ID:        newID(),
		Endpoint:  req.Endpoint,
		P256dhKey: req.Keys.P256dh,
		AuthKey:   req.Keys.Auth,
		UserID:    req.UserID,
		Filters:   filters,
	}, nil
}

// decodePushKey accepts the base64url encoding produced by
// PushSubscription.toJSON() (unpadded) and, defensively, the padded form.
// It never decodes standard-alphabet '+'/'/' keys, which browsers do not
// emit; accepting an unexpected alphabet here would let a caller smuggle
// URL-ish material into a column we later treat as opaque.
func decodePushKey(s string) ([]byte, error) {
	if s == "" {
		return nil, errors.New("empty key")
	}
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.URLEncoding.DecodeString(s)
}

// ---- handlers -----------------------------------------------------------------

// CreatePushSubscription handles POST /api/v1/push/subscriptions.
//
// Idempotent by endpoint: a repeat subscription with the same endpoint
// updates the stored row and answers 200 (created=false); a genuinely new
// endpoint answers 201. The response echoes the subscriber's own endpoint
// only — never the key material they just sent.
func (h *Handler) CreatePushSubscription(w http.ResponseWriter, r *http.Request) {
	var req createPushSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	sub, err := validatePushSubscription(req)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
		return
	}
	stored, created, err := h.Store.UpsertPushSubscription(r.Context(), sub)
	if err != nil {
		// err is logged without any field of the subscription except the id.
		h.Logger.Error("create push subscription", "id", stored.ID, "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to store subscription")
		return
	}
	resp := pushSubscriptionResponse{
		ID:        stored.ID,
		Endpoint:  stored.Endpoint,
		UserID:    stored.UserID,
		CreatedAt: stored.CreatedAt,
		UpdatedAt: stored.UpdatedAt,
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, resp)
}

// ListPushSubscriptions handles GET /api/v1/push/subscriptions?user_id=...
//
// This is the surface the alert fan-out (and, in development, the Next
// internal send route) reads from, so it returns the full key material. The
// router puts it behind ScopeReadWatchdog + the contributor role precisely
// because of that; it must never be exposed anonymously.
func (h *Handler) ListPushSubscriptions(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	subs, err := h.Store.ListPushSubscriptions(r.Context(), userID)
	if err != nil {
		h.Logger.Error("list push subscriptions", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list subscriptions")
		return
	}
	resp := make([]pushSubscriptionFull, len(subs))
	for i, s := range subs {
		var filters json.RawMessage
		if len(s.Filters) > 0 && string(s.Filters) != "{}" {
			filters = json.RawMessage(s.Filters)
		}
		resp[i] = pushSubscriptionFull{
			pushSubscriptionResponse: pushSubscriptionResponse{
				ID:        s.ID,
				Endpoint:  s.Endpoint,
				UserID:    s.UserID,
				CreatedAt: s.CreatedAt,
				UpdatedAt: s.UpdatedAt,
			},
			P256dhKey: s.P256dhKey,
			AuthKey:   s.AuthKey,
			Filters:   filters,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"subscriptions": resp})
}

// DeletePushSubscription handles DELETE /api/v1/push/subscriptions/{id}.
func (h *Handler) DeletePushSubscription(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.Store.DeletePushSubscription(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "subscription not found")
		return
	}
	if err != nil {
		h.Logger.Error("delete push subscription", "id", id, "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to delete subscription")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UnsubscribePush handles DELETE /api/v1/push/subscriptions?endpoint=...
//
// The browser unsubscribe flow knows only its own endpoint (that is the
// shape apps/web/app/api/push/subscribe/route.ts has always used), and the
// sender calls it on HTTP 410 Gone. The endpoint is an unguessable
// push-service URL, so possession of it is the capability; no credential is
// required. Unknown endpoints answer 404.
func (h *Handler) UnsubscribePush(w http.ResponseWriter, r *http.Request) {
	endpoint := r.URL.Query().Get("endpoint")
	if endpoint == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "endpoint query parameter is required")
		return
	}
	err := h.Store.DeletePushSubscriptionByEndpoint(r.Context(), endpoint)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "subscription not found")
		return
	}
	if err != nil {
		h.Logger.Error("unsubscribe push subscription", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to delete subscription")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
