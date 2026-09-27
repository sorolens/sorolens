package rulesengine

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	rulelang "github.com/sorolens/sorolens/apps/api/rulelang"
)

type fakeStore struct {
	rules      []Rule
	contracts  []Contract
	samples    map[string][]Sample
	alerts     []Alert
	failInsert bool
}

func (f *fakeStore) ListEnabledAlertRules(context.Context) ([]Rule, error) { return f.rules, nil }
func (f *fakeStore) ListContracts(context.Context) ([]Contract, error)     { return f.contracts, nil }
func (f *fakeStore) ContractMetricSamples(_ context.Context, contractID string, _, _ time.Time) ([]Sample, error) {
	return f.samples[contractID], nil
}
func (f *fakeStore) InsertAlert(_ context.Context, a Alert) error {
	if f.failInsert {
		return io.ErrUnexpectedEOF
	}
	f.alerts = append(f.alerts, a)
	return nil
}

// testNow is the clock the engine is pinned to in tests. series() builds its
// buckets relative to it, so the two must agree or the engine's sample window
// will not line up with the samples.
var testNow = time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

func testEngine(store Store) *Engine {
	e := New(store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	e.now = func() time.Time { return testNow }
	return e
}

// series builds per-minute buckets ending at testNow, with the given error_rate.
func series(values ...float64) []Sample {
	now := testNow
	start := now.Add(-time.Duration(len(values)) * time.Minute)
	out := make([]Sample, 0, len(values))
	for i, v := range values {
		out = append(out, Sample{
			At:     start.Add(time.Duration(i) * time.Minute),
			Values: map[string]float64{"error_rate": v, "invocations": 10, "events": 5},
		})
	}
	return out
}

func TestEngineFiresAndInsertsAlert(t *testing.T) {
	store := &fakeStore{
		rules:     []Rule{{ID: 1, Name: "high error rate", Source: "error_rate > 0.5 for 3m", Severity: "Warning"}},
		contracts: []Contract{{ID: "CABC", Network: "testnet"}},
		samples:   map[string][]Sample{"CABC": series(0.9, 0.8, 0.7)},
	}
	if err := testEngine(store).EvaluatePass(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.alerts) != 1 {
		t.Fatalf("alerts = %d, want 1", len(store.alerts))
	}
	a := store.alerts[0]
	if a.ContractID != "CABC" || a.Severity != "Warning" || a.Rule != "high error rate" {
		t.Fatalf("unexpected alert %+v", a)
	}
	if a.TxHash == "" {
		t.Fatal("alert should carry a deterministic dedupe key")
	}
}

func TestEngineDoesNotFireBelowThreshold(t *testing.T) {
	store := &fakeStore{
		rules:     []Rule{{ID: 1, Name: "r", Source: "error_rate > 0.5 for 3m"}},
		contracts: []Contract{{ID: "CABC", Network: "testnet"}},
		samples:   map[string][]Sample{"CABC": series(0.9, 0.1, 0.9)},
	}
	if err := testEngine(store).EvaluatePass(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.alerts) != 0 {
		t.Fatalf("alerts = %d, want 0", len(store.alerts))
	}
}

func TestEngineRespectsNetworkScope(t *testing.T) {
	store := &fakeStore{
		rules: []Rule{{ID: 1, Name: "r", Source: "error_rate > 0.5 for 3m", Network: "mainnet"}},
		contracts: []Contract{
			{ID: "CTEST", Network: "testnet"},
			{ID: "CMAIN", Network: "mainnet"},
		},
		samples: map[string][]Sample{
			"CTEST": series(0.9, 0.9, 0.9),
			"CMAIN": series(0.9, 0.9, 0.9),
		},
	}
	if err := testEngine(store).EvaluatePass(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.alerts) != 1 || store.alerts[0].ContractID != "CMAIN" {
		t.Fatalf("alerts = %+v, want one for CMAIN", store.alerts)
	}
}

func TestEngineContractScopedRuleSkipsFleetLoad(t *testing.T) {
	store := &fakeStore{
		rules: []Rule{{ID: 1, Name: "r", Source: "error_rate > 0.5 for 3m", ContractID: "CONLY"}},
		// contracts intentionally empty: a scoped rule must not need them.
		samples: map[string][]Sample{"CONLY": series(0.9, 0.9, 0.9)},
	}
	if err := testEngine(store).EvaluatePass(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.alerts) != 1 || store.alerts[0].ContractID != "CONLY" {
		t.Fatalf("alerts = %+v, want one for CONLY", store.alerts)
	}
}

func TestEngineSkipsInvalidStoredRule(t *testing.T) {
	store := &fakeStore{
		rules:     []Rule{{ID: 1, Name: "bad", Source: "not a rule"}},
		contracts: []Contract{{ID: "CABC", Network: "testnet"}},
		samples:   map[string][]Sample{"CABC": series(0.9)},
	}
	if err := testEngine(store).EvaluatePass(context.Background()); err != nil {
		t.Fatalf("invalid rule should be skipped, not fail the pass: %v", err)
	}
	if len(store.alerts) != 0 {
		t.Fatalf("alerts = %d, want 0", len(store.alerts))
	}
}

func TestEngineDefaultsSeverityAndWindow(t *testing.T) {
	store := &fakeStore{
		rules:     []Rule{{ID: 1, Name: "r", Source: "error_rate > 0.5"}}, // no window, no severity
		contracts: []Contract{{ID: "CABC", Network: "testnet"}},
		samples:   map[string][]Sample{"CABC": series(0.1, 0.2, 0.9)},
	}
	if err := testEngine(store).EvaluatePass(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.alerts) != 1 {
		t.Fatalf("alerts = %d, want 1", len(store.alerts))
	}
	if store.alerts[0].Severity != "Warning" {
		t.Fatalf("severity = %q, want Warning", store.alerts[0].Severity)
	}
}

func TestLibraryRulesEvaluateWithoutError(t *testing.T) {
	// Every library rule must at least evaluate (no data path included).
	for _, lr := range rulelang.Library() {
		store := &fakeStore{
			rules:     []Rule{{ID: 1, Name: lr.Name, Source: lr.Source, Severity: lr.Severity}},
			contracts: []Contract{{ID: "CABC", Network: "testnet"}},
			samples:   map[string][]Sample{"CABC": series(0.9, 0.9, 0.9)},
		}
		if err := testEngine(store).EvaluatePass(context.Background()); err != nil {
			t.Errorf("library rule %q: %v", lr.Name, err)
		}
	}
}
