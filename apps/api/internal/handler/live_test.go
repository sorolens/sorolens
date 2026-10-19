package handler_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestRecentEventsReturnsNewestAcrossContracts pins the ticker feed: the
// newest events from every contract, newest first, capped at limit.
func TestRecentEventsReturnsNewestAcrossContracts(t *testing.T) {
	ms, a, b, _ := seedLiveStore(t)
	base := time.Now().UTC().Truncate(time.Minute)
	ms.BatchInsertEvents(context.Background(), []store.Event{
		{ID: "old-a", ContractID: a, Network: "testnet", Ledger: 1, LedgerClosedAt: base.Add(-10 * time.Minute)},
		{ID: "new-b", ContractID: b, Network: "testnet", Ledger: 3, LedgerClosedAt: base.Add(-1 * time.Minute)},
		{ID: "mid-a", ContractID: a, Network: "testnet", Ledger: 2, LedgerClosedAt: base.Add(-5 * time.Minute)},
	})
	srv := newTestHandler(ms, true, true)

	code, body := getJSON(t, srv, "/api/v1/events/recent?limit=2")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d", code)
	}
	events, ok := body["events"].([]any)
	if !ok {
		t.Fatalf("want events array, got %T", body["events"])
	}
	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d", len(events))
	}
	first, _ := events[0].(map[string]any)
	if first["id"] != "new-b" {
		t.Errorf("want newest event first, got %v", first["id"])
	}
	second, _ := events[1].(map[string]any)
	if second["id"] != "mid-a" {
		t.Errorf("want second-newest second, got %v", second["id"])
	}
}

// TestLiveActivityBuckets pins the sparkline feed: a contiguous per-minute
// series whose length always equals the requested window.
func TestLiveActivityBuckets(t *testing.T) {
	ms, a, b, _ := seedLiveStore(t)
	now := time.Now().UTC()

	// Alpha emits three events in the current minute and one a minute ago;
	// Beta emits one event in the current minute.
	ms.BatchInsertEvents(context.Background(), []store.Event{
		{ID: "a1", ContractID: a, Network: "testnet", Ledger: 1, LedgerClosedAt: now},
		{ID: "a2", ContractID: a, Network: "testnet", Ledger: 2, LedgerClosedAt: now},
		{ID: "a3", ContractID: a, Network: "testnet", Ledger: 3, LedgerClosedAt: now},
		{ID: "a4", ContractID: a, Network: "testnet", Ledger: 4, LedgerClosedAt: now.Add(-time.Minute)},
		{ID: "b1", ContractID: b, Network: "testnet", Ledger: 5, LedgerClosedAt: now},
	})
	srv := newTestHandler(ms, true, true)

	code, body := getJSON(t, srv, "/api/v1/stats/activity?minutes=5")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d", code)
	}
	if body["minutes"] != float64(5) {
		t.Errorf("want minutes=5, got %v", body["minutes"])
	}
	windowStart, ok := body["window_start"].(string)
	if !ok {
		t.Fatalf("want window_start string, got %T", body["window_start"])
	}
	if _, err := time.Parse(time.RFC3339, windowStart); err != nil {
		t.Errorf("window_start %q is not RFC3339: %v", windowStart, err)
	}

	contracts, ok := body["contracts"].([]any)
	if !ok {
		t.Fatalf("want contracts array, got %T", body["contracts"])
	}
	if len(contracts) != 2 {
		t.Fatalf("want 2 active contracts, got %d", len(contracts))
	}

	// Hottest first: Alpha has 4 events, Beta has 1.
	alpha, _ := contracts[0].(map[string]any)
	if alpha["contract_id"] != a {
		t.Errorf("want hottest contract first, got %v", alpha["contract_id"])
	}
	if alpha["total"] != float64(4) {
		t.Errorf("want total=4, got %v", alpha["total"])
	}
	if alpha["label"] != "Alpha" {
		t.Errorf("want label Alpha, got %v", alpha["label"])
	}
	series, ok := alpha["per_minute"].([]any)
	if !ok {
		t.Fatalf("want per_minute array, got %T", alpha["per_minute"])
	}
	if len(series) != 5 {
		t.Fatalf("want a 5-bucket series, got %d", len(series))
	}
	// The series is oldest-first, so the last bucket is the current minute.
	if series[4] != float64(3) {
		t.Errorf("want 3 events in the current minute, got %v", series[4])
	}
	var sum float64
	for _, v := range series {
		sum += v.(float64)
	}
	if sum != 4 {
		t.Errorf("want bucket sum to equal total (4), got %v", sum)
	}
}

// TestContractEventRatesWindowAlignment keeps two calls taken seconds apart on
// the same x-axis, which is what stops the sparkline jittering on refresh.
func TestContractEventRatesWindowAlignment(t *testing.T) {
	now := time.Date(2026, 3, 4, 10, 20, 45, 0, time.UTC)
	later := now.Add(7 * time.Second)

	if got, want := store.LiveWindowStart(5, now), time.Date(2026, 3, 4, 10, 16, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("want window start %s, got %s", want, got)
	}
	if !store.LiveWindowStart(5, now).Equal(store.LiveWindowStart(5, later)) {
		t.Error("window start must be stable within the same wall-clock minute")
	}
}
