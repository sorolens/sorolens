package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// 56-character strkeys, matching the API's contract ID validation.
var (
	deleteContractID      = "C" + strings.Repeat("D", 55)
	deleteContractOtherID = "C" + strings.Repeat("E", 55)
	deleteUnknownID       = "C" + strings.Repeat("F", 55)
)

func seedDeleteStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := seedRBACUsers(t)
	ctx := context.Background()
	for _, id := range []string{deleteContractID, deleteContractOtherID} {
		if err := ms.UpsertContract(ctx, store.Contract{
			ID: id, Network: "testnet", Status: "active", Label: "tracked",
		}); err != nil {
			t.Fatal(err)
		}
	}
	// An indexed event: untracking must retain history.
	if err := ms.BatchInsertEvents(nil, []store.Event{
		{ID: "evt-1", ContractID: deleteContractID, Network: "testnet", Ledger: 100, LedgerClosedAt: time.Now().UTC(), TxHash: "tx-1", Type: "contract"},
	}); err != nil {
		t.Fatal(err)
	}
	return ms
}

func TestDeleteContractUntracks(t *testing.T) {
	ms := seedDeleteStore(t)
	srv := newTestHandler(ms, true, true)
	ctx := context.Background()

	w := doRequestAsUser(srv, http.MethodDelete, "/api/v1/contracts/"+deleteContractID, "", contributorUser, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d (%s)", w.Code, w.Body.String())
	}

	// The contract is paused, not deleted: the row and its history survive.
	c, err := ms.GetContract(ctx, deleteContractID)
	if err != nil {
		t.Fatalf("GetContract: %v", err)
	}
	if c.Status != "paused" {
		t.Errorf("status = %q, want paused", c.Status)
	}

	// "paused" is exactly the status the indexer skips: the poller only
	// indexes contracts whose status is pending, backfilling or active (see
	// TestPoller_SkipsPausedAndErroredContracts in services/indexer), so
	// the untracked contract stops being indexed.
	events, _, err := ms.ListEvents(ctx, deleteContractID, "", 10, store.EventFilters{})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 || events[0].ID != "evt-1" {
		t.Errorf("events = %+v, want the retained evt-1", events)
	}

	// The other contract is untouched.
	other, err := ms.GetContract(ctx, deleteContractOtherID)
	if err != nil {
		t.Fatalf("GetContract(other): %v", err)
	}
	if other.Status != "active" {
		t.Errorf("other status = %q, want active", other.Status)
	}
}

func TestDeleteContractHistoryStillServed(t *testing.T) {
	ms := seedDeleteStore(t)
	srv := newTestHandler(ms, true, true)

	if w := doRequestAsUser(srv, http.MethodDelete, "/api/v1/contracts/"+deleteContractID, "", contributorUser, ""); w.Code != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d (%s)", w.Code, w.Body.String())
	}
	w := doRequestAsUser(srv, http.MethodGet, "/api/v1/contracts/"+deleteContractID+"/events", "", contributorUser, "")
	if w.Code != http.StatusOK {
		t.Fatalf("events: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var resp struct {
		Events []struct {
			ID string `json:"id"`
		} `json:"events"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Events) != 1 || resp.Events[0].ID != "evt-1" {
		t.Errorf("events = %+v, want the retained evt-1", resp.Events)
	}
}

// The v2 twin (DELETE /api/v2/contracts/{id}) reuses the v1 handler: same
// 204, same status flip, same gate.
func TestDeleteContractV2(t *testing.T) {
	ms := seedDeleteStore(t)
	srv := newTestHandler(ms, true, true)
	ctx := context.Background()

	w := doRequestAsUser(srv, http.MethodDelete, "/api/v2/contracts/"+deleteContractID, "", contributorUser, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d (%s)", w.Code, w.Body.String())
	}
	c, err := ms.GetContract(ctx, deleteContractID)
	if err != nil {
		t.Fatalf("GetContract: %v", err)
	}
	if c.Status != "paused" {
		t.Errorf("status = %q, want paused", c.Status)
	}
}

func TestDeleteContractUnknownID(t *testing.T) {
	srv := newTestHandler(seedDeleteStore(t), true, true)
	w := doRequestAsUser(srv, http.MethodDelete, "/api/v1/contracts/"+deleteUnknownID, "", contributorUser, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%s)", w.Code, w.Body.String())
	}
}

// Deleting an already-paused contract is a no-op success, not an error.
func TestDeleteContractIdempotent(t *testing.T) {
	ms := seedDeleteStore(t)
	srv := newTestHandler(ms, true, true)
	ctx := context.Background()

	for i := range 2 {
		w := doRequestAsUser(srv, http.MethodDelete, "/api/v1/contracts/"+deleteContractID, "", contributorUser, "")
		if w.Code != http.StatusNoContent {
			t.Fatalf("delete %d: want 204, got %d (%s)", i+1, w.Code, w.Body.String())
		}
	}
	c, err := ms.GetContract(ctx, deleteContractID)
	if err != nil {
		t.Fatalf("GetContract: %v", err)
	}
	if c.Status != "paused" {
		t.Errorf("status = %q, want paused", c.Status)
	}
}

func TestDeleteContractRequiresContributorRole(t *testing.T) {
	srv := newTestHandler(seedDeleteStore(t), true, true)
	path := "/api/v1/contracts/" + deleteContractID

	if w := doRequestAsUser(srv, http.MethodDelete, path, "", viewerUser, ""); w.Code != http.StatusForbidden {
		t.Errorf("viewer: want 403, got %d (%s)", w.Code, w.Body.String())
	}
	if w := doRequestAsUser(srv, http.MethodDelete, path, "", "", ""); w.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: want 401, got %d (%s)", w.Code, w.Body.String())
	}
	if w := doRequestAsUser(srv, http.MethodDelete, path, "", contributorUser, ""); w.Code != http.StatusNoContent {
		t.Errorf("contributor: want 204, got %d (%s)", w.Code, w.Body.String())
	}
}

// A read-only API key must not reach a write route even for anonymous callers.
func TestDeleteContractDeniesReadOnlyScope(t *testing.T) {
	srv := newTestHandler(seedDeleteStore(t), true, true)
	w := doRequestAsUser(srv, http.MethodDelete, "/api/v1/contracts/"+deleteContractID, readContractsKey, "", "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestDeleteContractStoreFailure(t *testing.T) {
	ms := seedDeleteStore(t)
	ms.UntrackContractErr = errors.New("boom")
	srv := newTestHandler(ms, true, true)
	w := doRequestAsUser(srv, http.MethodDelete, "/api/v1/contracts/"+deleteContractID, "", contributorUser, "")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d (%s)", w.Code, w.Body.String())
	}
}
