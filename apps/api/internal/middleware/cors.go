package middleware

import (
	"net/http"
	"os"
	"strings"
)

// AllowedOriginsEnv is the environment variable that holds a comma-separated
// list of browser origins permitted to call the API (for example the Vercel
// frontend domain).
const AllowedOriginsEnv = "ALLOWED_ORIGINS"

const (
	corsAllowedHeaders = "Content-Type, X-Request-ID, Authorization, X-API-Key"
	corsAllowedMethods = "GET, POST, DELETE, OPTIONS"
)

// CORS allows browser clients to call the API. The set of allowed origins is
// read from ALLOWED_ORIGINS at router construction time.
//
// When ALLOWED_ORIGINS is unset the middleware keeps the historical permissive
// behaviour and answers with "Access-Control-Allow-Origin: *", so local
// development and Vercel preview deployments work without extra configuration.
// Deployments should set ALLOWED_ORIGINS to the exact frontend origin(s) to
// restrict access.
func CORS(next http.Handler) http.Handler {
	return NewCORS(AllowedOriginsFromEnv())(next)
}

// AllowedOriginsFromEnv parses ALLOWED_ORIGINS. It returns nil when the
// variable is unset or empty, which NewCORS interprets as "allow any origin".
// The literal value "*" is also treated as allow-any.
func AllowedOriginsFromEnv() []string {
	raw := strings.TrimSpace(os.Getenv(AllowedOriginsEnv))
	if raw == "" || raw == "*" {
		return nil
	}
	origins := make([]string, 0, strings.Count(raw, ",")+1)
	for _, part := range strings.Split(raw, ",") {
		if origin := strings.TrimSpace(part); origin != "" {
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		return nil
	}
	return origins
}

// NewCORS builds CORS middleware for an explicit allowlist of origins. An empty
// allowlist means "allow any origin". The request Origin is only reflected back
// when it matches, and Vary: Origin is set so shared caches do not mix
// responses across origins.
func NewCORS(allowed []string) func(http.Handler) http.Handler {
	allowAny := len(allowed) == 0
	set := make(map[string]struct{}, len(allowed))
	for _, origin := range allowed {
		set[origin] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if allowAny {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Add("Vary", "Origin")
				if origin := r.Header.Get("Origin"); origin != "" {
					if _, ok := set[origin]; ok {
						w.Header().Set("Access-Control-Allow-Origin", origin)
					}
				}
			}

			w.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
			w.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
