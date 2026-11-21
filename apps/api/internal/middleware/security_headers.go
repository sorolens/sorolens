package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeaders adds baseline browser protections, including on preflights
// and on responses that end before the router reaches a handler.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Strict-Transport-Security", "max-age=31536000")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if sensitivePath(r.URL.Path) {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func sensitivePath(path string) bool {
	for _, prefix := range []string{"/api/v1", "/api/v2"} {
		if rest, found := strings.CutPrefix(path, prefix+"/"); found {
			path = "/" + rest
			break
		}
	}
	for _, prefix := range []string{"/admin", "/api-keys", "/me/api-keys"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
