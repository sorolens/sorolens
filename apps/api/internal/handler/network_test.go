package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const (
	netContractA = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	netContractB = "CBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
)

func seedMultiNetworkStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{
		ID:      netContractA,
		Network: "testnet",
		Label:   "testnet contract",
		Status:  "active",
	}); err != nil {
		t.Fatal(err)
	}
	if err := ms.UpsertContract(nil, store.Contract{
		ID:      netContractB,
		Network: "mainnet",
		Label:   "mainnet contract",
		Status:  "active",
	}); err != nil {
		t.Fatal(err)
	}
	if err := ms.UpsertContract(nil, store.Contract{
		ID:      "CFUTURENETCONTRACT0000000000000000000000000000000000000",
		Network: "futurenet",
		Label:   "futurenet contract",
		Status:  "paused",
	}); err != nil {
		t.Fatal(err)
	}

	// Two events on the same contract, one per network, so the network
	// filter is exercised independently of the contract.
	if err := ms.BatchInsertEvents(nil, []store.Event{
		{
			ID: "evt-testnet", ContractID: netContractA, Network: "testnet",
			Ledger: 100, LedgerClosedAt: time.Now().UTC(), TxHash: "tx1", Type: "contract",
		},
		{
			ID: "evt-mainnet", ContractID: netContractA, Network: "mainnet",
			Ledger: 101, LedgerClosedAt: time.Now().UTC(), TxHash: "tx2", Type: "contract",
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := ms.UpsertMonitoredContract(nil, store.MonitoredContract{
		ContractID: "CWATCHTESTNET", Network: "testnet", Name: "tn", Owner: "G",
		Status: "Healthy", RegisteredAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := ms.UpsertMonitoredContract(nil, store.MonitoredContract{
		ContractID: "CWATCHMAINNET", Network: "mainnet", Name: "mn", Owner: "G",
		Status: "Degraded", RegisteredAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	return ms
}

func decodeContracts(t *testing.T, body []byte) []map[string]any {
	t.Helper()
	var parsed struct {
		Contracts []map[string]any `json:"contracts"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("decode contracts: %v", err)
	}
	return parsed.Contracts
}

func TestListContractsFiltersByNetwork(t *testing.T) {
	srv := newTestHandler(seedMultiNetworkStore(t), true, true)

	// No filter: all three networks are returned.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if got := len(decodeContracts(t, w.Body.Bytes())); got != 3 {
		t.Fatalf("unfiltered: want 3 contracts, got %d", got)
	}

	// Single-network filter.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/contracts?network=mainnet", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	contracts := decodeContracts(t, w.Body.Bytes())
	if len(contracts) != 1 {
		t.Fatalf("mainnet filter: want 1 contract, got %d", len(contracts))
	}
	if contracts[0]["network"] != "mainnet" {
		t.Errorf("want network=mainnet, got %v", contracts[0]["network"])
	}

	// "all" is treated as "no filter".
	req = httptest.NewRequest(http.MethodGet, "/api/v1/contracts?network=all", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if got := len(decodeContracts(t, w.Body.Bytes())); got != 3 {
		t.Fatalf("network=all: want 3 contracts, got %d", got)
	}
}

func TestListContractsCombinesNetworkAndStatus(t *testing.T) {
	srv := newTestHandler(seedMultiNetworkStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts?network=testnet&status=active", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	contracts := decodeContracts(t, w.Body.Bytes())
	if len(contracts) != 1 || contracts[0]["id"] != netContractA {
		t.Fatalf("want only %s, got %+v", netContractA, contracts)
	}

	// A network/status pair with no match returns an empty list, not an error.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/contracts?network=mainnet&status=paused", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if got := len(decodeContracts(t, w.Body.Bytes())); got != 0 {
		t.Fatalf("want 0 contracts, got %d", got)
	}
}

func TestListContractsIncludesLastActivity(t *testing.T) {
	lastActivity := time.Date(2026, 9, 25, 5, 42, 0, 0, time.UTC)
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{
		ID: "CLASTACTIVITY", Network: "testnet", Label: "active", Status: "active",
		LastActivityAt: &lastActivity,
	}); err != nil {
		t.Fatal(err)
	}

	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	contracts := decodeContracts(t, w.Body.Bytes())
	if len(contracts) != 1 {
		t.Fatalf("want 1 contract, got %d", len(contracts))
	}
	if got := contracts[0]["last_activity_at"]; got != lastActivity.Format(time.RFC3339) {
		t.Fatalf("want last_activity_at=%q, got %v", lastActivity.Format(time.RFC3339), got)
	}
}

func TestListContractsRejectsUnknownNetwork(t *testing.T) {
	srv := newTestHandler(seedMultiNetworkStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts?network=devnet", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", w.Code)
	}
}

func TestListEventsFiltersByNetwork(t *testing.T) {
	srv := newTestHandler(seedMultiNetworkStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/contracts/"+netContractA+"/events?network=mainnet", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var parsed struct {
		Events []map[string]any `json:"events"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Events) != 1 {
		t.Fatalf("want 1 mainnet event, got %d", len(parsed.Events))
	}
	if parsed.Events[0]["network"] != "mainnet" {
		t.Errorf("want network=mainnet, got %v", parsed.Events[0]["network"])
	}
}

func TestWatchdogFiltersByNetwork(t *testing.T) {
	srv := newTestHandler(seedMultiNetworkStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/watchdog/contracts?network=mainnet", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var parsed struct {
		Contracts []map[string]any `json:"contracts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Contracts) != 1 || parsed.Contracts[0]["contract_id"] != "CWATCHMAINNET" {
		t.Fatalf("want only mainnet watchdog contract, got %+v", parsed.Contracts)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/watchdog/stats?network=testnet", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var stats map[string]int64
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	if stats["total_monitored"] != 1 || stats["healthy"] != 1 {
		t.Fatalf("want 1 healthy testnet contract, got %+v", stats)
	}
}
