package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/sorolens/sorolens/apps/api/internal/apierror"
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
					apierror.WriteError(w, r, http.StatusInternalServerError, apierror.CodeInternal, "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
