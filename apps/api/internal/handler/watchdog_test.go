package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func seededWatchdogStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()
	if err := ms.UpsertMonitoredContract(nil, store.MonitoredContract{
		ContractID:    "CONTRACT_A",
		Name:          "app_v1",
		Owner:         "GOWNER",
		Status:        "Healthy",
		CheckInterval: 300,
		RegisteredAt:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	if err := ms.UpsertMonitoredContract(nil, store.MonitoredContract{
		ContractID:    "CONTRACT_B",
		Name:          "worker",
		Owner:         "GOWNER",
		Status:        "Degraded",
		CheckInterval: 60,
		RegisteredAt:  time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	if err := ms.InsertContractAlert(nil, store.ContractAlert{
		ContractID: "CONTRACT_B",
		Severity:   "Critical",
		Message:    "queue backing up",
		Ledger:     100,
		TxHash:     "tx1",
		Timestamp:  time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := ms.InsertHealthCheck(nil, store.HealthCheck{
		ContractID: "CONTRACT_A",
		Status:     "Healthy",
		Ledger:     101,
		TxHash:     "tx2",
		Timestamp:  time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	return ms
}

func TestWatchdogStats(t *testing.T) {
	srv := newTestHandler(seededWatchdogStore(t), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/watchdog/stats", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body map[string]int64
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["total_monitored"] != 2 {
		t.Errorf("total_monitored: got %d", body["total_monitored"])
	}
	if body["healthy"] != 1 {
		t.Errorf("healthy: got %d", body["healthy"])
	}
	if body["degraded"] != 1 {
		t.Errorf("degraded: got %d", body["degraded"])
	}
	if body["total_alerts"] != 1 {
		t.Errorf("total_alerts: got %d", body["total_alerts"])
	}
	if body["critical_alerts"] != 1 {
		t.Errorf("critical_alerts: got %d", body["critical_alerts"])
	}
}

func TestListMonitoredContracts(t *testing.T) {
	srv := newTestHandler(seededWatchdogStore(t), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/watchdog/contracts", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body struct {
		Contracts []map[string]any `json:"contracts"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Contracts) != 2 {
		t.Fatalf("want 2 contracts, got %d", len(body.Contracts))
	}
}

func TestGetMonitoredContractNotFound(t *testing.T) {
	srv := newTestHandler(seededWatchdogStore(t), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/watchdog/contracts/UNKNOWN", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestListAlertsFiltersBySeverity(t *testing.T) {
	srv := newTestHandler(seededWatchdogStore(t), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/watchdog/alerts?severity=Info", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body struct {
		Alerts []map[string]any `json:"alerts"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Alerts) != 0 {
		t.Fatalf("expected zero Info alerts, got %d", len(body.Alerts))
	}
}
