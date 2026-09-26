// Package metrics owns the Prometheus collectors the Sorolens indexer exposes
// on its /metrics endpoint (issues #198 and #121).
//
// Every collector is registered on a dedicated registry owned by Recorder so
// callers can run more than one recorder (for example in tests) without
// colliding on the process-wide default registry. HTTP handlers are produced
// with promhttp, the standard Prometheus exposition handler.
//
// See docs/metrics.md for the full metric catalogue.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// namespace is the metric namespace shared by every Sorolens metric. It is the
// "<prefix>" in names such as sorolens_indexer_lag_ledgers.
const namespace = "sorolens"

// Recorder registers and updates the indexer's Prometheus metrics.
// The zero value is not usable; construct one with New.
type Recorder struct {
	registry *prometheus.Registry

	lagLedgers        *prometheus.GaugeVec
	headLedger        *prometheus.GaugeVec
	lastIndexedLedger *prometheus.GaugeVec

	eventsProcessed *prometheus.CounterVec
	runDuration     *prometheus.HistogramVec
}

// New returns a Recorder with all indexer collectors registered on a new
// registry. The returned Recorder is safe for concurrent use.
func New() *Recorder {
	r := &Recorder{
		registry: prometheus.NewRegistry(),
		lagLedgers: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "indexer",
			Name:      "lag_ledgers",
			Help: "Number of ledgers the indexer is behind the network head " +
				"(latest_ledger - last_indexed_ledger) for the network.",
		}, []string{"network"}),
		headLedger: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "indexer",
			Name:      "head_ledger",
			Help:      "Latest ledger sequence reported by the Soroban RPC for the network.",
		}, []string{"network"}),
		lastIndexedLedger: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "indexer",
			Name:      "last_indexed_ledger",
			Help:      "Last ledger sequence committed by the indexer for the network.",
		}, []string{"network"}),
		// EventsProcessedTotal counts contract events successfully written.
		// It is only incremented after a batch has committed, so the series
		// always describes durable progress.
		eventsProcessed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "indexer",
			Name:      "events_processed_total",
			Help:      "Total number of contract events processed and persisted, by network.",
		}, []string{"network"}),
		// RunDuration observes the wall-clock duration of a complete indexer
		// pass. Buckets extend to 300s to cover the INDEXER_MAX_DURATION
		// default of 270s.
		runDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "indexer",
			Name:      "run_duration_seconds",
			Help:      "Duration of a full indexer pass in seconds, by run mode.",
			Buckets:   []float64{0.5, 1, 2.5, 5, 10, 30, 60, 120, 270, 300},
		}, []string{"mode"}),
	}
	r.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),

		r.lagLedgers,
		r.headLedger,
		r.lastIndexedLedger,
		r.eventsProcessed,
		r.runDuration,
	)
	return r
}

// ObserveNetwork records one lag sample for a network. head is the latest
// ledger reported by the network's RPC and lastIndexed is the last ledger the
// indexer has committed for that network (its network cursor). Lag is clamped
// at zero so a regressed cursor never reports a negative gauge.
func (r *Recorder) ObserveNetwork(network string, head, lastIndexed uint32) {
	if r == nil {
		return
	}
	var lag uint32
	if head > lastIndexed {
		lag = head - lastIndexed
	}
	r.headLedger.WithLabelValues(network).Set(float64(head))
	r.lastIndexedLedger.WithLabelValues(network).Set(float64(lastIndexed))
	r.lagLedgers.WithLabelValues(network).Set(float64(lag))
}

// AddEventsProcessed adds n successfully persisted events for a network. A
// zero (or negative) increment is ignored so an idle pass never creates an
// empty series.
func (r *Recorder) AddEventsProcessed(network string, n int) {
	if r == nil || n <= 0 {
		return
	}
	r.eventsProcessed.WithLabelValues(network).Add(float64(n))
}

// ObserveRunDuration records the duration of a completed pass.
func (r *Recorder) ObserveRunDuration(mode string, seconds float64) {
	if r == nil {
		return
	}
	r.runDuration.WithLabelValues(mode).Observe(seconds)
}

// Registry returns the registry holding the indexer collectors.
func (r *Recorder) Registry() *prometheus.Registry {
	if r == nil {
		return nil
	}
	return r.registry
}

// Handler returns the HTTP handler that serves the Prometheus text exposition
// on the /metrics endpoint.
func (r *Recorder) Handler() http.Handler {
	return promhttp.HandlerFor(r.Registry(), promhttp.HandlerOpts{})
}
