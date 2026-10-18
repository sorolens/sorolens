package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

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
	"POST /graphql": ScopeReadContracts,

	"GET /api/v1/stats/global":   ScopeReadContracts,
	"GET /api/v1/stats/activity": ScopeReadContracts,
	"GET /api/v1/events/recent":  ScopeReadContracts,
	"GET /api/v1/compare":        ScopeReadContracts,

	"GET /api/v1/contracts":                            ScopeReadContracts,
	"POST /api/v1/contracts":                           ScopeWriteContracts,
	"POST /api/v1/contracts/batch":                     ScopeWriteContracts,
	"POST /api/v1/contracts/validate":                  ScopeReadContracts,
	"GET /api/v1/contracts/{id}":                       ScopeReadContracts,
	"GET /api/v1/contracts/{id}/events":                ScopeReadContracts,
	"GET /api/v1/contracts/{id}/events.csv":            ScopeReadContracts,
	"GET /api/v1/contracts/{id}/invocations":           ScopeReadContracts,
	"GET /api/v1/contracts/{id}/invocations/{tx_hash}": ScopeReadContracts,
	"GET /api/v1/contracts/{id}/storage":               ScopeReadContracts,
	"GET /api/v1/contracts/{id}/stats":                 ScopeReadContracts,
	"GET /api/v1/contracts/{id}/summary":               ScopeReadContracts,
	"GET /api/v1/contracts/{id}/forecast":              ScopeReadContracts,
	"GET /api/v1/contracts/{id}/snapshot":              ScopeReadContracts,
	"GET /api/v1/contracts/{id}/spec":                  ScopeReadContracts,
	"POST /api/v1/contracts/{id}/tags":                 ScopeWriteContracts,
	"DELETE /api/v1/contracts/{id}/tags/{tag}":         ScopeWriteContracts,
	"GET /api/v1/contracts/{id}/snapshot.json":         ScopeReadContracts,
	"GET /api/v1/contracts/{id}/stream":                ScopeReadContracts,
	"GET /api/v1/contracts/{id}/wasm":                  ScopeReadContracts,
	"GET /api/v1/stream/events":                        ScopeReadContracts,
	"GET /api/v1/subscribe":                            ScopeReadContracts,
	"POST /api/v1/contracts/{id}/verify":               ScopeWriteContracts,
	"GET /api/v1/contracts/{id}/verification":          ScopeReadContracts,

	"GET /api/v1/events": ScopeReadContracts,

	// Markdown notes on a contract (issue #164): reads are open, authoring
	// needs the contributor role, and the handlers restrict edits to the
	// note's own author. Kept as its own block because the note patterns are
	// longer than the contract patterns above.
	"GET /api/v1/contracts/{id}/notes":             ScopeReadContracts,
	"POST /api/v1/contracts/{id}/notes":            ScopeWriteContracts,
	"PATCH /api/v1/contracts/{id}/notes/{noteID}":  ScopeWriteContracts,
	"DELETE /api/v1/contracts/{id}/notes/{noteID}": ScopeWriteContracts,

	"GET /api/v1/invocations/{tx_hash}/trace": ScopeReadContracts,

	"GET /api/v1/rules":           ScopeReadContracts,
	"GET /api/v1/rules/metrics":   ScopeReadContracts,
	"GET /api/v1/rules/library":   ScopeReadContracts,
	"POST /api/v1/rules/validate": ScopeReadContracts,
	"POST /api/v1/rules":          ScopeWriteContracts,
	"POST /api/v1/rules/preview":  ScopeWriteContracts,
	"PATCH /api/v1/rules/{id}":    ScopeWriteContracts,
	"DELETE /api/v1/rules/{id}":   ScopeWriteContracts,

	"GET /api/v1/dlq":               ScopeReadContracts,
	"POST /api/v1/dlq/{id}/requeue": ScopeWriteContracts,

	"POST /api/v1/watched-accounts":        ScopeWriteContracts,
	"GET /api/v1/watched-accounts":         ScopeReadContracts,
	"DELETE /api/v1/watched-accounts/{id}": ScopeWriteContracts,

	"GET /api/v1/watchdog/stats":                 ScopeReadWatchdog,
	"GET /api/v1/watchdog/alerts":                ScopeReadWatchdog,
	"GET /api/v1/watchdog/contracts":             ScopeReadWatchdog,
	"GET /api/v1/watchdog/contracts/{id}":        ScopeReadWatchdog,
	"GET /api/v1/watchdog/contracts/{id}/health": ScopeReadWatchdog,
	"GET /api/v1/watchdog/contracts/{id}/alerts": ScopeReadWatchdog,

	// Subscriptions are watchdog-scoped for reads and write-scoped for
	// mutations (with a contributor role via the router). The signing-secret
	// reveal/rotate endpoints touch key material and stay admin-gated.
	"GET /api/v1/watchdog/subscriptions":                     ScopeReadWatchdog,
	"POST /api/v1/watchdog/subscriptions":                    ScopeWriteContracts,
	"DELETE /api/v1/watchdog/subscriptions/{id}":             ScopeWriteContracts,
	"GET /api/v1/watchdog/subscriptions/{id}/signing-secret": ScopeAdmin,
	"POST /api/v1/watchdog/subscriptions/{id}/rotate":        ScopeAdmin,

	"GET /api/v1/api-keys":              ScopeAdmin,
	"POST /api/v1/api-keys":             ScopeAdmin,
	"GET /api/v1/api-keys/{id}":         ScopeAdmin,
	"DELETE /api/v1/api-keys/{id}":      ScopeAdmin,
	"POST /api/v1/api-keys/{id}/rotate": ScopeAdmin,

	// v2 (issue #144): the same scope vocabulary applies, keyed by the v2
	// route pattern.
	"GET /api/v2/stats/global":   ScopeReadContracts,
	"GET /api/v2/stats/activity": ScopeReadContracts,
	"GET /api/v2/events/recent":  ScopeReadContracts,

	"GET /api/v2/contracts":                  ScopeReadContracts,
	"POST /api/v2/contracts":                 ScopeWriteContracts,
	"POST /api/v2/contracts/validate":        ScopeReadContracts,
	"GET /api/v2/contracts/{id}":             ScopeReadContracts,
	"GET /api/v2/contracts/{id}/events":      ScopeReadContracts,
	"GET /api/v2/contracts/{id}/invocations": ScopeReadContracts,
	"GET /api/v2/contracts/{id}/storage":     ScopeReadContracts,
	"GET /api/v2/contracts/{id}/stats":       ScopeReadContracts,
	"GET /api/v2/contracts/{id}/forecast":    ScopeReadContracts,
	"GET /api/v2/contracts/{id}/snapshot":    ScopeReadContracts,
	"GET /api/v2/contracts/{id}/stream":      ScopeReadContracts,

	"GET /api/v2/watchdog/stats":                 ScopeReadWatchdog,
	"GET /api/v2/watchdog/alerts":                ScopeReadWatchdog,
	"GET /api/v2/watchdog/contracts":             ScopeReadWatchdog,
	"GET /api/v2/watchdog/contracts/{id}":        ScopeReadWatchdog,
	"GET /api/v2/watchdog/contracts/{id}/health": ScopeReadWatchdog,
	"GET /api/v2/watchdog/contracts/{id}/alerts": ScopeReadWatchdog,

	"GET /api/v2/api-keys":              ScopeAdmin,
	"POST /api/v2/api-keys":             ScopeAdmin,
	"GET /api/v2/api-keys/{id}":         ScopeAdmin,
	"DELETE /api/v2/api-keys/{id}":      ScopeAdmin,
	"POST /api/v2/api-keys/{id}/rotate": ScopeAdmin,
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

