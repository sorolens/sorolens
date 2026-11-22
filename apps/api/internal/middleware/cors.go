package middleware

import (
	"net/http"
	"strings"
)

// CORS configures the browser policy once at startup. An empty setting or a
// wildcard entry permits any origin. Other entries match the Origin exactly.
func CORS(config string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool)
	for _, origin := range strings.Split(config, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowed[origin] = true
		}
	}
	wildcard := strings.TrimSpace(config) == "" || allowed["*"]

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Add("Vary", "Origin")
			origin := r.Header.Get("Origin")
			switch {
			case wildcard:
				h.Set("Access-Control-Allow-Origin", "*")
			case allowed[origin]:
				h.Set("Access-Control-Allow-Origin", origin)
			}

			// OPTIONS is handled before authentication, independent of route group.
			if r.Method == http.MethodOptions {
				if h.Get("Access-Control-Allow-Origin") != "" {
					h.Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID, Authorization, X-API-Key, If-None-Match, X-Identity-User-ID, X-Identity-Timestamp, X-Identity-Signature")
					h.Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS")
					h.Set("Access-Control-Max-Age", "600")
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
