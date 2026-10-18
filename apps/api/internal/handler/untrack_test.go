package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// seedUntrackStore seeds the RBAC users/keys plus one active, tracked contract
// with a single indexed event, so untrack tests can assert both the status
// flip and that history survives the soft delete.
func seedUntrackStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := seedRBACUsers(t)
	if err := ms.UpsertContract(nil, store.Contract{
		ID:      netContractA,
		Network: "testnet",
		Label:   "tracked contract",
		Status:  "active",
	}); err != nil {
		t.Fatal(err)
	}
	if err := ms.BatchInsertEvents(nil, []store.Event{{
		ID:             "evt-untrack-1",
		ContractID:     netContractA,
		Network:        "testnet",
		Ledger:         100,
		LedgerClosedAt: time.Now().UTC(),
		TxHash:         "tx-untrack-1",
		Type:           "transfer",
	}}); err != nil {
		t.Fatal(err)
	}
	return ms
}

// TestUntrackContractStopsIndexingKeepsHistory covers the happy path: a
// contributor untracks a contract, which becomes paused (no longer polled by
// the indexer), disappears from the active set, keeps its indexed history,
// and can be re-registered to resume tracking.
func TestUntrackContractStopsIndexingKeepsHistory(t *testing.T) {
	ms := seedUntrackStore(t)
	srv := newTestHandler(ms, true, true)
	ctx := context.Background()

	w := doRequestAsUser(srv, http.MethodDelete, "/api/v1/contracts/"+netContractA, "", contributorUser, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("contributor untrack: want 204, got %d (%s)", w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Errorf("want empty 204 body, got %q", w.Body.String())
	}

	// Soft delete: status flips to paused so the indexer skips it.
	c, err := ms.GetContract(ctx, netContractA)
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != store.ContractStatusPaused {
		t.Errorf("status: want %q, got %q", store.ContractStatusPaused, c.Status)
	}

	active, _, err := ms.ListContracts(ctx, "", 50, store.ContractFilters{Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Errorf("active contracts after untrack: want 0, got %d", len(active))
	}
	paused, _, err := ms.ListContracts(ctx, "", 50, store.ContractFilters{Status: store.ContractStatusPaused})
	if err != nil {
		t.Fatal(err)
	}
	if len(paused) != 1 || paused[0].ID != netContractA {
		t.Errorf("paused contracts: want [%s], got %v", netContractA, paused)
	}

	// History is retained.
	events, _, err := ms.ListEvents(ctx, netContractA, "", 10, store.EventFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != "evt-untrack-1" {
		t.Errorf("history retained: want evt-untrack-1, got %v", events)
	}

	// Untracking is idempotent.
	w = doRequestAsUser(srv, http.MethodDelete, "/api/v1/contracts/"+netContractA, "", contributorUser, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("second untrack: want 204, got %d (%s)", w.Code, w.Body.String())
	}

	// Re-registering resumes tracking (status returns to pending).
	w = doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts", "", contributorUser, validContractBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("re-register: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "pending" {
		t.Errorf("re-registered status: want pending, got %v", resp["status"])
	}
}

// TestUntrackContractNotFound verifies a missing contract answers 404 with
// the standard error envelope.
func TestUntrackContractNotFound(t *testing.T) {
	srv := newTestHandler(seedUntrackStore(t), true, true)

	w := doRequestAsUser(srv, http.MethodDelete, "/api/v1/contracts/CNOSUCHCONTRACT", "", contributorUser, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%s)", w.Code, w.Body.String())
	}
	var env map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	errObj, _ := env["error"].(map[string]any)
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("want code=NOT_FOUND, got %v", errObj["code"])
	}
}

// TestUntrackContractRBAC verifies the same contributor+ role gate as
// registration protects untracking.
func TestUntrackContractRBAC(t *testing.T) {
	srv := newTestHandler(seedUntrackStore(t), true, true)

	cases := []struct {
		name   string
		userID string
		want   int
	}{
		{"viewer denied untrack", viewerUser, http.StatusForbidden},
		{"admin untracks", adminUser, http.StatusNoContent},
		{"anonymous denied untrack", "", http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doRequestAsUser(srv, http.MethodDelete, "/api/v1/contracts/"+netContractA, "", tc.userID, "")
			if w.Code != tc.want {
				t.Fatalf("want %d, got %d (%s)", tc.want, w.Code, w.Body.String())
			}
		})
	}
}

// TestUntrackContractRequiresWriteScope verifies the scope layer: a
// read:contracts credential cannot untrack even when the caller is a
// contributor, while an admin-scoped credential can.
func TestUntrackContractRequiresWriteScope(t *testing.T) {
	ms := seedUntrackStore(t)
	srv := newTestHandler(ms, true, true)

	w := doRequestAsUser(srv, http.MethodDelete, "/api/v1/contracts/"+netContractA, readContractsKey, contributorUser, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("read:contracts untrack: want 403, got %d (%s)", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "missing scope" {
		t.Errorf(`error: want "missing scope", got %q`, body["error"])
	}

	// Denied by scope, so the contract is untouched.
	c, err := ms.GetContract(context.Background(), netContractA)
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != "active" {
		t.Errorf("contract status after denied untrack: want active, got %q", c.Status)
	}

	w = doRequestAsUser(srv, http.MethodDelete, "/api/v1/contracts/"+netContractA, adminToken, contributorUser, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("admin-scoped key untrack: want 204, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestUntrackContractStoreError verifies store failures surface as 500 with
// the standard error envelope instead of silently succeeding.
func TestUntrackContractStoreError(t *testing.T) {
	ms := seedUntrackStore(t)
	ms.UntrackContractErr = context.DeadlineExceeded
	srv := newTestHandler(ms, true, true)

	w := doRequestAsUser(srv, http.MethodDelete, "/api/v1/contracts/"+netContractA, "", contributorUser, "")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d (%s)", w.Code, w.Body.String())
	}
}
