package middleware

import (
	"encoding/json"
	"net/http"
)

// RequireAuth returns middleware that rejects every request that carries no
// verifiable credential when `require` is true (API_REQUIRE_AUTH), except the
// listed exempt paths. Health checks, the version endpoint, Prometheus
// metrics and the session bootstrap endpoint stay reachable unauthenticated
// so an operator can still probe the service and mint a first session.
//
// It runs after Identity, so a verified caller (session, API key or trusted
// proxy) is always allowed through. A missing identity yields 401.
func RequireAuth(require bool, exempt ...string) func(http.Handler) http.Handler {
	exemptSet := make(map[string]struct{}, len(exempt))
	for _, p := range exempt {
		exemptSet[p] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !require {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := IdentityFromRequest(r); ok {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := exemptSet[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "authentication required",
			})
		})
	}
}
