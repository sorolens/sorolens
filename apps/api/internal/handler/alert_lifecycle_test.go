package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func seedAlertLifecycleStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := seedRoleStore(t)

	// Contract
	_ = ms.UpsertMonitoredContract(nil, store.MonitoredContract{
		ContractID:    "CONTRACT_LIFECYCLE",
		Network:       "testnet",
		Name:          "lifecycle_app",
		Owner:         "GOWNER",
		Status:        "Healthy",
		CheckInterval: 300,
		RegisteredAt:  alertT0,
	})

	// Insert alert 1 (open)
	_ = ms.InsertContractAlert(nil, store.ContractAlert{
		ContractID: "CONTRACT_LIFECYCLE",
		Severity:   "Critical",
		Message:    "alert 1 critical",
		Ledger:     101,
		TxHash:     "tx1",
		Timestamp:  alertT0,
		State:      store.AlertStateOpen,
	})

	// Insert alert 2 (open)
	_ = ms.InsertContractAlert(nil, store.ContractAlert{
		ContractID: "CONTRACT_LIFECYCLE",
		Severity:   "Warning",
		Message:    "alert 2 warning",
		Ledger:     102,
		TxHash:     "tx2",
		Timestamp:  alertT0.Add(1 * time.Minute),
		State:      store.AlertStateOpen,
	})

	return ms
}

func TestAlertLifecycle_Acknowledge(t *testing.T) {
	ms := seedAlertLifecycleStore(t)
	srv := newTestHandler(ms, true, true)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/1/ack", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	req = withTestIdentity(req, contributorUser)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	var alert map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&alert); err != nil {
		t.Fatal(err)
	}
	if alert["state"] != "acknowledged" {
		t.Errorf("state = %v, want acknowledged", alert["state"])
	}
	if alert["acknowledged_by"] != contributorUser {
		t.Errorf("acknowledged_by = %v, want %s", alert["acknowledged_by"], contributorUser)
	}
}

func TestAlertLifecycle_Snooze(t *testing.T) {
	ms := seedAlertLifecycleStore(t)
	srv := newTestHandler(ms, true, true)

	body := []byte(`{"duration":"2h"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/1/snooze", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withTestIdentity(req, contributorUser)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	var alert map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&alert); err != nil {
		t.Fatal(err)
	}
	if alert["state"] != "snoozed" {
		t.Errorf("state = %v, want snoozed", alert["state"])
	}
	if alert["snoozed_until"] == nil || alert["snoozed_until"] == "" {
		t.Errorf("snoozed_until must not be empty")
	}
}

func TestAlertLifecycle_Resolve(t *testing.T) {
	ms := seedAlertLifecycleStore(t)
	srv := newTestHandler(ms, true, true)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/1/resolve", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	req = withTestIdentity(req, contributorUser)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	var alert map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&alert); err != nil {
		t.Fatal(err)
	}
	if alert["state"] != "resolved" {
		t.Errorf("state = %v, want resolved", alert["state"])
	}
	if alert["resolved_by"] != contributorUser {
		t.Errorf("resolved_by = %v, want %s", alert["resolved_by"], contributorUser)
	}
}

func TestAlertLifecycle_InvalidTransitionsReturn409(t *testing.T) {
	ms := seedAlertLifecycleStore(t)
	srv := newTestHandler(ms, true, true)

	// First resolve alert 1
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/1/resolve", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	req = withTestIdentity(req, contributorUser)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200", rr.Code)
	}

	// Attempting to ack a resolved alert must return 409 Conflict
	reqAck := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/1/ack", bytes.NewReader([]byte("{}")))
	reqAck.Header.Set("Content-Type", "application/json")
	reqAck = withTestIdentity(reqAck, contributorUser)
	rrAck := httptest.NewRecorder()
	srv.ServeHTTP(rrAck, reqAck)
	if rrAck.Code != http.StatusConflict {
		t.Fatalf("ack resolved alert status = %d, want 409 Conflict; body: %s", rrAck.Code, rrAck.Body.String())
	}

	// Attempting to snooze a resolved alert must return 409 Conflict
	body := []byte(`{"duration":"1h"}`)
	reqSnz := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/1/snooze", bytes.NewReader(body))
	reqSnz.Header.Set("Content-Type", "application/json")
	reqSnz = withTestIdentity(reqSnz, contributorUser)
	rrSnz := httptest.NewRecorder()
	srv.ServeHTTP(rrSnz, reqSnz)
	if rrSnz.Code != http.StatusConflict {
		t.Fatalf("snooze resolved alert status = %d, want 409 Conflict; body: %s", rrSnz.Code, rrSnz.Body.String())
	}
}

func TestAlertLifecycle_AlertCounts(t *testing.T) {
	ms := seedAlertLifecycleStore(t)
	srv := newTestHandler(ms, true, true)

	// Initially 2 open alerts
	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/counts", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Counts map[string]int64 `json:"counts"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Counts["open"] != 2 {
		t.Errorf("open count = %d, want 2", resp.Counts["open"])
	}
}

func TestAlertLifecycle_RoleRequired(t *testing.T) {
	ms := seedAlertLifecycleStore(t)
	srv := newTestHandler(ms, true, true)

	// Unauthenticated request returns 401 Unauthorized
	reqAnon := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/1/ack", bytes.NewReader([]byte("{}")))
	reqAnon.Header.Set("Content-Type", "application/json")
	rrAnon := httptest.NewRecorder()
	srv.ServeHTTP(rrAnon, reqAnon)

	if rrAnon.Code != http.StatusUnauthorized {
		t.Fatalf("anon status = %d, want 401 Unauthorized", rrAnon.Code)
	}

	// Viewer request returns 403 Forbidden
	reqViewer := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/1/ack", bytes.NewReader([]byte("{}")))
	reqViewer.Header.Set("Content-Type", "application/json")
	reqViewer = withTestIdentity(reqViewer, viewerUser)
	rrViewer := httptest.NewRecorder()
	srv.ServeHTTP(rrViewer, reqViewer)

	if rrViewer.Code != http.StatusForbidden {
		t.Fatalf("viewer status = %d, want 403 Forbidden", rrViewer.Code)
	}
}
