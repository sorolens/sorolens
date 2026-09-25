package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// APIKeyLookup is the subset of the store the scope middleware needs.
type APIKeyLookup interface {
	GetAPIKeyByHash(ctx context.Context, hash string) (store.APIKey, error)
	TouchAPIKey(ctx context.Context, id string) error
}

// Scope strings. Kept here so the route table and middleware share one
// vocabulary with the store.
const (
	ScopeReadContracts  = store.ScopeReadContracts
	ScopeWriteContracts = store.ScopeWriteContracts
	ScopeReadWatchdog   = store.ScopeReadWatchdog
	ScopeAdmin          = store.ScopeAdmin
)

// routeScopes maps "METHOD <chi route pattern>" to the scope a caller must
// present to hit that route. Routes absent from this table are unscoped
// (e.g. /health, /readyz) and are reachable by any authenticated key.
var routeScopes = map[string]string{
	"GET /api/v1/stats/global": ScopeReadContracts,

	"GET /api/v1/contracts":                  ScopeReadContracts,
	"POST /api/v1/contracts":                 ScopeWriteContracts,
	"GET /api/v1/contracts/{id}":             ScopeReadContracts,
	"GET /api/v1/contracts/{id}/events":      ScopeReadContracts,
	"GET /api/v1/contracts/{id}/invocations": ScopeReadContracts,
	"GET /api/v1/contracts/{id}/storage":     ScopeReadContracts,
	"GET /api/v1/contracts/{id}/stats":       ScopeReadContracts,
	"GET /api/v1/contracts/{id}/forecast":    ScopeReadContracts,
	"GET /api/v1/contracts/{id}/snapshot":    ScopeReadContracts,
	"GET /api/v1/contracts/{id}/stream":      ScopeReadContracts,
	"GET /api/v1/stream/events":               ScopeReadContracts,

	"GET /api/v1/watchdog/stats":                 ScopeReadWatchdog,
	"GET /api/v1/watchdog/alerts":                ScopeReadWatchdog,
	"GET /api/v1/watchdog/contracts":             ScopeReadWatchdog,
	"GET /api/v1/watchdog/contracts/{id}":        ScopeReadWatchdog,
	"GET /api/v1/watchdog/contracts/{id}/health": ScopeReadWatchdog,
	"GET /api/v1/watchdog/contracts/{id}/alerts": ScopeReadWatchdog,

	"GET /api/v1/api-keys":         ScopeAdmin,
	"POST /api/v1/api-keys":        ScopeAdmin,
	"DELETE /api/v1/api-keys/{id}": ScopeAdmin,
}

// RequiredScope returns the scope required for a method + route pattern.
// The boolean is false when the route is not in the metadata table.
func RequiredScope(method, pattern string) (string, bool) {
	pattern = normalizePattern(pattern)
	scope, ok := routeScopes[method+" "+pattern]
	return scope, ok
}

// normalizePattern strips the trailing slash chi appends to nested route
// patterns (e.g. "/api/v1/contracts/{id}/") so lookups are stable.
func normalizePattern(pattern string) string {
	if pattern == "" {
		return pattern
	}
	trimmed := strings.TrimSuffix(pattern, "/")
	if trimmed == "" {
		return "/"
	}
	return trimmed
}

// RequireScopes authenticates requests that carry an API key and enforces the
// per-route scope from the metadata table.
//
// Behavior:
//   - No credential presented: the request is allowed through unchanged on
//     read/write routes. This keeps the public v0.1 surface open for anonymous
//     dashboard traffic and matches the pre-scopes behavior. Admin routes
//     (API key management) always require a credential, so keys cannot be
//     minted anonymously.
//   - Credential presented but unknown or revoked: 401.
//   - Credential valid but missing the required scope: 403 with
//     {"error":"missing scope","required":"<scope>"}.
func RequireScopes(lookup APIKeyLookup, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			required, hasScopeRule := lookupRequiredScope(r)
			token := extractAPIKey(r)
			if token == "" {
				if hasScopeRule && required == ScopeAdmin {
					writeScopeError(w, http.StatusUnauthorized, map[string]string{
						"error": "authentication required",
					})
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			key, err := lookup.GetAPIKeyByHash(r.Context(), store.HashKey(token))
			if err != nil || key.Revoked() {
				writeScopeError(w, http.StatusUnauthorized, map[string]string{
					"error": "invalid API key",
				})
				return
			}
			if err := lookup.TouchAPIKey(r.Context(), key.ID); err != nil && logger != nil {
				logger.Warn("touch api key", "err", err, "key_id", key.ID)
			}

			if !hasScopeRule || key.HasScope(required) {
				next.ServeHTTP(w, r)
				return
			}

			writeScopeError(w, http.StatusForbidden, map[string]string{
				"error":    "missing scope",
				"required": required,
			})
		})
	}
}

func lookupRequiredScope(r *http.Request) (string, bool) {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return "", false
	}
	return RequiredScope(r.Method, rctx.RoutePattern())
}

// extractAPIKey pulls the token from Authorization: Bearer <token> or the
// ?api_key query param. Bearer wins when both are present.
func extractAPIKey(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
			return strings.TrimSpace(h[7:])
		}
	}
	return strings.TrimSpace(r.URL.Query().Get("api_key"))
}

func writeScopeError(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
