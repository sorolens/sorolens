package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// signingSecretRevealWindow is how long a signing secret stays retrievable
// through GET .../signing-secret after it is created or rotated. The plaintext
// is returned once at creation regardless; this window is the only way to
// re-fetch it. See docs/webhooks.md.
const signingSecretRevealWindow = 5 * time.Minute

// webhookSecretPrefix matches webhooksig.SecretPrefix in the indexer (the two
// live in separate modules, so the constant is duplicated deliberately).
const webhookSecretPrefix = "whsec_"

// ---- request/response types ------------------------------------------------

type createSubscriptionRequest struct {
	ContractID     string `json:"contract_id"`
	WebhookURL     string `json:"webhook_url"`
	SeverityFilter string `json:"severity_filter"`
}

type subscriptionResponse struct {
	ID             string    `json:"id"`
	ContractID     string    `json:"contract_id"`
	WebhookURL     string    `json:"webhook_url"`
	SeverityFilter string    `json:"severity_filter"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// createSubscriptionResponse adds the plaintext signing secret, which is
// returned exactly once, at creation (the API-key pattern).
type createSubscriptionResponse struct {
	subscriptionResponse
	SigningSecret string `json:"signing_secret"`
}

// signingSecretResponse is the body of the reveal and rotate endpoints.
type signingSecretResponse struct {
	ID            string     `json:"id"`
	SigningSecret string     `json:"signing_secret"`
	CreatedAt     time.Time  `json:"created_at"`
	RotatedAt     *time.Time `json:"rotated_at,omitempty"`
}

type subscriptionsResponse struct {
	Subscriptions []subscriptionResponse `json:"subscriptions"`
}

// ---- converters -------------------------------------------------------------

// subscriptionFromStore never carries the signing secret: it is only ever
// returned by the create, reveal and rotate endpoints.
func subscriptionFromStore(s store.AlertSubscription) subscriptionResponse {
	return subscriptionResponse{
		ID:             s.ID,
		ContractID:     s.ContractID,
		WebhookURL:     s.WebhookURL,
		SeverityFilter: s.SeverityFilter,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

// ---- handlers ---------------------------------------------------------------

// CreateSubscription handles POST /api/v1/watchdog/subscriptions.
//
// A 32-byte signing secret is generated, stored (with its SHA-256 hash) and
// returned once as `signing_secret`. Deliveries to this subscription are signed
// with it.
func (h *Handler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	var req createSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	if req.ContractID == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "contract_id is required")
		return
	}
	if req.WebhookURL == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "webhook_url is required")
		return
	}
	severity := req.SeverityFilter
	if severity == "" {
		severity = "Critical"
	}

	secret, err := newWebhookSecret()
	if err != nil {
		h.Logger.Error("generate webhook signing secret", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to create subscription")
		return
	}
	now := time.Now().UTC()

	sub := store.AlertSubscription{
		ID:                     fmt.Sprintf("sub_%d", time.Now().UnixNano()),
		ContractID:             req.ContractID,
		WebhookURL:             req.WebhookURL,
		SeverityFilter:         severity,
		SigningSecret:          secret,
		SigningSecretHash:      store.HashKey(secret),
		SigningSecretCreatedAt: now,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := h.Store.Create(r.Context(), sub); err != nil {
		h.Logger.Error("create alert subscription", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to create subscription")
		return
	}
	writeJSON(w, http.StatusCreated, createSubscriptionResponse{
		subscriptionResponse: subscriptionFromStore(sub),
		SigningSecret:        secret,
	})
}

// ListSubscriptions handles GET /api/v1/watchdog/subscriptions.
func (h *Handler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	subs, err := h.Store.ListAll(r.Context())
	if err != nil {
		h.Logger.Error("list alert subscriptions", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list subscriptions")
		return
	}
	resp := make([]subscriptionResponse, len(subs))
	for i, s := range subs {
		resp[i] = subscriptionFromStore(s)
	}
	writeJSON(w, http.StatusOK, subscriptionsResponse{Subscriptions: resp})
}

// DeleteSubscription handles DELETE /api/v1/watchdog/subscriptions/:id.
func (h *Handler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.Store.Delete(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "subscription not found")
			return
		}
		h.Logger.Error("delete alert subscription", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to delete subscription")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetSubscriptionSigningSecret handles
// GET /api/v1/watchdog/subscriptions/:id/signing-secret.
//
// The secret is only returned within signingSecretRevealWindow of its creation
// or its most recent rotation; otherwise the caller must rotate to get a new
// one. This keeps long-lived plaintext key material out of the API surface.
func (h *Handler) GetSubscriptionSigningSecret(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sub, err := h.Store.GetSubscription(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "subscription not found")
		return
	}
	if err != nil {
		h.Logger.Error("get alert subscription", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load subscription")
		return
	}

	if !signingSecretRevealable(sub, time.Now().UTC()) {
		writeError(w, r, http.StatusForbidden, CodeForbidden,
			"signing secret is only available for 5 minutes after creation or rotation; rotate to get a new one")
		return
	}

	writeJSON(w, http.StatusOK, signingSecretResponse{
		ID:            sub.ID,
		SigningSecret: sub.SigningSecret,
		CreatedAt:     sub.SigningSecretCreatedAt,
		RotatedAt:     sub.SigningSecretRotatedAt,
	})
}

// RotateSubscriptionSigningSecret handles
// POST /api/v1/watchdog/subscriptions/:id/rotate.
//
// It generates a fresh secret, invalidates the previous one for future
// deliveries, and returns the new plaintext once (re-opening the reveal
// window).
func (h *Handler) RotateSubscriptionSigningSecret(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	secret, err := newWebhookSecret()
	if err != nil {
		h.Logger.Error("generate webhook signing secret", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to rotate signing secret")
		return
	}
	rotatedAt := time.Now().UTC()

	if err := h.Store.RotateSigningSecret(r.Context(), id, secret, store.HashKey(secret), rotatedAt); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "subscription not found")
			return
		}
		h.Logger.Error("rotate signing secret", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to rotate signing secret")
		return
	}

	sub, err := h.Store.GetSubscription(r.Context(), id)
	if err != nil {
		h.Logger.Error("get alert subscription after rotate", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load subscription")
		return
	}

	writeJSON(w, http.StatusOK, signingSecretResponse{
		ID:            sub.ID,
		SigningSecret: sub.SigningSecret,
		CreatedAt:     sub.SigningSecretCreatedAt,
		RotatedAt:     sub.SigningSecretRotatedAt,
	})
}

// ---- helpers ----------------------------------------------------------------

// signingSecretRevealable reports whether the plaintext secret may be returned
// right now: within the window of its creation or of its last rotation.
func signingSecretRevealable(sub store.AlertSubscription, now time.Time) bool {
	if !sub.SigningSecretCreatedAt.IsZero() && now.Sub(sub.SigningSecretCreatedAt) <= signingSecretRevealWindow {
		return true
	}
	if sub.SigningSecretRotatedAt != nil && now.Sub(*sub.SigningSecretRotatedAt) <= signingSecretRevealWindow {
		return true
	}
	return false
}

// newWebhookSecret returns a random "whsec_"-prefixed signing secret. 32 random
// bytes match the entropy documented in docs/webhooks.md.
func newWebhookSecret() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return webhookSecretPrefix + base64.RawURLEncoding.EncodeToString(b[:]), nil
}
