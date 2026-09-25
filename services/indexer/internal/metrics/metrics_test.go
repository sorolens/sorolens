package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sorolens/sorolens/services/indexer/internal/metrics"
)

// requiredMetrics are the three indexer-owned series named in the acceptance
// criteria for the metrics endpoint.
var requiredMetrics = []string{
	"sorolens_indexer_ledger_lag",
	"sorolens_indexer_events_processed_total",
	"sorolens_indexer_run_duration_seconds",
}

// TestIndexerMetricsExposeRequiredNames scrapes the indexer metrics mux and
// asserts every required metric name is present in a valid exposition.
func TestIndexerMetricsExposeRequiredNames(t *testing.T) {
	// Populate each series: a GaugeVec/CounterVec/HistogramVec emits nothing
	// until it has been observed at least once.
	metrics.ObserveLedgerLag("testnet", 42)
	metrics.AddEventsProcessed("testnet", 3)
	metrics.ObserveRunDuration("once", 1.5)

	rr := httptest.NewRecorder()
	metrics.NewAdminMux().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /metrics: got status %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("GET /metrics: got Content-Type %q, want a text/plain exposition", ct)
	}

	body := rr.Body.String()
	for _, name := range requiredMetrics {
		if !strings.Contains(body, name) {
			t.Errorf("GET /metrics: exposition is missing metric %q; body:\n%s", name, body)
		}
	}

	// Spot-check that the values recorded actually reach the exposition.
	if !strings.Contains(body, `sorolens_indexer_ledger_lag{network="testnet"} 42`) {
		t.Errorf("expected ledger lag 42 for testnet in exposition; body:\n%s", body)
	}
	if !strings.Contains(body, "sorolens_indexer_run_duration_seconds_count{mode=\"once\"}") {
		t.Errorf("expected a run-duration observation for mode=once; body:\n%s", body)
	}
}

// TestObserveLedgerLagClampsNegative documents that a negative lag (a reorg, or
// an RPC response that reports a lower head than the ledger just processed) is
// clamped to 0 rather than reported as a meaningless negative value.
func TestObserveLedgerLagClampsNegative(t *testing.T) {
	metrics.ObserveLedgerLag("mainnet", -5)

	rr := httptest.NewRecorder()
	metrics.NewAdminMux().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if body := rr.Body.String(); !strings.Contains(body, `sorolens_indexer_ledger_lag{network="mainnet"} 0`) {
		t.Errorf("expected negative lag clamped to 0; body:\n%s", body)
	}
}

// TestAddEventsProcessedIgnoresZero guards against noisy zero increments.
func TestAddEventsProcessedIgnoresZero(t *testing.T) {
	metrics.AddEventsProcessed("futurenet", 0)

	rr := httptest.NewRecorder()
	metrics.NewAdminMux().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if body := rr.Body.String(); strings.Contains(body, `network="futurenet"`) {
		t.Errorf("zero increments should not create a series; body:\n%s", body)
	}
}
