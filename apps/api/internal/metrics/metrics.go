// Package metrics defines the Prometheus collectors exposed by the API and the
// HTTP handler that serves them in the text exposition format.
//
// The collectors live on a dedicated registry instead of the process-wide
// prometheus.DefaultRegisterer for two reasons:
//
//   - Tests can assert on a known, isolated set of series without other
//     packages' collectors leaking into the scrape.
//   - Repeated calls to NewRegistry-style constructors in tests can never
//     panic with "duplicate metrics collector registration attempted".
//
// See docs/metrics.md for the full metric catalogue.
package metrics

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// namespace prefixes every Sorolens metric so they are easy to select in
// PromQL (e.g. {__name__=~"sorolens_.*"}).
const namespace = "sorolens"

// Registry holds every collector the API exposes.
var Registry = prometheus.NewRegistry()

// Handler serves Registry in the Prometheus text exposition format. It is
// mounted unauthenticated at GET /metrics.
var Handler = promhttp.HandlerFor(Registry, promhttp.HandlerOpts{})

var (
	// HTTPRequestsInFlight counts requests currently being served.
	HTTPRequestsInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "http",
		Name:      "requests_in_flight",
		Help:      "Number of HTTP requests currently being served.",
	})

	// HTTPRequestsTotal counts completed requests by method, route and status.
	HTTPRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total number of HTTP requests handled, partitioned by method, route and status code.",
	}, []string{"method", "route", "status"})

	// HTTPRequestDuration observes handler latency by method, route and status.
	// The route label is the chi route pattern (e.g. /api/v1/contracts/{id}),
	// not the raw path, which keeps series cardinality bounded.
	HTTPRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request latency in seconds, partitioned by method, route and status code.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "route", "status"})
)

func init() {
	Registry.MustRegister(
		// Default Go runtime and process collectors (goroutines, GC, RSS,
		// open file descriptors, ...).
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),

		HTTPRequestsInFlight,
		HTTPRequestsTotal,
		HTTPRequestDuration,
	)
}

// ObserveRequest records one completed HTTP request. route may be empty when
// chi could not resolve a route pattern (a 404 against an unregistered path);
// such requests are folded into the single "unmatched" series so an unknown
// path cannot create unbounded label cardinality.
func ObserveRequest(method, route string, status int, seconds float64) {
	if route == "" {
		route = "unmatched"
	}
	code := strconv.Itoa(status)
	HTTPRequestsTotal.WithLabelValues(method, route, code).Inc()
	HTTPRequestDuration.WithLabelValues(method, route, code).Observe(seconds)
}

// NewAdminMux returns a mux serving only the metrics endpoint. It backs the
// optional dedicated admin listener (METRICS_PORT) so operators can scrape the
// endpoint without exposing it on the public API port.
func NewAdminMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", Handler)
	return mux
}
