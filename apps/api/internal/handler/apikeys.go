package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ---- response types ---------------------------------------------------------

type apiKeyResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

// createAPIKeyResponse includes the plaintext token exactly once, at creation.
type createAPIKeyResponse struct {
	apiKeyResponse
	Key string `json:"key"`
}

type createAPIKeyRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

func apiKeyFromStore(k store.APIKey) apiKeyResponse {
	scopes := k.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	return apiKeyResponse{
		ID:         k.ID,
		Name:       k.Name,
		KeyPrefix:  k.KeyPrefix,
		Scopes:     scopes,
		CreatedAt:  k.CreatedAt,
		LastUsedAt: k.LastUsedAt,
		RevokedAt:  k.RevokedAt,
	}
}

// ---- handlers ---------------------------------------------------------------

// CreateAPIKey handles POST /api/v1/api-keys.
//
// The plaintext key is returned once in the `key` field and never stored;
// only its SHA-256 hash is persisted.
func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req createAPIKeyRequest
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
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput,
			"scopes must contain at least one of: read:contracts, write:contracts, read:watchdog, admin:*")
		return
	}
	for _, s := range req.Scopes {
		if !store.ValidScopes[s] {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput,
				"unknown scope "+s+"; valid scopes are: read:contracts, write:contracts, read:watchdog, admin:*")
			return
		}
	}

	token, err := newAPIToken()
	if err != nil {
		h.Logger.Error("generate api key", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to create API key")
		return
	}
	key := store.APIKey{
		ID:        newID(),
		Name:      req.Name,
		KeyPrefix: token[:11], // "sl_" + 8 chars
		KeyHash:   store.HashKey(token),
		Scopes:    req.Scopes,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.Store.CreateAPIKey(r.Context(), key); err != nil {
		if errors.Is(err, store.ErrInvalidScope) {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid scope")
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

// ListAPIKeys handles GET /api/v1/api-keys. Key material is never returned.
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	rawCursor, ok := decodeCursor(r.URL.Query().Get("cursor"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
		return
	}
	keys, nextRaw, err := h.Store.ListAPIKeys(r.Context(), rawCursor, intQuery(r, "limit", 50))
	if err != nil {
		h.Logger.Error("list api keys", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list API keys")
		return
	}
	resp := make([]apiKeyResponse, len(keys))
	for i, k := range keys {
		resp[i] = apiKeyFromStore(k)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"api_keys":    resp,
		"next_cursor": encodeCursor(nextRaw),
	})
}

// RevokeAPIKey handles DELETE /api/v1/api-keys/{id}.
func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.Store.RevokeAPIKey(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "API key not found")
		return
	}
	if err != nil {
		h.Logger.Error("revoke api key", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to revoke API key")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- helpers ----------------------------------------------------------------

// newAPIToken returns a random "sl_"-prefixed token. 32 random bytes give
// roughly 256 bits of entropy, far beyond brute force.
func newAPIToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "sl_" + base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
