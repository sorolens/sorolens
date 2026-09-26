package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ---- fixtures --------------------------------------------------------------

var (
	alertT0 = time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	alertT1 = time.Date(2025, 6, 1, 12, 5, 0, 0, time.UTC) // +5 min
	alertT2 = time.Date(2025, 6, 1, 12, 10, 0, 0, time.UTC) // +10 min
)

func seededAlertGroupStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()

	// Register a monitored contract so the store has the FK target.
	if err := ms.UpsertMonitoredContract(nil, store.MonitoredContract{
		ContractID:    "CONTRACT_A",
		Network:       "testnet",
		Name:          "app_v1",
		Owner:         "GOWNER",
		Status:        "Healthy",
		CheckInterval: 300,
		RegisteredAt:  alertT0,
	}); err != nil {
		t.Fatal(err)
	}

	// Two groups for CONTRACT_A.
	if _, err := ms.UpsertAlertGroup(nil, store.AlertGroup{
		GroupKey:         "CONTRACT_A|Critical|high_error_rate",
		ContractID:       "CONTRACT_A",
		Severity:         "Critical",
		Rule:             "high_error_rate",
		Count:            3,
		DedupeWindowSecs: 300,
		FirstSeen:        alertT0,
		LastSeen:         alertT1,
		LastMessage:      "error rate exceeded threshold",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.UpsertAlertGroup(nil, store.AlertGroup{
		GroupKey:         "CONTRACT_A|Warning|",
		ContractID:       "CONTRACT_A",
		Severity:         "Warning",
		Rule:             "",
		Count:            1,
		DedupeWindowSecs: 300,
		FirstSeen:        alertT2,
		LastSeen:         alertT2,
		LastMessage:      "latency spike",
	}); err != nil {
		t.Fatal(err)
	}

	// Raw alerts for ?flat=true tests.
	for i, sev := range []string{"Critical", "Warning", "Info"} {
		if err := ms.InsertContractAlert(nil, store.ContractAlert{
			ContractID: "CONTRACT_A",
			Severity:   sev,
			Message:    "raw alert " + sev,
			Ledger:     int64(100 + i),
			TxHash:     "txhash" + sev,
			Timestamp:  alertT0.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}

	return ms
}

// ---- grouped view ----------------------------------------------------------

func TestListAlertsGroupedReturnsGroups(t *testing.T) {
	srv := newTestHandler(seededAlertGroupStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	var body struct {
		Groups     []map[string]any `json:"groups"`
		NextCursor string           `json:"next_cursor"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Groups) != 2 {
		t.Errorf("len(groups) = %d, want 2", len(body.Groups))
	}
}

func TestListAlertsGroupedFiltersBySeverity(t *testing.T) {
	srv := newTestHandler(seededAlertGroupStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts?severity=Critical", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body struct {
		Groups []map[string]any `json:"groups"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Groups) != 1 {
		t.Errorf("len(groups) = %d, want 1 (Critical only)", len(body.Groups))
	}
	if body.Groups[0]["severity"] != "Critical" {
		t.Errorf("severity = %v, want Critical", body.Groups[0]["severity"])
	}
}

func TestListAlertsGroupedFiltersByContract(t *testing.T) {
	srv := newTestHandler(seededAlertGroupStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts?contract_id=CONTRACT_A", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body struct {
		Groups []map[string]any `json:"groups"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Groups) != 2 {
		t.Errorf("len(groups) = %d, want 2", len(body.Groups))
	}
}

func TestListAlertsGroupedResponseShape(t *testing.T) {
	srv := newTestHandler(seededAlertGroupStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts?severity=Critical", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	var body struct {
		Groups []map[string]any `json:"groups"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	g := body.Groups[0]
	for _, field := range []string{"id", "group_key", "contract_id", "severity", "rule",
		"count", "dedupe_window_secs", "first_seen", "last_seen", "last_message", "backfill_eligible"} {
		if _, ok := g[field]; !ok {
			t.Errorf("response missing field %q", field)
		}
	}
}

func TestListAlertGroupsInvalidCursorRejected(t *testing.T) {
	srv := newTestHandler(seededAlertGroupStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts?cursor=notvalidbase64!!!", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rr.Code)
	}
}

// ---- flat view -------------------------------------------------------------

func TestListAlertsFlatReturnsRawAlerts(t *testing.T) {
	srv := newTestHandler(seededAlertGroupStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts?flat=true", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Alerts []map[string]any `json:"alerts"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Alerts) != 3 {
		t.Errorf("len(alerts) = %d, want 3 raw alerts", len(body.Alerts))
	}
}

func TestListAlertsFlatFiltersBySeverity(t *testing.T) {
	srv := newTestHandler(seededAlertGroupStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts?flat=true&severity=Critical", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body struct {
		Alerts []map[string]any `json:"alerts"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Alerts) != 1 {
		t.Errorf("len(alerts) = %d, want 1 (Critical only)", len(body.Alerts))
	}
}
