package handler

import (
	"crypto/hmac"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ---- request / response types ----------------------------------------------

type createReportSubscriptionRequest struct {
	Email     string `json:"email"`
	Frequency string `json:"frequency"`
	DayOfWeek *int   `json:"day_of_week"`
}

type reportSubscriptionResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Frequency string    `json:"frequency"`
	DayOfWeek int       `json:"day_of_week"`
	CreatedAt time.Time `json:"created_at"`
}

func reportSubscriptionFromStore(s store.ReportSubscription) reportSubscriptionResponse {
	return reportSubscriptionResponse{
		ID:        s.ID,
		Email:     s.Email,
		Frequency: s.Frequency,
		DayOfWeek: s.DayOfWeek,
		CreatedAt: s.CreatedAt,
	}
}

// ---- handlers ---------------------------------------------------------------

// CreateReportSubscription handles POST /api/v1/reports/subscriptions.
func (h *Handler) CreateReportSubscription(w http.ResponseWriter, r *http.Request) {
	var req createReportSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	addr, err := mail.ParseAddress(strings.TrimSpace(req.Email))
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "a valid email is required")
		return
	}
	if !store.ValidReportFrequencies[req.Frequency] {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "frequency must be 'daily' or 'weekly'")
		return
	}
	day := 1
	if req.DayOfWeek != nil {
		day = *req.DayOfWeek
	}
	if day < 0 || day > 6 {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "day_of_week must be between 0 (Sunday) and 6 (Saturday)")
		return
	}

	sub := store.ReportSubscription{
		ID:        newID(),
		Email:     addr.Address,
		Frequency: req.Frequency,
		DayOfWeek: day,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.Store.CreateReportSubscription(r.Context(), sub); err != nil {
		h.Logger.Error("create report subscription", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to create subscription")
		return
	}
	writeJSON(w, http.StatusCreated, reportSubscriptionFromStore(sub))
}

// ListReportSubscriptions handles GET /api/v1/reports/subscriptions?email=...
func (h *Handler) ListReportSubscriptions(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.URL.Query().Get("email"))
	if email == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "email query parameter is required")
		return
	}
	subs, err := h.Store.ListReportSubscriptions(r.Context(), email)
	if err != nil {
		h.Logger.Error("list report subscriptions", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list subscriptions")
		return
	}
	resp := make([]reportSubscriptionResponse, len(subs))
	for i, s := range subs {
		resp[i] = reportSubscriptionFromStore(s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"subscriptions": resp})
}

// DeleteReportSubscription handles DELETE /api/v1/reports/subscriptions/{id}.
func (h *Handler) DeleteReportSubscription(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.Store.DeleteReportSubscription(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "subscription not found")
		return
	}
	if err != nil {
		h.Logger.Error("delete report subscription", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to delete subscription")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UnsubscribeReport handles GET /api/v1/reports/unsubscribe?token=...
//
// The token is the signed value embedded in every digest email, so a
// recipient can opt out in one click without authenticating. The link is
// intentionally a GET so it works straight from a mail client.
func (h *Handler) UnsubscribeReport(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	id, ok := h.verifyUnsubscribeToken(token)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid or missing unsubscribe token")
		return
	}
	err := h.Store.DeleteReportSubscription(r.Context(), id)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		h.Logger.Error("unsubscribe report", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to unsubscribe")
		return
	}
	// Idempotent: an already-unsubscribed link still reports success.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("You have been unsubscribed from Sorolens digest reports."))
}

// verifyUnsubscribeToken checks a token of the form "<id>.<hex-hmac>" and
// returns the subscription id when valid. When no signing key is configured
// the token is the bare id, so unsubscribe still works in local development.
func (h *Handler) verifyUnsubscribeToken(token string) (string, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	key := h.reportsSigningKey()
	id, sig, hasSig := strings.Cut(token, ".")
	if len(key) == 0 {
		// Unsigned mode: accept the bare id, reject a stray signature.
		if hasSig {
			return "", false
		}
		return id, true
	}
	if !hasSig {
		return "", false
	}
	want := signReport(key, []byte(id))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return "", false
	}
	return id, true
}

// UnsubscribeToken builds the signed one-click unsubscribe token for a
// subscription id, used when composing digest emails.
func UnsubscribeToken(signingKey []byte, id string) string {
	sig := signReport(signingKey, []byte(id))
	if sig == "" {
		return id
	}
	return id + "." + sig
}
