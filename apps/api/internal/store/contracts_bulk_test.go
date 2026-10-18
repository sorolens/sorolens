package store

import (
	"context"
	"errors"
	"testing"
)

func seedBulkStore(t *testing.T) *MockStore {
	t.Helper()
	ms := NewMockStore()
	ctx := context.Background()
	for _, id := range []string{"c1", "c2"} {
		if err := ms.UpsertContract(ctx, Contract{
			ID: id, Network: "testnet", Status: "active", Label: "old-" + id,
		}); err != nil {
			t.Fatal(err)
		}
	}
	ms.events = []Event{{ID: "e1", ContractID: "c1"}, {ID: "e2", ContractID: "c2"}}
	ms.invocations = []Invocation{{TxHash: "t1", ContractID: "c1"}}
	ms.storageEntries = []StorageEntry{
		{ContractID: "c1", KeyXDR: "k1"},
		{ContractID: "c2", KeyXDR: "k2"},
	}
	ms.contractUpgrades = []ContractUpgrade{{ID: 1, ContractID: "c1"}}
	ms.syncStates["c1"] = SyncState{ContractID: "c1", LastLedger: 10}
	ms.healthScores = map[string]ContractHealthScore{
		"c1": {ContractID: "c1", Score: 90},
	}
	ms.watchlist["user-1"] = map[string]bool{"c1": true, "c2": true}
	return ms
}

func TestDeleteContractsRemovesContractAndIndexedData(t *testing.T) {
	ms := seedBulkStore(t)
	ctx := context.Background()

	// "missing" is not tracked: it must be ignored, not treated as an error.
	deleted, err := ms.DeleteContracts(ctx, []string{"c1", "missing"})
	if err != nil {
		t.Fatalf("DeleteContracts: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1 (unknown ids are ignored)", deleted)
	}

	if _, err := ms.GetContract(ctx, "c1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("c1 should be gone, got err=%v", err)
	}
	if _, err := ms.GetContract(ctx, "c2"); err != nil {
		t.Errorf("c2 should survive, got err=%v", err)
	}

	// Dependent rows for c1 are gone; c2's are untouched.
	if len(ms.events) != 1 || ms.events[0].ContractID != "c2" {
		t.Errorf("events = %+v, want only c2", ms.events)
	}
	if len(ms.invocations) != 0 {
		t.Errorf("invocations = %+v, want none", ms.invocations)
	}
	if len(ms.storageEntries) != 1 || ms.storageEntries[0].ContractID != "c2" {
		t.Errorf("storageEntries = %+v, want only c2", ms.storageEntries)
	}
	if len(ms.contractUpgrades) != 0 {
		t.Errorf("contractUpgrades = %+v, want none", ms.contractUpgrades)
	}
	if _, ok := ms.syncStates["c1"]; ok {
		t.Error("sync state for c1 should be deleted")
	}
	if _, ok := ms.healthScores["c1"]; ok {
		t.Error("health score for c1 should be deleted")
	}
	if ms.watchlist["user-1"]["c1"] {
		t.Error("watchlist entry for c1 should be deleted")
	}
	if !ms.watchlist["user-1"]["c2"] {
		t.Error("watchlist entry for c2 should survive")
	}
}

func TestDeleteContractsEmptyInputIsNoop(t *testing.T) {
	ms := seedBulkStore(t)
	deleted, err := ms.DeleteContracts(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0", deleted)
	}
	if len(ms.contracts) != 2 {
		t.Fatalf("contracts = %d, want 2 (nothing deleted)", len(ms.contracts))
	}
}

func TestSetContractLabelUpdatesSelectedContracts(t *testing.T) {
	ms := seedBulkStore(t)
	ctx := context.Background()

	updated, err := ms.SetContractLabel(ctx, []string{"c1", "missing"}, "payments")
	if err != nil {
		t.Fatalf("SetContractLabel: %v", err)
	}
	if updated != 1 {
		t.Fatalf("updated = %d, want 1", updated)
	}

	c1, err := ms.GetContract(ctx, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if c1.Label != "payments" {
		t.Errorf("c1 label = %q, want payments", c1.Label)
	}
	c2, err := ms.GetContract(ctx, "c2")
	if err != nil {
		t.Fatal(err)
	}
	if c2.Label != "old-c2" {
		t.Errorf("c2 label = %q, want unchanged old-c2", c2.Label)
	}
}

func TestBulkStoreErrorInjection(t *testing.T) {
	ms := seedBulkStore(t)
	ctx := context.Background()

	ms.DeleteContractsErr = errors.New("boom")
	if _, err := ms.DeleteContracts(ctx, []string{"c1"}); !errors.Is(err, ms.DeleteContractsErr) {
		t.Errorf("DeleteContracts err = %v, want injected", err)
	}
	ms.DeleteContractsErr = nil

	ms.SetContractLabelErr = errors.New("boom")
	if _, err := ms.SetContractLabel(ctx, []string{"c1"}, "x"); !errors.Is(err, ms.SetContractLabelErr) {
		t.Errorf("SetContractLabel err = %v, want injected", err)
	}
}
