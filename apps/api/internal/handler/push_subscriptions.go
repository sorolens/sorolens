package handler

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Persistent Web Push subscriptions (issue #592).
//
// The dashboard used to keep subscriptions in an in-memory Map inside the Next
// route, so every cold start forgot every device. These endpoints store them
// in Postgres, scoped to the verified session user. The internal dispatch
// routes let the Next route fan out pushes from persisted subscriptions; they
// authenticate with the shared PUSH_INTERNAL_SECRET, never a user session.

const maxPushEndpointBytes = 2048

type pushKeys struct {
	Auth   string `json:"auth"`
	P256DH string `json:"p256dh"`
}

type pushSubscriptionRequest struct {
	Endpoint string         `json:"endpoint"`
	Keys     pushKeys       `json:"keys"`
	Filters  map[string]any `json:"filters,omitempty"`
}

type pushSubscriptionResponse struct {
	ID        string         `json:"id"`
	Endpoint  string         `json:"endpoint"`
	Filters   map[string]any `json:"filters"`
	CreatedAt time.Time      `json:"created_at"`
}

type pushDispatchResponse struct {
	Endpoint string         `json:"endpoint"`
	Keys     pushKeys       `json:"keys"`
	Filters  map[string]any `json:"filters"`
}

// validPushEndpoint accepts the same https push services the browser route
// validates: no credentials, port, query tricks or fragment, and a hostname
// owned by a known push service.
func validPushEndpoint(endpoint string) bool {
	if endpoint == "" || len(endpoint) > maxPushEndpointBytes {
		return false
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	if u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" || u.Host == "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case host == "fcm.googleapis.com",
		host == "android.googleapis.com",
		host == "push.services.mozilla.com",
		host == "updates.push.services.mozilla.com":
		return true
	case strings.HasSuffix(host, ".push.apple.com"):
		return true
	case strings.HasSuffix(host, ".notify.windows.com"):
		return true
	default:
		return false
	}
}

func validPushKey(value string, size int) bool {
	if value == "" || !base64URLAlphabet(value) {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		// Accept padded base64url as well; browsers normally send unpadded.
		decoded, err = base64.URLEncoding.DecodeString(value)
		if err != nil {
			return false
		}
	}
	if len(decoded) != size {
		return false
	}
	if size == 65 && decoded[0] != 4 {
		return false
	}
	return true
}

func base64URLAlphabet(value string) bool {
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '=':
		default:
			return false
		}
	}
	return true
}

// validatePushFilters keeps the filter object small and predictable: only the
// fields the dispatcher understands are accepted.
func validatePushFilters(filters map[string]any) (map[string]any, error) {
	if len(filters) == 0 {
		return map[string]any{}, nil
	}
	out := make(map[string]any, len(filters))
	for key, value := range filters {
		switch key {
		case "severity":
			severity, ok := value.(string)
			if !ok || (severity != "Info" && severity != "Warning" && severity != "Critical") {
				return nil, errors.New("severity must be Info, Warning or Critical")
			}
			out[key] = severity
		case "contract_id":
			contract, ok := value.(string)
			if !ok || len(contract) > 128 {
				return nil, errors.New("contract_id must be a string of at most 128 characters")
			}
			out[key] = contract
		case "network":
			network, ok := value.(string)
			if !ok || len(network) > 32 {
				return nil, errors.New("network must be a string of at most 32 characters")
			}
			out[key] = network
		default:
			return nil, errors.New("unknown filter " + key)
		}
	}
	return out, nil
}

