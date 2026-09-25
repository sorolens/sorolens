package handler_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// waitForAudit polls the mock store until it holds n audit rows. The audit
// middleware writes in the background, so rows land shortly after the
// response.
func waitForAudit(t *testing.T, ms *store.MockStore, n int) []store.AuditEvent {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		events := ms.AuditEvents()
		if len(events) >= n {
			return events
		}
		if time.Now().After(deadline) {
			t.Fatalf("want %d audit rows, got %d", n, len(events))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestAuditRecordsMutatingRequest(t *testing.T) {
	ms := seedRBACUsers(t)
	srv := newTestHandler(ms, true, true)

	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts", "", contributorUser, validContractBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", w.Code, w.Body.String())
	}

	e := waitForAudit(t, ms, 1)[0]
	if e.Actor != contributorUser {
		t.Errorf("actor: want %q, got %q", contributorUser, e.Actor)
	}
	if e.Action != "POST /api/v1/contracts" {
		t.Errorf("action: got %q", e.Action)
	}
	if e.ResourceType != "contracts" {
		t.Errorf("resource_type: got %q", e.ResourceType)
	}
	if e.Status != http.StatusCreated {
		t.Errorf("status: want 201, got %d", e.Status)
	}
	if e.RequestBodyHash != sha256Hex(validContractBody) {
		t.Errorf("request_body_hash: want sha256 of body, got %q", e.RequestBodyHash)
	}
	if e.IP == "" || e.At.IsZero() {
		t.Errorf("ip/at not populated: %+v", e)
	}
}

func TestAuditRecordsFailedRequest(t *testing.T) {
	ms := seedRBACUsers(t)
	srv := newTestHandler(ms, true, true)

	// Rejected by the role middleware before reaching the handler.
	w := doRequestAsUser(srv, http.MethodDelete, "/api/v1/watched-accounts/"+watchedAccountA, "", viewerUser, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Code)
	}

	e := waitForAudit(t, ms, 1)[0]
	if e.Status != http.StatusForbidden {
		t.Errorf("status: want 403, got %d", e.Status)
	}
	if e.Action != "DELETE /api/v1/watched-accounts/{id}" {
		t.Errorf("action: got %q", e.Action)
	}
	if e.ResourceType != "watched-accounts" || e.ResourceID != watchedAccountA {
		t.Errorf("resource: got %q/%q", e.ResourceType, e.ResourceID)
	}
}

func TestAuditRecordsContentTypeRejection(t *testing.T) {
	ms := seedRBACUsers(t)
	srv := newTestHandler(ms, true, true)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts", strings.NewReader(validContractBody))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("X-User-ID", contributorUser)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("want 415, got %d", w.Code)
	}

	e := waitForAudit(t, ms, 1)[0]
	if e.Status != http.StatusUnsupportedMediaType || e.Action != "POST /api/v1/contracts" {
		t.Fatalf("unexpected audit row: %+v", e)
	}
}

func TestAuditSkipsReads(t *testing.T) {
	ms := seedRBACUsers(t)
	srv := newTestHandler(ms, true, true)

	doRequestAsUser(srv, http.MethodGet, "/api/v1/contracts", "", "", "")
	// Follow with a write so we know the middleware has had a chance to run.
	doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts", "", contributorUser, validContractBody)

	waitForAudit(t, ms, 1)
	time.Sleep(20 * time.Millisecond)
	events := ms.AuditEvents()
	if len(events) != 1 || events[0].Action != "POST /api/v1/contracts" {
		t.Fatalf("want only the POST audited, got %+v", events)
	}
}

func TestAuditWriteErrorDoesNotFailRequest(t *testing.T) {
	ms := seedRBACUsers(t)
	ms.InsertAuditErr = errors.New("db down")
	srv := newTestHandler(ms, true, true)

	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts", "", contributorUser, validContractBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201 despite audit failure, got %d (%s)", w.Code, w.Body.String())
	}
}

type auditListBody struct {
	Events []struct {
		ID     int64  `json:"id"`
		Action string `json:"action"`
		Status int    `json:"status"`
	} `json:"events"`
	NextCursor string `json:"next_cursor"`
}

func TestListAuditEventsPagination(t *testing.T) {
	ms := seedRBACUsers(t)
	srv := newTestHandler(ms, true, true)

	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		if err := ms.InsertAuditEvent(context.Background(), store.AuditEvent{
			Action: "POST /api/v1/contracts",
			Status: 201,
			At:     base.Add(time.Duration(i) * time.Hour),
		}); err != nil {
			t.Fatal(err)
		}
	}

	var ids []int64
	path := "/api/v1/admin/audit?limit=2"
	for page := 0; page < 5; page++ {
		w := doRequestAsUser(srv, http.MethodGet, path, "", adminUser, "")
		if w.Code != http.StatusOK {
			t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
		}
		var body auditListBody
		if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		for _, e := range body.Events {
			ids = append(ids, e.ID)
		}
		if body.NextCursor == "" {
			break
		}
		path = "/api/v1/admin/audit?limit=2&cursor=" + body.NextCursor
	}
	want := []int64{5, 4, 3, 2, 1}
	if len(ids) != len(want) {
		t.Fatalf("want ids %v, got %v", want, ids)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("want ids %v (newest first), got %v", want, ids)
		}
	}

	// since is inclusive and filters older rows out.
	w := doRequestAsUser(srv, http.MethodGet,
		"/api/v1/admin/audit?since="+base.Add(3*time.Hour).Format(time.RFC3339), "", adminUser, "")
	var body auditListBody
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Events) != 2 || body.Events[0].ID != 5 || body.Events[1].ID != 4 {
		t.Fatalf("since filter: got %+v", body.Events)
	}
}

func TestListAuditEventsValidationAndRoles(t *testing.T) {
	srv := newTestHandler(seedRBACUsers(t), true, true)

	cases := []struct {
		name   string
		path   string
		userID string
		want   int
	}{
		{"bad since", "/api/v1/admin/audit?since=yesterday", adminUser, http.StatusUnprocessableEntity},
		{"bad cursor", "/api/v1/admin/audit?cursor=bm90LWEtbnVtYmVy", adminUser, http.StatusUnprocessableEntity},
		{"contributor denied", "/api/v1/admin/audit", contributorUser, http.StatusForbidden},
		{"viewer denied", "/api/v1/admin/audit", viewerUser, http.StatusForbidden},
		{"anonymous denied", "/api/v1/admin/audit", "", http.StatusUnauthorized},
		{"admin ok", "/api/v1/admin/audit", adminUser, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doRequestAsUser(srv, http.MethodGet, tc.path, "", tc.userID, "")
			if w.Code != tc.want {
				t.Fatalf("want %d, got %d (%s)", tc.want, w.Code, w.Body.String())
			}
		})
	}
}
