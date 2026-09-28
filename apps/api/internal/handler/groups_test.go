package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const groupUser = "user_1"

func groupRequest(t *testing.T, srv http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Buffer
	if body == "" {
		reader = bytes.NewBuffer(nil)
	} else {
		reader = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", groupUser)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func createGroup(t *testing.T, srv http.Handler, name string) string {
	t.Helper()
	w := groupRequest(t, srv, http.MethodPost, "/api/v1/groups", `{"name":"`+name+`"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create group: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID == "" {
		t.Fatal("create group: empty id")
	}
	return resp.ID
}

func TestCreateGroupRequiresUserID(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/groups", bytes.NewBufferString(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 without X-User-ID, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestCreateGroupRejectsBlankName(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	w := groupRequest(t, srv, http.MethodPost, "/api/v1/groups", `{"name":"   "}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 for blank name, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestListGroupsIncludesAggregateStats(t *testing.T) {
	ctx := context.Background()
	ms := store.NewMockStore()
	if err := ms.UpsertContract(ctx, store.Contract{ID: "CONTRACT_A", Network: "testnet", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if err := ms.BatchInsertEvents(ctx, []store.Event{{ID: "e1", ContractID: "CONTRACT_A"}, {ID: "e2", ContractID: "CONTRACT_A"}}); err != nil {
		t.Fatal(err)
	}
	if err := ms.BatchInsertInvocations(ctx, []store.Invocation{{TxHash: "tx1", ContractID: "CONTRACT_A"}}); err != nil {
		t.Fatal(err)
	}
	if err := ms.UpsertStorageEntries(ctx, []store.StorageEntry{{ContractID: "CONTRACT_A", KeyXDR: "k1"}}); err != nil {
		t.Fatal(err)
	}
	if err := ms.UpsertContractHealthScore(ctx, store.ContractHealthScore{ContractID: "CONTRACT_A", Score: 88}); err != nil {
		t.Fatal(err)
	}
	srv := newTestHandler(ms, true, true)

	groupID := createGroup(t, srv, "Protocol")
	if w := groupRequest(t, srv, http.MethodPost, "/api/v1/groups/"+groupID+"/contracts", `{"contract_id":"CONTRACT_A"}`); w.Code != http.StatusCreated {
		t.Fatalf("add membership: want 201, got %d (%s)", w.Code, w.Body.String())
	}

	w := groupRequest(t, srv, http.MethodGet, "/api/v1/groups", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list groups: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var resp struct {
		Groups []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Stats struct {
				ContractCount      int64   `json:"contract_count"`
				EventCount         int64   `json:"event_count"`
				InvocationCount    int64   `json:"invocation_count"`
				StorageEntryCount  int64   `json:"storage_entry_count"`
				AverageHealthScore float64 `json:"average_health_score"`
			} `json:"stats"`
		} `json:"groups"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(resp.Groups))
	}
	got := resp.Groups[0]
	if got.ID != groupID || got.Name != "Protocol" {
		t.Errorf("group = %+v, want id=%s name=Protocol", got, groupID)
	}
	if got.Stats.ContractCount != 1 || got.Stats.EventCount != 2 || got.Stats.InvocationCount != 1 || got.Stats.StorageEntryCount != 1 {
		t.Errorf("stats = %+v, want 1/2/1/1", got.Stats)
	}
	if got.Stats.AverageHealthScore != 88 {
		t.Errorf("average_health_score = %v, want 88", got.Stats.AverageHealthScore)
	}
}

func TestGroupCRUDAndMembership(t *testing.T) {
	ctx := context.Background()
	ms := store.NewMockStore()
	if err := ms.UpsertContract(ctx, store.Contract{ID: "CONTRACT_A", Network: "testnet", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	srv := newTestHandler(ms, true, true)

	groupID := createGroup(t, srv, "Portfolio")

	// Membership via the documented body endpoint.
	if w := groupRequest(t, srv, http.MethodPost, "/api/v1/groups/"+groupID+"/contracts", `{"contract_id":"CONTRACT_A"}`); w.Code != http.StatusCreated {
		t.Fatalf("add membership: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	// Adding the same contract again is idempotent.
	if w := groupRequest(t, srv, http.MethodPost, "/api/v1/groups/"+groupID+"/contracts", `{"contract_id":"CONTRACT_A"}`); w.Code != http.StatusCreated {
		t.Fatalf("re-add membership: want 201, got %d", w.Code)
	}

	// Detail includes the member with its per-contract signals.
	w := groupRequest(t, srv, http.MethodGet, "/api/v1/groups/"+groupID, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get group: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var detail struct {
		Contracts []struct {
			ContractID string `json:"contract_id"`
		} `json:"contracts"`
	}
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Contracts) != 1 || detail.Contracts[0].ContractID != "CONTRACT_A" {
		t.Fatalf("contracts = %+v, want single CONTRACT_A", detail.Contracts)
	}

	// Rename.
	if w := groupRequest(t, srv, http.MethodPatch, "/api/v1/groups/"+groupID, `{"name":"Renamed"}`); w.Code != http.StatusOK {
		t.Fatalf("patch group: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if w := groupRequest(t, srv, http.MethodPatch, "/api/v1/groups/"+groupID, `{"name":""}`); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("patch blank name: want 422, got %d", w.Code)
	}

	// Remove membership via the path-segment form the UI uses.
	if w := groupRequest(t, srv, http.MethodDelete, "/api/v1/groups/"+groupID+"/contracts/CONTRACT_A", ""); w.Code != http.StatusOK {
		t.Fatalf("delete membership: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	// And via the documented body form.
	if w := groupRequest(t, srv, http.MethodPost, "/api/v1/groups/"+groupID+"/contracts", `{"contract_id":"CONTRACT_A"}`); w.Code != http.StatusCreated {
		t.Fatalf("re-add after remove: want 201, got %d", w.Code)
	}
	if w := groupRequest(t, srv, http.MethodDelete, "/api/v1/groups/"+groupID+"/contracts", `{"contract_id":"CONTRACT_A"}`); w.Code != http.StatusOK {
		t.Fatalf("delete membership by body: want 200, got %d (%s)", w.Code, w.Body.String())
	}

	// Delete the group.
	if w := groupRequest(t, srv, http.MethodDelete, "/api/v1/groups/"+groupID, ""); w.Code != http.StatusOK {
		t.Fatalf("delete group: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if w := groupRequest(t, srv, http.MethodGet, "/api/v1/groups/"+groupID, ""); w.Code != http.StatusNotFound {
		t.Fatalf("get deleted group: want 404, got %d", w.Code)
	}
	if w := groupRequest(t, srv, http.MethodGet, "/api/v1/groups/"+groupID+"/stats", ""); w.Code != http.StatusNotFound {
		t.Fatalf("stats of deleted group: want 404, got %d", w.Code)
	}
}

func TestGroupStatsEndpoint(t *testing.T) {
	ctx := context.Background()
	ms := store.NewMockStore()
	_ = ms.UpsertContract(ctx, store.Contract{ID: "CONTRACT_A", Network: "testnet", Status: "active"})
	_ = ms.UpsertContract(ctx, store.Contract{ID: "CONTRACT_B", Network: "testnet", Status: "active"})
	_ = ms.BatchInsertEvents(ctx, []store.Event{{ID: "e1", ContractID: "CONTRACT_A"}, {ID: "e2", ContractID: "CONTRACT_B"}, {ID: "e3", ContractID: "CONTRACT_B"}})
	_ = ms.BatchInsertInvocations(ctx, []store.Invocation{{TxHash: "tx1", ContractID: "CONTRACT_B"}})
	_ = ms.UpsertStorageEntries(ctx, []store.StorageEntry{{ContractID: "CONTRACT_A", KeyXDR: "k1"}})
	_ = ms.UpsertContractHealthScore(ctx, store.ContractHealthScore{ContractID: "CONTRACT_A", Score: 90})
	_ = ms.UpsertContractHealthScore(ctx, store.ContractHealthScore{ContractID: "CONTRACT_B", Score: 50})
	srv := newTestHandler(ms, true, true)

	groupID := createGroup(t, srv, "Mixed")
	for _, id := range []string{"CONTRACT_A", "CONTRACT_B"} {
		if w := groupRequest(t, srv, http.MethodPost, "/api/v1/groups/"+groupID+"/contracts", `{"contract_id":"`+id+`"}`); w.Code != http.StatusCreated {
			t.Fatalf("add %s: got %d", id, w.Code)
		}
	}

	w := groupRequest(t, srv, http.MethodGet, "/api/v1/groups/"+groupID+"/stats", "")
	if w.Code != http.StatusOK {
		t.Fatalf("stats: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var stats struct {
		GroupID            string  `json:"group_id"`
		ContractCount      int64   `json:"contract_count"`
		EventCount         int64   `json:"event_count"`
		InvocationCount    int64   `json:"invocation_count"`
		StorageEntryCount  int64   `json:"storage_entry_count"`
		AverageHealthScore float64 `json:"average_health_score"`
	}
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats.GroupID != groupID || stats.ContractCount != 2 || stats.EventCount != 3 ||
		stats.InvocationCount != 1 || stats.StorageEntryCount != 1 || stats.AverageHealthScore != 70 {
		t.Errorf("stats = %+v, want group %s 2/3/1/1/70", stats, groupID)
	}
}

func TestAddGroupContractUnknownContract(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	groupID := createGroup(t, srv, "Empty")
	w := groupRequest(t, srv, http.MethodPost, "/api/v1/groups/"+groupID+"/contracts", `{"contract_id":"CNOPE"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for unknown contract, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGroupRoutesScopedToOwner(t *testing.T) {
	ms := store.NewMockStore()
	srv := newTestHandler(ms, true, true)
	groupID := createGroup(t, srv, "Mine")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups/"+groupID, nil)
	req.Header.Set("X-User-ID", "someone_else")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("other user's group: want 404, got %d (%s)", w.Code, w.Body.String())
	}

	// A malformed group ID is also a clean 404.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/groups/not-a-uuid", nil)
	req.Header.Set("X-User-ID", groupUser)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("malformed group id: want 404, got %d (%s)", w.Code, w.Body.String())
	}
}