func pushFromRequest(req pushSubscriptionRequest, userID string) (store.PushSubscription, error) {
	req.Endpoint = strings.TrimSpace(req.Endpoint)
	if !validPushEndpoint(req.Endpoint) {
		return store.PushSubscription{}, errors.New("invalid push endpoint")
	}
	req.Keys.Auth = strings.TrimSpace(req.Keys.Auth)
	req.Keys.P256DH = strings.TrimSpace(req.Keys.P256DH)
	if !validPushKey(req.Keys.Auth, 16) || !validPushKey(req.Keys.P256DH, 65) {
		return store.PushSubscription{}, errors.New("invalid push keys")
	}
	filters, err := validatePushFilters(req.Filters)
	if err != nil {
		return store.PushSubscription{}, err
	}
	return store.PushSubscription{
		ID:        newID(),
		Endpoint:  req.Endpoint,
		AuthKey:   req.Keys.Auth,
		P256DHKey: req.Keys.P256DH,
		UserID:    userID,
		Filters:   filters,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// CreatePushSubscription handles POST /api/v1/push/subscriptions.
func (h *Handler) CreatePushSubscription(w http.ResponseWriter, r *http.Request) {
	userID := middleware.VerifiedUserID(r)
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
		return
	}
	var req pushSubscriptionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	sub, err := pushFromRequest(req, userID)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
		return
	}
	saved, err := h.Store.UpsertPushSubscription(r.Context(), sub)
	if err != nil {
		h.Logger.Error("upsert push subscription", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to save push subscription")
		return
	}
	if !saved {
		writeError(w, r, http.StatusConflict, CodeConflict, "push endpoint is registered to another user")
		return
	}
	writeJSON(w, http.StatusCreated, pushSubscriptionResponse{
		ID:        sub.ID,
		Endpoint:  sub.Endpoint,
		Filters:   sub.Filters,
		CreatedAt: sub.CreatedAt,
	})
}

// ListPushSubscriptions handles GET /api/v1/push/subscriptions. It returns the
// caller's own subscriptions and never the key material.
func (h *Handler) ListPushSubscriptions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.VerifiedUserID(r)
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
		return
	}
	subs, err := h.Store.ListPushSubscriptions(r.Context(), userID)
	if err != nil {
		h.Logger.Error("list push subscriptions", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list push subscriptions")
		return
	}
	resp := make([]pushSubscriptionResponse, 0, len(subs))
	for _, sub := range subs {
		resp = append(resp, pushSubscriptionResponse{
			ID:        sub.ID,
			Endpoint:  sub.Endpoint,
			Filters:   sub.Filters,
			CreatedAt: sub.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"subscriptions": resp})
}

// DeletePushSubscription handles DELETE /api/v1/push/subscriptions with an
// `{"endpoint": "..."}` body. Deleting an already-absent endpoint succeeds:
// unsubscribe is idempotent and must not leak another user's subscriptions.
func (h *Handler) DeletePushSubscription(w http.ResponseWriter, r *http.Request) {
	userID := middleware.VerifiedUserID(r)
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
		return
	}
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	if !validPushEndpoint(strings.TrimSpace(req.Endpoint)) {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid push endpoint")
		return
	}
	if _, err := h.Store.DeletePushSubscription(r.Context(), userID, req.Endpoint); err != nil {
		h.Logger.Error("delete push subscription", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to delete push subscription")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// validDispatchSecret compares the request's X-Push-Secret with the configured
// PUSH_INTERNAL_SECRET in constant time. An unconfigured secret disables the
// internal routes instead of accepting an empty header.
func validDispatchSecret(r *http.Request) bool {
	expected := os.Getenv("PUSH_INTERNAL_SECRET")
	supplied := r.Header.Get("X-Push-Secret")
	if expected == "" || supplied == "" {
		return false
	}
	a := sha256.Sum256([]byte(supplied))
	b := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}

// ListPushSubscriptionsForDispatch handles
// GET /api/v1/push/subscriptions/dispatch for the internal alerting pipeline.
// It includes the key material the sender needs and is gated by the shared
// dispatch secret, never a user session.
func (h *Handler) ListPushSubscriptionsForDispatch(w http.ResponseWriter, r *http.Request) {
	if !validDispatchSecret(r) {
		writeError(w, r, http.StatusUnauthorized, CodeUnauthorized, "invalid dispatch secret")
		return
	}
	subs, err := h.Store.ListPushSubscriptions(r.Context(), "")
	if err != nil {
		h.Logger.Error("list dispatch push subscriptions", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list push subscriptions")
		return
	}
	resp := make([]pushDispatchResponse, 0, len(subs))
	for _, sub := range subs {
		resp = append(resp, pushDispatchResponse{
			Endpoint: sub.Endpoint,
			Keys:     pushKeys{Auth: sub.AuthKey, P256DH: sub.P256DHKey},
			Filters:  sub.Filters,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"subscriptions": resp})
}

// DeletePushSubscriptionForDispatch handles
// DELETE /api/v1/push/subscriptions/dispatch. The sender prunes endpoints the
// push service reports as gone (404/410).
func (h *Handler) DeletePushSubscriptionForDispatch(w http.ResponseWriter, r *http.Request) {
	if !validDispatchSecret(r) {
		writeError(w, r, http.StatusUnauthorized, CodeUnauthorized, "invalid dispatch secret")
		return
	}
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	if !validPushEndpoint(strings.TrimSpace(req.Endpoint)) {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid push endpoint")
		return
	}
	if _, err := h.Store.DeletePushSubscription(r.Context(), "", req.Endpoint); err != nil {
		h.Logger.Error("prune push subscription", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to delete push subscription")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
