package middleware

import (
	"net/http"
	"strings"
)

const (
	contentTypeOptionsValue    = "nosniff"
	contentSecurityPolicyValue = "default-src 'none'; frame-ancestors 'none'; base-uri 'none'"
	hstsValue                  = "max-age=31536000; includeSubDomains"
	referrerPolicyValue        = "strict-origin-when-cross-origin"
)

// SecurityHeaders adds the standard response headers used by the API. API key
// management and admin responses are never stored by clients or intermediaries.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", contentTypeOptionsValue)
		w.Header().Set("Content-Security-Policy", contentSecurityPolicyValue)
		w.Header().Set("Strict-Transport-Security", hstsValue)
		w.Header().Set("Referrer-Policy", referrerPolicyValue)
		if isSensitiveRoute(r.URL.Path) {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func isSensitiveRoute(path string) bool {
	for _, prefix := range []string{
		"/api/v1/api-keys",
		"/api/v1/admin",
		"/api/v2/api-keys",
		"/api/v2/admin",
	} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
