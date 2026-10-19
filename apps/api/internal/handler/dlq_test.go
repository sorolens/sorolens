package handler_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func TestListFailedEvents_Empty(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	w := doRequest(srv, http.MethodGet, "/api/v1/dlq", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var resp struct {
		Items []any `json:"items"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 0 {
		t.Fatalf("items = %d, want 0", len(resp.Items))
	}
}

func TestListFailedEvents_ReturnsSeeded(t *testing.T) {
	ms := store.NewMockStore()
	ms.SeedFailedEvent("evt-bad-1", "CABC", "decode boom", store.Event{
		ID: "evt-bad-1", ContractID: "CABC", Network: "testnet", Type: "contract",
	})
	srv := newTestHandler(ms, true, true)

	w := doRequest(srv, http.MethodGet, "/api/v1/dlq", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var resp struct {
		Items []struct {
			EventID      string `json:"event_id"`
			ErrorMessage string `json:"error_message"`
		} `json:"items"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(resp.Items))
	}
	if resp.Items[0].EventID != "evt-bad-1" {
		t.Fatalf("event_id = %q", resp.Items[0].EventID)
	}
	if resp.Items[0].ErrorMessage != "decode boom" {
		t.Fatalf("error_message = %q", resp.Items[0].ErrorMessage)
	}
}

func TestRequeueFailedEvent_InsertsAndClears(t *testing.T) {
	ms := seedRoleStore(t)
	fe := ms.SeedFailedEvent("evt-bad-2", "CXYZ", "insert fail", store.Event{
		ID: "evt-bad-2", ContractID: "CXYZ", Network: "testnet",
		Type: "contract", TxHash: "abc", ValueXDR: "AAA=", TopicXDR: []string{},
	})
	srv := newTestHandler(ms, true, true)

	path := "/api/v1/dlq/" + strconv.FormatInt(fe.ID, 10) + "/requeue"
	w := doRequestAsUser(srv, http.MethodPost, path, "", contributorUser, "{}")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var resp struct {
		Status  string `json:"status"`
		EventID string `json:"event_id"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != "requeued" {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.EventID != "evt-bad-2" {
		t.Fatalf("event_id = %q", resp.EventID)
	}

	if _, err := ms.GetFailedEvent(nil, fe.ID); err != store.ErrNotFound {
		t.Fatalf("expected DLQ cleared, got err=%v", err)
	}

	events, _, err := ms.ListEvents(nil, "CXYZ", "", 50, store.EventFilters{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if e.ID == "evt-bad-2" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected event re-inserted into events store")
	}
}

func TestRequeueFailedEvent_NotFound(t *testing.T) {
	srv := newTestHandler(seedRoleStore(t), true, true)
	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/dlq/999/requeue", "", contributorUser, "{}")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%s)", w.Code, w.Body.String())
	}
}