// RequireScopes enforces the per-route scope from the metadata table for
// callers that presented a scoped API key.
//
// Credentials are verified once by the Identity middleware, which also
// resolves a key's optional user binding. This middleware never trusts raw
// identity headers (issue #518).
//
// Behavior:
//   - No credential presented: the request is allowed through unchanged on
//     read/write routes. This keeps the public v0.1 surface open for anonymous
//     dashboard traffic. Admin routes (API key management) always require a
//     credential, so keys cannot be minted anonymously.
//   - Credential presented but unknown, revoked or expired: 401.
//   - API key valid but missing the required scope: 403 with
//     {"error":"missing scope","required":"<scope>"}.
//   - Session or trusted-proxy identity: allowed; scopes are a property of
//     API keys only, and role middleware still applies where wired.
func RequireScopes(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			required, hasScopeRule := lookupRequiredScope(r)
			token := extractAPIKey(r)
			id, hasIdentity := IdentityFromRequest(r)

			if !hasIdentity {
				if token != "" {
					// Presented but not verifiable as an API key or session.
					if logger != nil {
						logger.Debug("scope middleware: invalid credential", "path", r.URL.Path)
					}
					writeScopeError(w, http.StatusUnauthorized, map[string]string{
						"error": "invalid API key",
					})
					return
				}
				if hasScopeRule && required == ScopeAdmin {
					writeScopeError(w, http.StatusUnauthorized, map[string]string{
						"error": "authentication required",
					})
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			if id.APIKey == nil {
				next.ServeHTTP(w, r)
				return
			}
			if !hasScopeRule || id.APIKey.HasScope(required) {
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
