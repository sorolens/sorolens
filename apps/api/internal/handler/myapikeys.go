package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Self-service API keys (issue #613).
//
// The admin-only /api/v1/api-keys routes are for operators minting keys for
// other users. A workspace session minted by POST /api/v1/auth/session is a
// contributor, so it cannot use those routes. These handlers let that session
// manage its own keys: every key is bound to the verified session user, and a
// key cannot be listed or revoked by anyone else.

type createMyAPIKeyRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
	// AllowedCIDRs and AllowedOrigins optionally restrict the key (issue #793).
	AllowedCIDRs   []string `json:"allowed_cidrs,omitempty"`
	AllowedOrigins []string `json:"allowed_origins,omitempty"`
}

// ListMyAPIKeys handles GET /api/v1/me/api-keys.
func (h *Handler) ListMyAPIKeys(w http.ResponseWriter, r *http.Request) {
	userID := middleware.VerifiedUserID(r)
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
		return
	}

	// API keys are few per user; one bounded page is enough and keeps the
	// store surface unchanged. The result is filtered to keys this session
	// owns, so another user's keys are never exposed.
	keys, _, err := h.Store.ListAPIKeys(r.Context(), "", 200)
	if err != nil {
		h.Logger.Error("list my api keys", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list API keys")
		return
	}

	resp := make([]apiKeyResponse, 0, len(keys))
	for _, key := range keys {
		if key.Revoked() || key.UserID == nil || *key.UserID != userID {
			continue
		}
		resp = append(resp, apiKeyFromStore(key))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"api_keys":    resp,
		"next_cursor": "",
	})
}

// CreateMyAPIKey handles POST /api/v1/me/api-keys. The plaintext key is
// returned exactly once and bound to the session user.
func (h *Handler) CreateMyAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := middleware.VerifiedUserID(r)
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
		return
	}

	var req createMyAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "name is required")
		return
	}
	if len(req.Scopes) == 0 {
		req.Scopes = []string{store.ScopeReadContracts}
	}
	for _, scope := range req.Scopes {
		if !store.ValidScopes[scope] || scope == store.ScopeAdmin {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput,
				"unknown scope "+scope+"; valid self-service scopes are: read:contracts, write:contracts, read:watchdog")
			return
		}
	}

	token, err := newAPIToken()
	if err != nil {
		h.Logger.Error("generate api key", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to create API key")
		return
	}
	allowedCIDRs, allowedOrigins, err := normalizeRestrictions(req.AllowedCIDRs, req.AllowedOrigins)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
		return
	}
	key := store.APIKey{
		ID:        newID(),
		Name:      req.Name,
		KeyPrefix: token[:11],
		KeyHash:   store.HashKey(token),
		Scopes:    req.Scopes,
		CreatedAt: time.Now().UTC(),
		UserID:    &userID,
		// Self-service keys can be restricted to their owner's networks and
		// origins just like admin-created ones.
		AllowedCIDRs:   allowedCIDRs,
		AllowedOrigins: allowedOrigins,
	}
	if err := h.Store.CreateAPIKey(r.Context(), key); err != nil {
		if errors.Is(err, store.ErrInvalidScope) {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid scope")
			return
		}
		if errors.Is(err, store.ErrInvalidCIDR) || errors.Is(err, store.ErrInvalidOrigin) {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
			return
		}
		h.Logger.Error("create api key", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to create API key")
		return
	}

	writeJSON(w, http.StatusCreated, createAPIKeyResponse{
		apiKeyResponse: apiKeyFromStore(key),
		Key:            token,
	})
}

// RevokeMyAPIKey handles DELETE /api/v1/me/api-keys/{id}. A key owned by
// another user is reported as not found so its existence is not disclosed.
func (h *Handler) RevokeMyAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := middleware.VerifiedUserID(r)
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
		return
	}

	id := chi.URLParam(r, "id")
	key, err := h.Store.GetAPIKey(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && (key.UserID == nil || *key.UserID != userID)) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "API key not found")
		return
	}
	if err != nil {
		h.Logger.Error("get api key", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch API key")
		return
	}

	if err := h.Store.RevokeAPIKey(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "API key not found")
			return
		}
		h.Logger.Error("revoke api key", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to revoke API key")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
