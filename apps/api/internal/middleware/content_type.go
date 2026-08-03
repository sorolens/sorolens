// Package middleware provides shared net/http middleware for the Sorolens
// API server.
package middleware

import (
	"encoding/json"
	"mime"
	"net/http"
)

// RequireJSONContentType returns middleware that rejects POST and PUT
// requests whose Content-Type header is missing or is not
// "application/json". Matching requests are passed through to next
// unchanged; all other methods are never inspected.
//
// A rejected request receives a 415 Unsupported Media Type response with a
// JSON body of the form {"error": "...", "code": "UNSUPPORTED_MEDIA_TYPE"},
// consistent with the API's documented error contract.
func RequireJSONContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodPut {
			next.ServeHTTP(w, r)
			return
		}

		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			writeUnsupportedMediaType(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeUnsupportedMediaType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnsupportedMediaType)
	// Ignore encode errors (e.g., client disconnect while writing the response); there's no meaningful recovery here.
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "Content-Type header must be application/json",
		"code":  "UNSUPPORTED_MEDIA_TYPE",
	})
}
)

// ContentTypeJSON rejects POST and PUT requests whose Content-Type is not application/json.
func ContentTypeJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnsupportedMediaType)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]string{
						"code":       "UNSUPPORTED_MEDIA_TYPE",
						"message":    "Content-Type must be application/json",
						"request_id": GetRequestID(r.Context()),
					},
				})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
