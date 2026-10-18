package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeaders returns an http.Handler middleware that sets standard security
// headers on every response.

// It always sets:
//   - X-Content-Type-Options: nosniff
//   - Strict-Transport-Security: max-age=31536000; includeSubdomains
//   - Referrer-Policy: no-referrer
//
// On key-management and admin routes it additionally sets Cache-Control:
// no-store so credentials and admin payloads are never cached by intermediaries
// or the browser.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubdomains")
			h.Set("Referrer-Policy", "no-referrer")
			if isSensitivePath(r.URL.Path) {
				h.Set("Cache-Control", "no-store")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isSensitivePath reports whether the path belongs to a route whose responses
// must never be cached (API key management or admin endpoints).
func isSensitivePath(path string) bool {
	return strings.Contains(path, "/api-keys") || strings.Contains(path, "/admin")
}
