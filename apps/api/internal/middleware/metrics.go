package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/metrics"
)

// Metrics instruments every request with the collectors in the metrics
// package: an in-flight gauge, a per-route/status counter, and a
// per-route/status latency histogram.
//
// It must run after chi has resolved the route pattern for the label to be
// the pattern rather than the raw path. chi populates the RouteContext during
// routing, so reading chi.RouteContext(r.Context()).RoutePattern() *after*
// next.ServeHTTP returns the matched pattern.
//
// Requests that match no route (404s) have an empty pattern and are recorded
// as route="unmatched" to bound label cardinality.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics.HTTPRequestsInFlight.Inc()
		defer metrics.HTTPRequestsInFlight.Dec()

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()

		next.ServeHTTP(rec, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		metrics.ObserveRequest(r.Method, route, rec.status, time.Since(start).Seconds())
	})
}
