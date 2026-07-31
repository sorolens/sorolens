package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
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
