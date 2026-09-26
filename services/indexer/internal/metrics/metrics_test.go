package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecorderObserveNetwork(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		head        uint32
		lastIndexed uint32
		wantLag     float64
	}{
		{name: "behind", head: 1000, lastIndexed: 940, wantLag: 60},
		{name: "caught up", head: 1000, lastIndexed: 1000, wantLag: 0},
		{name: "cursor ahead of head clamps to zero", head: 1000, lastIndexed: 1010, wantLag: 0},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := New()
			r.ObserveNetwork("testnet", tc.head, tc.lastIndexed)

			if got := testutil.ToFloat64(r.lagLedgers.WithLabelValues("testnet")); got != tc.wantLag {
				t.Fatalf("lag = %v, want %v", got, tc.wantLag)
			}
			if got := testutil.ToFloat64(r.headLedger.WithLabelValues("testnet")); got != float64(tc.head) {
				t.Fatalf("head ledger = %v, want %v", got, tc.head)
			}
			if got := testutil.ToFloat64(r.lastIndexedLedger.WithLabelValues("testnet")); got != float64(tc.lastIndexed) {
				t.Fatalf("last indexed ledger = %v, want %v", got, tc.lastIndexed)
			}
		})
	}
}

func TestRecorderObserveNetworkIsPerNetwork(t *testing.T) {
	t.Parallel()

	r := New()
	r.ObserveNetwork("testnet", 1000, 900)
	r.ObserveNetwork("mainnet", 2000, 1990)

	if got := testutil.ToFloat64(r.lagLedgers.WithLabelValues("testnet")); got != 100 {
		t.Fatalf("testnet lag = %v, want 100", got)
	}
	if got := testutil.ToFloat64(r.lagLedgers.WithLabelValues("mainnet")); got != 10 {
		t.Fatalf("mainnet lag = %v, want 10", got)
	}
}

func TestRecorderNilObserveNetworkIsNoop(t *testing.T) {
	t.Parallel()

	var r *Recorder
	r.ObserveNetwork("testnet", 1000, 900) // must not panic
	r.AddEventsProcessed("testnet", 5)     // must not panic
	r.ObserveRunDuration("once", 1.5)      // must not panic
}

func TestRecorderAddEventsProcessed(t *testing.T) {
	t.Parallel()

	r := New()
	r.AddEventsProcessed("testnet", 3)
	r.AddEventsProcessed("testnet", 2)

	if got := testutil.ToFloat64(r.eventsProcessed.WithLabelValues("testnet")); got != 5 {
		t.Fatalf("events processed = %v, want 5", got)
	}

	// A zero increment must not create a series for a network.
	r.AddEventsProcessed("futurenet", 0)
	if got := testutil.CollectAndCount(r.eventsProcessed); got != 1 {
		t.Fatalf("events_processed_total series = %d, want 1 (zero increment created a series)", got)
	}
}

func TestRecorderObserveRunDuration(t *testing.T) {
	t.Parallel()

	r := New()
	r.ObserveRunDuration("once", 1.5)

	if got := testutil.CollectAndCount(r.runDuration); got != 1 {
		t.Fatalf("run_duration_seconds series = %d, want 1", got)
	}
}

func TestHandlerExposesLagMetric(t *testing.T) {
	t.Parallel()

	r := New()
	r.ObserveNetwork("mainnet", 5000, 4880)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	body := rec.Body.String()
	for _, want := range []string{
		"sorolens_indexer_lag_ledgers{network=\"mainnet\"} 120",
		"sorolens_indexer_head_ledger{network=\"mainnet\"} 5000",
		"sorolens_indexer_last_indexed_ledger{network=\"mainnet\"} 4880",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("/metrics body missing %q\nbody:\n%s", want, body)
		}
	}
}

// TestHandlerExposesEventsAndRunDuration pins the names required by the
// metrics endpoint acceptance criteria: the event counter and pass-duration
// histogram must be present once observed.
func TestHandlerExposesEventsAndRunDuration(t *testing.T) {
	t.Parallel()

	r := New()
	r.AddEventsProcessed("testnet", 3)
	r.ObserveRunDuration("once", 1.5)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, want := range []string{
		"sorolens_indexer_events_processed_total",
		"sorolens_indexer_run_duration_seconds",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("/metrics body missing %q\nbody:\n%s", want, body)
		}
	}
}
