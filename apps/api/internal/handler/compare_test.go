package handler_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// compareID builds a valid 56-character Soroban contract ID ('C' + 55 chars)
// whose tail keeps fixtures distinguishable.
func compareID(tail string) string {
	base := "C" + strings.Repeat("A", 55)
	if tail == "" {
		return base
	}
	return base[:len(base)-len(tail)] + tail
}

// seedCompareContract registers a contract and seeds `events` events and
// `invocations` invocations within the trailing hour so the hourly activity
// window always includes them, plus an optional cached health score.
func seedCompareContract(t *testing.T, ms *store.MockStore, id, network string, events, invocations int, cpuPer, feePer int64, score *int32) {
	t.Helper()
	if err := ms.UpsertContract(nil, store.Contract{ID: id, Network: network, Status: "active"}); err != nil {
		t.Fatalf("upsert contract: %v", err)
	}
	now := time.Now().UTC()
	evs := make([]store.Event, 0, events)
	for i := 0; i < events; i++ {
		evs = append(evs, store.Event{
			ID:             fmt.Sprintf("%s-ev-%d", id, i),
			ContractID:     id,
			Network:        network,
			Ledger:         uint32(100 + i),
			LedgerClosedAt: now.Add(-time.Duration(i) * time.Minute),
			Type:           "transfer",
		})
	}
	if err := ms.BatchInsertEvents(nil, evs); err != nil {
		t.Fatalf("insert events: %v", err)
	}
	invs := make([]store.Invocation, 0, invocations)
	for i := 0; i < invocations; i++ {
		invs = append(invs, store.Invocation{
			TxHash:             fmt.Sprintf("%s-inv-%d", id, i),
			ContractID:         id,
			Network:            network,
			Ledger:             uint32(100 + i),
			LedgerClosedAt:     now.Add(-time.Duration(i) * time.Minute),
			Status:             "SUCCESS",
			CPUInsn:            cpuPer,
			ResourceFeeCharged: feePer,
		})
	}
	if err := ms.BatchInsertInvocations(nil, invs); err != nil {
		t.Fatalf("insert invocations: %v", err)
	}
	if score != nil {
		if err := ms.UpsertContractHealthScore(nil, store.ContractHealthScore{
			ContractID: id,
			Score:      *score,
			ComputedAt: now,
		}); err != nil {
			t.Fatalf("upsert health score: %v", err)
		}
	}
}

// compareResponse mirrors the handler's JSON so tests assert the wire shape,
// not the unexported Go struct.
type compareResponse struct {
	Window    string `json:"window"`
	Contracts []struct {
		ID              string  `json:"id"`
		Network         string  `json:"network"`
		Status          string  `json:"status"`
		Tracked         bool    `json:"tracked"`
		HasData         bool    `json:"has_data"`
		EventCount      int64   `json:"event_count"`
		InvocationCount int64   `json:"invocation_count"`
		AvgCPU          float64 `json:"avg_cpu"`
		AvgFee          float64 `json:"avg_fee"`
		HealthScore     *int32  `json:"health_score"`
		EventVolume     []struct {
			Timestamp string `json:"timestamp"`
			Count     int64  `json:"count"`
		} `json:"event_volume"`
		Error string `json:"error"`
	} `json:"contracts"`
}

func doCompare(t *testing.T, ms *store.MockStore, query string) (*httptest.ResponseRecorder, compareResponse) {
	t.Helper()
	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/compare"+query, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp compareResponse
	if w.Code == http.StatusOK {
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode compare response: %v", err)
		}
	}
	return w, resp
}

// TestCompareContracts_Validation is table-driven over the malformed inputs
// the endpoint must reject before any fan-out happens.
func TestCompareContracts_Validation(t *testing.T) {
	ids5 := []string{
		compareID("A1"), compareID("A2"), compareID("A3"), compareID("A4"), compareID("A5"),
	}
	cases := []struct {
		name  string
		query string
	}{
		{"missing ids", ""},
		{"empty ids", "?ids="},
		{"blank entry", "?ids=" + compareID("A1") + ",,"},
		{"too many ids", "?ids=" + strings.Join(ids5, ",")},
		{"malformed id", "?ids=not-a-contract"},
		{"malformed among valid", "?ids=" + compareID("A1") + ",abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := doCompare(t, store.NewMockStore(), tc.query)
			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("want 422, got %d (%s)", w.Code, w.Body.String())
			}
			var env map[string]any
			_ = json.NewDecoder(w.Body).Decode(&env)
			errObj, _ := env["error"].(map[string]any)
			if errObj["code"] != "INVALID_INPUT" {
				t.Errorf("want code=INVALID_INPUT, got %v", errObj["code"])
			}
		})
	}
}

