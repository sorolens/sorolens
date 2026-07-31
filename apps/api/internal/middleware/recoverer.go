package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recoverer catches panics, logs the stack trace, and returns a 500 error envelope.
func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						"error", rec,
						"stack", string(debug.Stack()),
						"request_id", GetRequestID(r.Context()),
					)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]any{
						"error": map[string]string{
							"code":       "INTERNAL",
							"message":    "internal server error",
							"request_id": GetRequestID(r.Context()),
						},
					})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
