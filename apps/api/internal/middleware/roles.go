package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Role strings mirror store.Role*. Kept here so route wiring and the
// middleware share one vocabulary.
const (
	RoleViewer      = store.RoleViewer
	RoleContributor = store.RoleContributor
	RoleAdmin       = store.RoleAdmin
)

// RequireRole returns middleware that denies the request unless the
// authenticated caller holds at least minRole. It is the RBAC counterpart to
// RequireScopes; routes that need role enforcement apply it with r.With so
// chi has already resolved the leaf route.
//
// The identity comes exclusively from the context populated by Identity: a
// session token, an API key bound to a user, or trusted-proxy signed headers.
// Raw identity headers are ignored (issue #518).
//
// Role semantics: viewer < contributor < admin.
//
//   - No identity resolvable: 401.
//   - Caller not found or role below the requirement: 403.
//   - An admin:* API key satisfies every role even when it is not bound to a
//     user row, so operators can bootstrap and rotate admin keys.
func RequireRole(logger *slog.Logger, minRole string) func(http.Handler) http.Handler {
	return requireRole(logger, minRole, http.StatusUnauthorized)
}

// RequireRoleOrForbidden is RequireRole with the unauthenticated rejection
// collapsed into 403. It is for surfaces that should not advertise themselves:
// an anonymous caller receives the same answer as a signed-in caller who lacks
// the role. The admin-gated pprof endpoints use it so probing /debug/pprof/*
// always answers 403 unless the caller is an admin.
func RequireRoleOrForbidden(logger *slog.Logger, minRole string) func(http.Handler) http.Handler {
	return requireRole(logger, minRole, http.StatusForbidden)
}

// requireRole is the shared implementation behind RequireRole and
// RequireRoleOrForbidden. unauthStatus is the response used when no identity
// can be resolved; an identified caller below minRole always gets 403.
func requireRole(logger *slog.Logger, minRole string, unauthStatus int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := IdentityFromRequest(r)
			if !ok {
				if logger != nil {
					logger.Debug("role middleware: no identity", "path", r.URL.Path)
				}
				writeRoleError(w, unauthStatus, map[string]string{
					"error": "authentication required",
				})
				return
			}

			if id.UserID != "" {
				if store.RoleRank(id.Role) < store.RoleRank(minRole) {
					writeRoleError(w, http.StatusForbidden, map[string]string{
						"error":    "insufficient role",
						"required": minRole,
						"role":     id.Role,
					})
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// An admin:* key carries its own grant. This is what allows an
			// operator to manage API keys before any user row is bound.
			if id.IsAdminKey() {
				next.ServeHTTP(w, r)
				return
			}

			// A valid credential that is not tied to a user cannot act as one.
			writeRoleError(w, http.StatusForbidden, map[string]string{
				"error":    "insufficient role",
				"required": minRole,
			})
		})
	}
}

func writeRoleError(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
