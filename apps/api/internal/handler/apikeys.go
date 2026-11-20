package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/sorolens/sorolens/apps/api/internal/cursor"
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
	UserID     *string    `json:"user_id"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	RotatedAt  *time.Time `json:"rotated_at"`
	// AllowedCIDRs and AllowedOrigins are the key's source restrictions
	// (issue #793). An empty array means unrestricted.
	AllowedCIDRs   []string `json:"allowed_cidrs"`
	AllowedOrigins []string `json:"allowed_origins"`
}

// createAPIKeyResponse includes the plaintext token exactly once, at creation.
type createAPIKeyResponse struct {
	apiKeyResponse
	Key string `json:"key"`
}

type createAPIKeyRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
	// UserID optionally binds the key to an existing user (issue #518).
	// A bound key carries that user's identity; an unbound key only
	// authenticates scopes.
	UserID *string `json:"user_id,omitempty"`
	// AllowedCIDRs and AllowedOrigins optionally restrict where the key may
	// be used (issue #793). Omitted or empty means unrestricted.
	AllowedCIDRs   []string `json:"allowed_cidrs,omitempty"`
	AllowedOrigins []string `json:"allowed_origins,omitempty"`
}

// optionalStringList distinguishes an omitted PATCH field from an explicit
// null or empty array, which clears the restriction.
type optionalStringList struct {
	Set    bool
	Values []string
}

func (o *optionalStringList) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Values = nil
		return nil
	}
	return json.Unmarshal(data, &o.Values)
}

type updateAPIKeyRequest struct {
	AllowedCIDRs   optionalStringList `json:"allowed_cidrs"`
	AllowedOrigins optionalStringList `json:"allowed_origins"`
}

func apiKeyFromStore(k store.APIKey) apiKeyResponse {
	scopes := k.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	cidrs := k.AllowedCIDRs
	if cidrs == nil {
		cidrs = []string{}
	}
	origins := k.AllowedOrigins
	if origins == nil {
		origins = []string{}
	}
	return apiKeyResponse{
		ID:             k.ID,
		Name:           k.Name,
		KeyPrefix:      k.KeyPrefix,
		Scopes:         scopes,
		UserID:         k.UserID,
		CreatedAt:      k.CreatedAt,
		LastUsedAt:     k.LastUsedAt,
		RevokedAt:      k.RevokedAt,
		RotatedAt:      k.RotatedAt,
		AllowedCIDRs:   cidrs,
		AllowedOrigins: origins,
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

	allowedCIDRs, err := store.NormalizeAllowedCIDRs(req.AllowedCIDRs)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
		return
	}
	allowedOrigins, err := store.NormalizeAllowedOrigins(req.AllowedOrigins)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
		return
	}

	var boundUser *string
	if req.UserID != nil {
		userID := strings.TrimSpace(*req.UserID)
		if userID != "" {
			if _, err := h.Store.GetUserByID(r.Context(), userID); err != nil {
				writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput,
					"user_id does not match an existing user")
				return
			}
			boundUser = &userID
		}
	}

	token, err := newAPIToken()
	if err != nil {
		h.Logger.Error("generate api key", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to create API key")
		return
	}
	key := store.APIKey{
		ID:             newID(),
		Name:           req.Name,
		KeyPrefix:      token[:11], // "sl_" + 8 chars
		KeyHash:        store.HashKey(token),
		Scopes:         req.Scopes,
		CreatedAt:      time.Now().UTC(),
		UserID:         boundUser,
		AllowedCIDRs:   allowedCIDRs,
		AllowedOrigins: allowedOrigins,
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
	rawCursor, ok := cursor.Decode(r.URL.Query().Get("cursor"))
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
		"next_cursor": cursor.Encode(nextRaw),
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

// GetAPIKey handles GET /api/v1/api-keys/{id}. Key material is never returned,
// but metadata including rotated_at is, so callers can confirm a rotation.
func (h *Handler) GetAPIKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	key, err := h.Store.GetAPIKey(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "API key not found")
		return
	}
	if err != nil {
		h.Logger.Error("get api key", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch API key")
		return
	}
	writeJSON(w, http.StatusOK, apiKeyFromStore(key))
}

// UpdateAPIKey handles PATCH /api/v1/api-keys/{id}. It replaces the key's
// source allowlists (issue #793); omitted fields are left unchanged and an
// explicit empty array or null clears a restriction.
func (h *Handler) UpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	key, err := h.Store.GetAPIKey(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "API key not found")
		return
	}
	if err != nil {
		h.Logger.Error("get api key", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch API key")
		return
	}
	h.updateAPIKeyAllowlists(w, r, key)
}

// updateAPIKeyAllowlists validates and persists a PATCH body against key.
func (h *Handler) updateAPIKeyAllowlists(w http.ResponseWriter, r *http.Request, key store.APIKey) {
	var req updateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	if !req.AllowedCIDRs.Set && !req.AllowedOrigins.Set {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput,
			"at least one of allowed_cidrs or allowed_origins is required")
		return
	}

	cidrs := key.AllowedCIDRs
	if req.AllowedCIDRs.Set {
		cidrs = req.AllowedCIDRs.Values
	}
	origins := key.AllowedOrigins
	if req.AllowedOrigins.Set {
		origins = req.AllowedOrigins.Values
	}
	normalizedCIDRs, err := store.NormalizeAllowedCIDRs(cidrs)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
		return
	}
	normalizedOrigins, err := store.NormalizeAllowedOrigins(origins)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
		return
	}

	if err := h.Store.UpdateAPIKeyAllowlists(r.Context(), key.ID, normalizedCIDRs, normalizedOrigins); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "API key not found")
			return
		}
		h.Logger.Error("update api key allowlists", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to update API key")
		return
	}

	updated, err := h.Store.GetAPIKey(r.Context(), key.ID)
	if err != nil {
		h.Logger.Error("load updated api key", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to update API key")
		return
	}
	writeJSON(w, http.StatusOK, apiKeyFromStore(updated))
}

// RotateAPIKey handles POST /api/v1/api-keys/{id}/rotate.
//
// It regenerates the key's secret in place: a fresh plaintext token is
// returned once, the old secret is invalidated immediately, and the key's id,
// name, scopes and created_at are left unchanged. rotated_at is stamped so the
// rotation is observable via GetAPIKey. A revoked key cannot be rotated.
func (h *Handler) RotateAPIKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	token, err := newAPIToken()
	if err != nil {
		h.Logger.Error("generate rotated api key", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to rotate API key")
		return
	}

	err = h.Store.RotateAPIKey(r.Context(), id, store.HashKey(token), token[:11])
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "API key not found")
		return
	}
	if err != nil {
		h.Logger.Error("rotate api key", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to rotate API key")
		return
	}

	key, err := h.Store.GetAPIKey(r.Context(), id)
	if err != nil {
		h.Logger.Error("load rotated api key", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to rotate API key")
		return
	}

	writeJSON(w, http.StatusOK, createAPIKeyResponse{
		apiKeyResponse: apiKeyFromStore(key),
		Key:            token,
	})
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
