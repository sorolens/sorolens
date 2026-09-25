// Package metrics defines the Prometheus collectors the indexer publishes and
// the HTTP handler that serves them.
//
// The collectors live on a dedicated registry (not the process-wide
// prometheus.DefaultRegisterer) so tests can assert on a known set of series
// and repeated construction can never panic on duplicate registration.
//
// See docs/metrics.md for the full metric catalogue.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "sorolens"

// Registry holds every collector the indexer exposes.
var Registry = prometheus.NewRegistry()

// Handler serves Registry in the Prometheus text exposition format.
var Handler = promhttp.HandlerFor(Registry, promhttp.HandlerOpts{})

var (
	// LedgerLag is how far behind the chain head the indexer currently is, in
	// ledgers. 0 means the pass caught up to the ledger it observed via
	// getLatestLedger; a growing value means indexing is falling behind.
	LedgerLag = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "indexer",
		Name:      "ledger_lag",
		Help:      "Number of ledgers between the latest known ledger and the last ledger processed, by network.",
	}, []string{"network"})

	// EventsProcessedTotal counts contract events successfully written.
	EventsProcessedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "indexer",
		Name:      "events_processed_total",
		Help:      "Total number of contract events processed and persisted, by network.",
	}, []string{"network"})

	// RunDuration observes the wall-clock duration of a complete indexer pass.
	RunDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "indexer",
		Name:      "run_duration_seconds",
		Help:      "Duration of a full indexer pass in seconds, by run mode.",
		Buckets:   []float64{0.5, 1, 2.5, 5, 10, 30, 60, 120, 270, 300},
	}, []string{"mode"})
)

func init() {
	Registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),

		LedgerLag,
		EventsProcessedTotal,
		RunDuration,
	)
}

// ObserveLedgerLag records how far the indexer is behind the chain head for a
// network. Negative values are clamped to 0 so a reorg or an out-of-order RPC
// response cannot produce a misleading negative lag.
func ObserveLedgerLag(network string, lag int64) {
	if lag < 0 {
		lag = 0
	}
	LedgerLag.WithLabelValues(network).Set(float64(lag))
}

// AddEventsProcessed adds n successfully persisted events for a network.
func AddEventsProcessed(network string, n int) {
	if n <= 0 {
		return
	}
	EventsProcessedTotal.WithLabelValues(network).Add(float64(n))
}

// ObserveRunDuration records the duration of a completed pass.
func ObserveRunDuration(mode string, seconds float64) {
	RunDuration.WithLabelValues(mode).Observe(seconds)
}

// NewAdminMux returns a mux serving only the metrics endpoint. It backs the
// optional METRICS_PORT listener so the indexer can be scraped without
// exposing anything else.
func NewAdminMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", Handler)
	return mux
}
