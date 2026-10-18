package handler

import (
	"net/http"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// sessionResponse is the credential returned by POST /api/v1/auth/session.
// The token is shown once; the client stores it and sends it as a bearer
// token on subsequent requests.
type sessionResponse struct {
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CreateSession handles POST /api/v1/auth/session.
//
// It mints an unguessable workspace identity and returns an HMAC-signed
// session token bound to it. The identity is a self-service workspace, not a
// proof of a person's identity: it receives the contributor role so the
// dashboard can edit its own watchlist, groups, notes and tags, but it can
// never reach admin-only routes. Before issue #518 the dashboard asserted a
// localStorage ID in X-User-ID; that header is no longer trusted, so the
// server must mint and sign the identity instead.
func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	if h.SessionSigner == nil || !h.SessionSigner.Enabled() {
		writeError(w, r, http.StatusServiceUnavailable, CodeInternal,
			"session authentication is not configured")
		return
	}

	userID := "sess_" + newID()
	if err := h.Store.UpsertUser(r.Context(), store.User{
		ID:   userID,
		Role: store.RoleContributor,
	}); err != nil {
		h.Logger.Error("create session user", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal,
			"failed to create session")
		return
	}

	token, expiresAt := h.SessionSigner.Issue(userID)
	writeJSON(w, http.StatusCreated, sessionResponse{
		UserID:    userID,
		Token:     token,
		ExpiresAt: expiresAt,
	})
}
