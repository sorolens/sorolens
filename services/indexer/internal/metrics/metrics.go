// Package metrics owns the Prometheus collectors the Sorolens indexer exposes
// on its /metrics endpoint (issue #198).
//
// Every collector is registered on a dedicated registry owned by Recorder so
// callers can run more than one recorder (for example in tests) without
// colliding on the process-wide default registry. HTTP handlers are produced
// with promhttp, the standard Prometheus exposition handler.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
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
	}
	r.registry.MustRegister(r.lagLedgers, r.headLedger, r.lastIndexedLedger)
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
