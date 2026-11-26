package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMaintenanceWindow_CRUD(t *testing.T) {
	ms := seedRoleStore(t)
	srv := newTestHandler(ms, true, true)

	startTime := time.Now().UTC().Add(1 * time.Hour)
	endTime := startTime.Add(2 * time.Hour)

	// 1. Create maintenance window (POST /api/v1/maintenance-windows)
	createPayload := fmt.Sprintf(`{
		"name": "Scheduled DB Upgrade",
		"scope": "global",
		"start_time": %q,
		"end_time": %q,
		"timezone": "UTC",
		"recurrence": "none",
		"suppress_notifications": true
	}`, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/maintenance-windows", bytes.NewReader([]byte(createPayload)))
	req.Header.Set("Content-Type", "application/json")
	req = withTestIdentity(req, contributorUser)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201; body: %s", rr.Code, rr.Body.String())
	}

	var created struct {
		ID                    int64  `json:"id"`
		Name                  string `json:"name"`
		Scope                 string `json:"scope"`
		SuppressNotifications bool   `json:"suppress_notifications"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 {
		t.Fatalf("expected non-zero window ID")
	}
	if created.Name != "Scheduled DB Upgrade" {
		t.Errorf("name = %s, want 'Scheduled DB Upgrade'", created.Name)
	}
	if !created.SuppressNotifications {
		t.Errorf("expected SuppressNotifications = true")
	}

	// 2. Get maintenance window (GET /api/v1/maintenance-windows/{id})
	reqGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/maintenance-windows/%d", created.ID), nil)
	rrGet := httptest.NewRecorder()
	srv.ServeHTTP(rrGet, reqGet)

	if rrGet.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200; body: %s", rrGet.Code, rrGet.Body.String())
	}

	// 3. List maintenance windows (GET /api/v1/maintenance-windows)
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/maintenance-windows", nil)
	rrList := httptest.NewRecorder()
	srv.ServeHTTP(rrList, reqList)

	if rrList.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", rrList.Code)
	}
	var listResp struct {
		Windows []map[string]any `json:"maintenance_windows"`
	}
	if err := json.NewDecoder(rrList.Body).Decode(&listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Windows) != 1 {
		t.Errorf("len(windows) = %d, want 1", len(listResp.Windows))
	}

	// 4. Update maintenance window (PUT /api/v1/maintenance-windows/{id})
	updatePayload := fmt.Sprintf(`{
		"name": "Rescheduled DB Upgrade",
		"start_time": %q,
		"end_time": %q,
		"timezone": "UTC",
		"recurrence": "none"
	}`, startTime.Format(time.RFC3339), endTime.Add(1*time.Hour).Format(time.RFC3339))

	reqUpdate := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/maintenance-windows/%d", created.ID), bytes.NewReader([]byte(updatePayload)))
	reqUpdate.Header.Set("Content-Type", "application/json")
	reqUpdate = withTestIdentity(reqUpdate, contributorUser)
	rrUpdate := httptest.NewRecorder()
	srv.ServeHTTP(rrUpdate, reqUpdate)

	if rrUpdate.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200; body: %s", rrUpdate.Code, rrUpdate.Body.String())
	}

	// 5. Delete maintenance window (DELETE /api/v1/maintenance-windows/{id})
	reqDel := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/maintenance-windows/%d", created.ID), nil)
	reqDel = withTestIdentity(reqDel, contributorUser)
	rrDel := httptest.NewRecorder()
	srv.ServeHTTP(rrDel, reqDel)

	if rrDel.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", rrDel.Code)
	}

	// 6. Confirm 404 after deletion
	reqGetAfter := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/maintenance-windows/%d", created.ID), nil)
	rrGetAfter := httptest.NewRecorder()
	srv.ServeHTTP(rrGetAfter, reqGetAfter)

	if rrGetAfter.Code != http.StatusNotFound {
		t.Fatalf("get after delete status = %d, want 404", rrGetAfter.Code)
	}
}

func TestMaintenanceWindow_Validation(t *testing.T) {
	ms := seedRoleStore(t)
	srv := newTestHandler(ms, true, true)

	// End time before start time
	invalidPayload := `{
		"name": "Invalid Window",
		"scope": "global",
		"start_time": "2025-06-01T15:00:00Z",
		"end_time": "2025-06-01T14:00:00Z"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/maintenance-windows", bytes.NewReader([]byte(invalidPayload)))
	req.Header.Set("Content-Type", "application/json")
	req = withTestIdentity(req, contributorUser)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 for end_time before start_time", rr.Code)
	}

	// Missing target_id for contract scope
	contractScopePayload := `{
		"name": "Contract Window",
		"scope": "contract",
		"start_time": "2025-06-01T14:00:00Z",
		"end_time": "2025-06-01T15:00:00Z"
	}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/maintenance-windows", bytes.NewReader([]byte(contractScopePayload)))
	req2.Header.Set("Content-Type", "application/json")
	req2 = withTestIdentity(req2, contributorUser)
	rr2 := httptest.NewRecorder()
	srv.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 for missing target_id", rr2.Code)
	}
}

func TestMaintenanceWindow_RoleRequired(t *testing.T) {
	ms := seedRoleStore(t)
	srv := newTestHandler(ms, true, true)

	payload := `{
		"name": "Test Window",
		"scope": "global",
		"start_time": "2025-06-01T14:00:00Z",
		"end_time": "2025-06-01T15:00:00Z"
	}`

	// Unauthenticated returns 401
	reqAnon := httptest.NewRequest(http.MethodPost, "/api/v1/maintenance-windows", bytes.NewReader([]byte(payload)))
	reqAnon.Header.Set("Content-Type", "application/json")
	rrAnon := httptest.NewRecorder()
	srv.ServeHTTP(rrAnon, reqAnon)
	if rrAnon.Code != http.StatusUnauthorized {
		t.Fatalf("anon status = %d, want 401", rrAnon.Code)
	}

	// Viewer returns 403
	reqViewer := httptest.NewRequest(http.MethodPost, "/api/v1/maintenance-windows", bytes.NewReader([]byte(payload)))
	reqViewer.Header.Set("Content-Type", "application/json")
	reqViewer = withTestIdentity(reqViewer, viewerUser)
	rrViewer := httptest.NewRecorder()
	srv.ServeHTTP(rrViewer, reqViewer)
	if rrViewer.Code != http.StatusForbidden {
		t.Fatalf("viewer status = %d, want 403", rrViewer.Code)
	}
}
