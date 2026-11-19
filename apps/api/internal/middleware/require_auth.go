package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// isHealthEndpoint reports whether the path is a health check endpoint
// exempt from required authentication mode.
func isHealthEndpoint(p string) bool {
	p = strings.TrimSuffix(p, "/")
	return p == "/health" || p == "/readyz" || p == "/api/v1/health"
}

// RequireAuthMode enforces authentication across all endpoints when enabled.
// Health endpoints (/health, /readyz, /api/v1/health) and CORS preflights are exempt.
func RequireAuthMode(enabled bool, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled {
				next.ServeHTTP(w, r)
				return
			}
			if r.Method == http.MethodOptions || isHealthEndpoint(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := IdentityFromRequest(r); !ok {
				token := extractAPIKey(r)
				if token != "" {
					if logger != nil {
						logger.Debug("require-auth mode: invalid credential", "path", r.URL.Path)
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"error": "invalid API key",
					})
					return
				}
				if logger != nil {
					logger.Debug("require-auth mode: unauthenticated request rejected", "path", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "authentication required",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
