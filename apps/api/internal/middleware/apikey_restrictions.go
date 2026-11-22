package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/sorolens/sorolens/apps/api/internal/allowlist"
)

// CodeAPIKeyRestricted is returned when an authenticated key is used from a
// network or browser origin its allowlists do not permit (issue #793). It is
// distinct from FORBIDDEN and UNAUTHORIZED so a client can tell a restriction
// rejection from a missing scope or a bad credential.
const CodeAPIKeyRestricted = "API_KEY_RESTRICTED"

// APIKeyRestrictions enforces the per-key allowed_cidrs and allowed_origins
// lists. It must run after Identity so it only ever inspects a key that has
// already authenticated, and it never changes the outcome for requests without
// one: sessions, trusted-proxy identities and anonymous callers pass through.
//
// The client address comes from clientIPWithTrustedProxies, the same resolver
// the rate limiter uses, so X-Forwarded-For only counts when the direct peer is
// in trustedProxies. CORS preflights are skipped: a browser does not attach the
// key to them, and the CORS middleware owns that decision.
func APIKeyRestrictions(trustedProxies []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			identity, ok := IdentityFromRequest(r)
			if !ok || identity.APIKey == nil {
				next.ServeHTTP(w, r)
				return
			}
			key := identity.APIKey
			if len(key.AllowedCIDRs) == 0 && len(key.AllowedOrigins) == 0 {
				next.ServeHTTP(w, r)
				return
			}
			if len(key.AllowedCIDRs) > 0 &&
				!allowlist.IPAllowed(key.AllowedCIDRs, clientIPWithTrustedProxies(r, trustedProxies)) {
				writeAPIKeyRestricted(w, r, "API key is not allowed from this IP address")
				return
			}
			if !allowlist.OriginAllowed(key.AllowedOrigins, r.Header.Get("Origin")) {
				writeAPIKeyRestricted(w, r, "API key is not allowed from this origin")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeAPIKeyRestricted(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{
		"code":       CodeAPIKeyRestricted,
		"message":    message,
		"request_id": GetRequestID(r.Context()),
	}})
}
