package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Role strings mirror store.Role*. Kept here so route wiring and the
// middleware share one vocabulary.
const (
	RoleViewer      = store.RoleViewer
	RoleContributor = store.RoleContributor
	RoleAdmin       = store.RoleAdmin
)

// UserLookup is the subset of the store the role middleware needs.
type UserLookup interface {
	GetUserByID(ctx context.Context, id string) (store.User, error)
	GetUserByGitHubID(ctx context.Context, githubID string) (store.User, error)
}

// identityFromRequest resolves the calling user from the request. Role
// enforcement is GitHub-ID based (the credential the dashboard and CLI hold):
// the X-User-ID header, or a GitHub login in the X-GitHub-Login header, else
// an API key hash owner lookup via the middleware chain.
type identityFromRequest func(r *http.Request) (store.User, bool)

// RequireRole returns middleware that denies the request unless the
// authenticated caller holds at least minRole. It is the RBAC counterpart to
// RequireScopes; routes that need role enforcement apply it with r.With so
// chi has already resolved the leaf route.
//
// Role semantics: viewer < contributor < admin.
//
//   - No identity resolvable: 401.
//   - Caller not found or role below the requirement: 403.
func RequireRole(lookup UserLookup, logger *slog.Logger, minRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := resolveUser(r, lookup)
			if !ok {
				if logger != nil {
					logger.Debug("role middleware: no identity", "path", r.URL.Path)
				}
				writeRoleError(w, http.StatusUnauthorized, map[string]string{
					"error": "authentication required",
				})
				return
			}
			if store.RoleRank(user.Role) < store.RoleRank(minRole) {
				writeRoleError(w, http.StatusForbidden, map[string]string{
					"error":    "insufficient role",
					"required": minRole,
					"role":     user.Role,
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// resolveUser tries, in order: the X-User-ID header (a stored sorolens user
// id), the X-GitHub-Login header (a GitHub login mapped via github_id), and
// finally GitHub OAuth-style sub claim in the X-User-Sub header if present.
func resolveUser(r *http.Request, lookup UserLookup) (store.User, bool) {
	if id := strings.TrimSpace(r.Header.Get("X-User-ID")); id != "" {
		u, err := lookup.GetUserByID(r.Context(), id)
		if err == nil {
			return u, true
		}
	}
	if login := strings.TrimSpace(r.Header.Get("X-GitHub-Login")); login != "" {
		u, err := lookup.GetUserByGitHubID(r.Context(), login)
		if err == nil {
			return u, true
		}
	}
	if sub := strings.TrimSpace(r.Header.Get("X-User-Sub")); sub != "" {
		u, err := lookup.GetUserByID(r.Context(), sub)
		if err == nil {
			return u, true
		}
	}
	if login := strings.TrimSpace(r.Header.Get("X-GitHub-ID")); login != "" {
		u, err := lookup.GetUserByID(r.Context(), login)
		if err == nil {
			if u.GitHubID != nil && *u.GitHubID == login {
				return u, true
			}
		}
		if u2, err := lookup.GetUserByGitHubID(r.Context(), login); err == nil {
			return u2, true
		}
	}
	return store.User{}, false
}

func writeRoleError(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
