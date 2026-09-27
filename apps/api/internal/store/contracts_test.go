package store

import (
	"context"
	"testing"
	"time"
)

func seedSortableContracts(t *testing.T, ms *MockStore) {
	t.Helper()
	added := map[string]time.Time{
		"c1": time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		"c2": time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		"c3": time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
	}
	for id, when := range added {
		if err := ms.UpsertContract(context.Background(), Contract{
			ID:      id,
			Network: "testnet",
			Label:   "label-" + id,
			Status:  "active",
			AddedAt: when,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func sortedIDs(t *testing.T, ms *MockStore, f ContractFilters) []string {
	t.Helper()
	contracts, next, err := ms.ListContracts(context.Background(), "", 50, f)
	if err != nil {
		t.Fatal(err)
	}
	if next != "" {
		t.Fatalf("want no next cursor for small fixture, got %q", next)
	}
	out := make([]string, 0, len(contracts))
	for _, c := range contracts {
		out = append(out, c.ID)
	}
	return out
}

func TestListContractsDefaultSortsByID(t *testing.T) {
	ms := NewMockStore()
	seedSortableContracts(t, ms)

	got := sortedIDs(t, ms, ContractFilters{})
	want := []string{"c1", "c2", "c3"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("default order = %v, want %v", got, want)
		}
	}
}

func TestListContractsSortsByAddedAt(t *testing.T) {
	ms := NewMockStore()
	seedSortableContracts(t, ms)

	got := sortedIDs(t, ms, ContractFilters{Sort: "added_at", SortDir: "asc"})
	want := []string{"c1", "c3", "c2"} // Jan, Feb, Mar
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("added_at asc = %v, want %v", got, want)
		}
	}

	got = sortedIDs(t, ms, ContractFilters{Sort: "added_at", SortDir: "desc"})
	want = []string{"c2", "c3", "c1"} // Mar, Feb, Jan
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("added_at desc = %v, want %v", got, want)
		}
	}
}

func TestListContractsUnknownSortFallsBackToID(t *testing.T) {
	ms := NewMockStore()
	seedSortableContracts(t, ms)

	got := sortedIDs(t, ms, ContractFilters{Sort: "nope", SortDir: "asc"})
	want := []string{"c1", "c2", "c3"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unknown sort order = %v, want %v", got, want)
		}
	}
}

func TestValidContractSort(t *testing.T) {
	for _, col := range []string{"id", "label", "network", "status", "added_at"} {
		if !ValidContractSort(col) {
			t.Errorf("ValidContractSort(%q) = false, want true", col)
		}
	}
	if ValidContractSort("created_at_ledger; drop table contracts") {
		t.Error("ValidContractSort accepted an injected value")
	}
}