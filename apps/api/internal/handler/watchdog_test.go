package handler_test

import (
	"encoding/json"
	"fmt"
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

// seededAlertsFeedStore returns a mock store whose alerts feed holds five
// alerts for one contract, one minute apart, so cursor pages are
// deterministic (issue #150).
func seededAlertsFeedStore(t *testing.T) *store.MockStore {
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
	base := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		if err := ms.InsertContractAlert(nil, store.ContractAlert{
			ContractID: "CONTRACT_A",
			Severity:   "Warning",
			Message:    fmt.Sprintf("alert %d", i),
			Ledger:     int64(200 + i),
			TxHash:     fmt.Sprintf("tx-%d", i),
			Timestamp:  base.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}
	return ms
}

func TestListAlertsFirstPageAndContinuation(t *testing.T) {
	srv := newTestHandler(seededAlertsFeedStore(t), true, true)

	// First page: two newest alerts plus a next_cursor.
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/watchdog/alerts?limit=2", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("first page: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var first struct {
		Alerts []struct {
			TxHash string `json:"tx_hash"`
		} `json:"alerts"`
		NextCursor string `json:"next_cursor"`
	}
	if err := json.NewDecoder(w.Body).Decode(&first); err != nil {
		t.Fatal(err)
	}
	if len(first.Alerts) != 2 {
		t.Fatalf("first page: want 2 alerts, got %d", len(first.Alerts))
	}
	if first.Alerts[0].TxHash != "tx-4" || first.Alerts[1].TxHash != "tx-3" {
		t.Fatalf("first page should hold the two newest alerts, got %v", first.Alerts)
	}
	if first.NextCursor == "" {
		t.Fatalf("first page must carry a next_cursor")
	}

	// Continuation: the next two alerts, no overlap with page one.
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, httptest.NewRequest(http.MethodGet,
		"/api/v1/watchdog/alerts?limit=2&cursor="+first.NextCursor, nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("continuation: want 200, got %d (%s)", w2.Code, w2.Body.String())
	}
	var second struct {
		Alerts []struct {
			TxHash string `json:"tx_hash"`
		} `json:"alerts"`
		NextCursor string `json:"next_cursor"`
	}
	if err := json.NewDecoder(w2.Body).Decode(&second); err != nil {
		t.Fatal(err)
	}
	if len(second.Alerts) != 2 {
		t.Fatalf("continuation: want 2 alerts, got %d", len(second.Alerts))
	}
	if second.Alerts[0].TxHash != "tx-2" || second.Alerts[1].TxHash != "tx-1" {
		t.Fatalf("continuation should hold the next alerts, got %v", second.Alerts)
	}
	if second.NextCursor == "" {
		t.Fatalf("continuation should still have a next_cursor (one alert left)")
	}

	// Walk to exhaustion: the union of all pages must cover the feed once.
	seen := map[string]bool{first.Alerts[0].TxHash: true, first.Alerts[1].TxHash: true,
		second.Alerts[0].TxHash: true, second.Alerts[1].TxHash: true}
	cursor := second.NextCursor
	for cursor != "" {
		wN := httptest.NewRecorder()
		srv.ServeHTTP(wN, httptest.NewRequest(http.MethodGet,
			"/api/v1/watchdog/alerts?limit=2&cursor="+cursor, nil))
		if wN.Code != http.StatusOK {
			t.Fatalf("walk: want 200, got %d", wN.Code)
		}
		var page struct {
			Alerts []struct {
				TxHash string `json:"tx_hash"`
			} `json:"alerts"`
			NextCursor string `json:"next_cursor"`
		}
		if err := json.NewDecoder(wN.Body).Decode(&page); err != nil {
			t.Fatal(err)
		}
		for _, a := range page.Alerts {
			if seen[a.TxHash] {
				t.Fatalf("alert %s returned twice across pages", a.TxHash)
			}
			seen[a.TxHash] = true
		}
		cursor = page.NextCursor
	}
	if len(seen) != 5 {
		t.Fatalf("walk should cover all 5 alerts once, saw %d", len(seen))
	}
}

func TestListAlertsInvalidCursorRejected(t *testing.T) {
	srv := newTestHandler(seededAlertsFeedStore(t), true, true)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/watchdog/alerts?cursor=not-a-cursor", nil))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 for an undecodable cursor, got %d (%s)", w.Code, w.Body.String())
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

func TestGetContractUptimeDefault(t *testing.T) {
	ms := seededWatchdogStore(t)
	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/watchdog/contracts/CONTRACT_A/uptime", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["contract_id"] != "CONTRACT_A" {
		t.Errorf("contract_id: got %v", body["contract_id"])
	}
	if body["window"] != "24h" {
		t.Errorf("window: got %v, want 24h", body["window"])
	}
	// CONTRACT_A has 1 health check with status Healthy, so uptime should be 100
	if body["uptime_pct"].(float64) != 100.0 {
		t.Errorf("uptime_pct: got %v, want 100.0", body["uptime_pct"])
	}
}

func TestGetContractUptimeWindows(t *testing.T) {
	ms := seededWatchdogStore(t)
	srv := newTestHandler(ms, true, true)

	for _, window := range []string{"24h", "7d", "30d"} {
		t.Run(window, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/watchdog/contracts/CONTRACT_A/uptime?window="+window, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
			}
			var body map[string]any
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["window"] != window {
				t.Errorf("window: got %v, want %s", body["window"], window)
			}
			if _, ok := body["uptime_pct"]; !ok {
				t.Error("missing uptime_pct field")
			}
		})
	}
}

func TestGetContractUptimeInvalidWindow(t *testing.T) {
	srv := newTestHandler(seededWatchdogStore(t), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/watchdog/contracts/CONTRACT_A/uptime?window=99d", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGetContractUptimeNoChecks(t *testing.T) {
	ms := seededWatchdogStore(t)
	srv := newTestHandler(ms, true, true)
	// CONTRACT_B has no health checks seeded, so uptime should be 0
	req := httptest.NewRequest(http.MethodGet, "/api/v1/watchdog/contracts/CONTRACT_B/uptime?window=24h", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["uptime_pct"].(float64) != 0.0 {
		t.Errorf("uptime_pct: got %v, want 0.0 for contract with no checks", body["uptime_pct"])
	}
}
