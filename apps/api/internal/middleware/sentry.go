package middleware

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/getsentry/sentry-go"
)

// Sentry reports unhandled panics and handler responses with a 5xx status to
// Sentry, then re-panics so Recoverer (registered after this middleware in
// the router's chain) still produces the standard 500 response. It must run
// before Recoverer for that ordering to work.
//
// This is a safe no-op when Sentry has not been initialized (SENTRY_DSN
// unset): sentry-go falls back to a client with a no-op transport, so every
// call below still runs but delivers nothing.
//
// Sensitive request data (Authorization headers, API keys passed as a query
// parameter, cookies, etc.) is scrubbed automatically by sentry-go's default
// data-collection deny-list before an event is sent; contract IDs, which only
// ever appear in the URL path, are unaffected by it.
func Sentry(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub := sentry.CurrentHub().Clone()
		hub.Scope().SetRequest(r)
		if id := GetRequestID(r.Context()); id != "" {
			hub.Scope().SetTag("request_id", id)
		}

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		defer func() {
			if err := recover(); err != nil {
				hub.Recover(err)
				panic(err)
			}
			if rec.status >= http.StatusInternalServerError {
				hub.WithScope(func(scope *sentry.Scope) {
					scope.SetTag("status_code", strconv.Itoa(rec.status))
					hub.CaptureMessage(fmt.Sprintf("%s %s returned %d", r.Method, r.URL.Path, rec.status))
				})
			}
		}()

		next.ServeHTTP(rec, r)
	})
}