// TestCompareContracts_MixedNetworks covers the happy path: two contracts on
// different networks returned in one response array, with per-contract stats,
// averages derived from hourly activity, and cached health scores.
func TestCompareContracts_MixedNetworks(t *testing.T) {
	testnet := compareID("A1")
	mainnet := compareID("B1")
	scoreHigh, scoreLow := int32(91), int32(38)

	ms := store.NewMockStore()
	seedCompareContract(t, ms, testnet, "testnet", 3, 2, 1000, 200, &scoreHigh)
	seedCompareContract(t, ms, mainnet, "mainnet", 1, 1, 5000, 100, &scoreLow)

	w, resp := doCompare(t, ms, "?ids="+testnet+","+mainnet+"&window=7d")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if resp.Window != "7d" {
		t.Errorf("window = %q, want 7d", resp.Window)
	}
	if len(resp.Contracts) != 2 {
		t.Fatalf("want 2 contracts, got %d", len(resp.Contracts))
	}

	got := map[string]int{}
	for i, c := range resp.Contracts {
		got[c.ID] = i
	}

	sn := resp.Contracts[got[testnet]]
	if sn.Network != "testnet" || !sn.Tracked || !sn.HasData {
		t.Errorf("testnet entry = %+v", sn)
	}
	if sn.EventCount != 3 || sn.InvocationCount != 2 {
		t.Errorf("testnet counts = (%d,%d), want (3,2)", sn.EventCount, sn.InvocationCount)
	}
	if sn.AvgCPU != 1000 || sn.AvgFee != 200 {
		t.Errorf("testnet averages = (%v,%v), want (1000,200)", sn.AvgCPU, sn.AvgFee)
	}
	if sn.HealthScore == nil || *sn.HealthScore != 91 {
		t.Errorf("testnet health_score = %v, want 91", sn.HealthScore)
	}
	if len(sn.EventVolume) != 24*7 {
		t.Errorf("testnet sparkline points = %d, want %d", len(sn.EventVolume), 24*7)
	}
	sparkTotal := int64(0)
	for _, p := range sn.EventVolume {
		sparkTotal += p.Count
	}
	if sparkTotal != 3 {
		t.Errorf("testnet sparkline total = %d, want 3", sparkTotal)
	}

	mn := resp.Contracts[got[mainnet]]
	if mn.Network != "mainnet" || !mn.HasData {
		t.Errorf("mainnet entry = %+v", mn)
	}
	if mn.AvgCPU != 5000 || mn.AvgFee != 100 {
		t.Errorf("mainnet averages = (%v,%v), want (5000,100)", mn.AvgCPU, mn.AvgFee)
	}
	if mn.HealthScore == nil || *mn.HealthScore != 38 {
		t.Errorf("mainnet health_score = %v, want 38", mn.HealthScore)
	}
}

// TestCompareContracts_NoDataContract verifies a tracked-but-not-yet-indexed
// contract yields a present entry with zero metrics instead of an error, and
// that the sparkline is still a well-formed zero series.
func TestCompareContracts_NoDataContract(t *testing.T) {
	id := compareID("C1")
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{ID: id, Network: "futurenet", Status: "pending"}); err != nil {
		t.Fatal(err)
	}

	w, resp := doCompare(t, ms, "?ids="+id)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if len(resp.Contracts) != 1 {
		t.Fatalf("want 1 contract, got %d", len(resp.Contracts))
	}
	c := resp.Contracts[0]
	if !c.Tracked {
		t.Error("tracked = false, want true")
	}
	if c.HasData {
		t.Error("has_data = true, want false")
	}
	if c.EventCount != 0 || c.InvocationCount != 0 || c.AvgCPU != 0 || c.AvgFee != 0 {
		t.Errorf("want zero metrics, got %+v", c)
	}
	if c.HealthScore != nil {
		t.Errorf("health_score = %v, want null", *c.HealthScore)
	}
	if c.Error != "" {
		t.Errorf("error = %q, want empty", c.Error)
	}
	if len(c.EventVolume) != 24 {
		t.Errorf("sparkline points = %d, want 24", len(c.EventVolume))
	}
}

// TestCompareContracts_UntrackedContract verifies an ID that is not tracked at
// all still comes back as a present, empty entry.
func TestCompareContracts_UntrackedContract(t *testing.T) {
	id := compareID("D1")
	w, resp := doCompare(t, store.NewMockStore(), "?ids="+id)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if len(resp.Contracts) != 1 {
		t.Fatalf("want 1 contract, got %d", len(resp.Contracts))
	}
	c := resp.Contracts[0]
	if c.Tracked || c.HasData || c.Error != "" {
		t.Errorf("untracked entry = %+v", c)
	}
}

// TestCompareContracts_QueryErrorIsolatesContract verifies a stats lookup
// failure on one contract is reported on its entry without failing the
// request.
func TestCompareContracts_QueryErrorIsolatesContract(t *testing.T) {
	id := compareID("E1")
	ms := store.NewMockStore()
	ms.GetContractStatsErr = errors.New("boom")

	w, resp := doCompare(t, ms, "?ids="+id)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if len(resp.Contracts) != 1 || resp.Contracts[0].Error == "" {
		t.Errorf("want an isolated error entry, got %+v", resp.Contracts)
	}
}
