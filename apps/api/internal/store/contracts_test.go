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

func TestContractTokenAndSACDiscovery(t *testing.T) {
	ctx := context.Background()
	ms := NewMockStore()

	// 1. Discover a SAC contract
	sacContractID := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	accountID := "GA5ZSEJYB37JRC5AVCIA5MOP4RHTM335X2KGX3IHOJAPP5RE34K4KZVN"
	sacToken := &TokenMetadata{
		AssetCode: "USDC",
		Issuer:    accountID,
		Decimals:  7,
		Symbol:    "USDC",
		Name:      "USDC:GA5ZSEJYB37JRC5AVCIA5MOP4RHTM335X2KGX3IHOJAPP5RE34K4KZVN",
	}

	tracked, err := ms.RecordDiscoveredContract(ctx, DiscoveredContract{
		ContractID: sacContractID,
		Network:    "testnet",
		AccountID:  accountID,
		Ledger:     100,
		IsSAC:      true,
		SAC:        sacToken,
	})
	if err != nil || !tracked {
		t.Fatalf("expected SAC contract to be tracked: tracked=%v err=%v", tracked, err)
	}

	c, err := ms.GetContract(ctx, sacContractID)
	if err != nil {
		t.Fatalf("failed to get contract: %v", err)
	}
	if c.Label != "USDC" {
		t.Errorf("expected label 'USDC', got %q", c.Label)
	}
	if c.Token == nil || c.Token.AssetCode != "USDC" || c.Token.Decimals != 7 {
		t.Errorf("expected hydrated token with AssetCode USDC, got %+v", c.Token)
	}

	// 2. Discovering non-SAC contract uses DiscoveredByLabel and has nil token
	nonSACID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM"
	tracked, err = ms.RecordDiscoveredContract(ctx, DiscoveredContract{
		ContractID: nonSACID,
		Network:    "testnet",
		AccountID:  accountID,
		Ledger:     101,
		IsSAC:      false,
	})
	if err != nil || !tracked {
		t.Fatalf("expected non-SAC contract to be tracked: %v", err)
	}
	nonSAC, err := ms.GetContract(ctx, nonSACID)
	if err != nil {
		t.Fatalf("failed to get non-SAC contract: %v", err)
	}
	if nonSAC.Label != DiscoveredByLabel(accountID) {
		t.Errorf("expected label %q, got %q", DiscoveredByLabel(accountID), nonSAC.Label)
	}
	if nonSAC.Token != nil {
		t.Errorf("expected nil token for non-SAC contract, got %+v", nonSAC.Token)
	}

	// 3. Manual labels are NEVER overwritten when discovered
	manualID := "CBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBAAAAA"
	if err := ms.UpsertContract(ctx, Contract{
		ID:      manualID,
		Network: "testnet",
		Label:   "MyManualLabel",
		Status:  "active",
	}); err != nil {
		t.Fatal(err)
	}

	tracked, err = ms.RecordDiscoveredContract(ctx, DiscoveredContract{
		ContractID: manualID,
		Network:    "testnet",
		AccountID:  accountID,
		Ledger:     102,
		IsSAC:      true,
		SAC: &TokenMetadata{
			AssetCode: "EURC",
			Issuer:    accountID,
			Decimals:  7,
			Symbol:    "EURC",
			Name:      "EURC",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tracked {
		t.Errorf("expected tracked=false for already existing contract")
	}

	manualContract, err := ms.GetContract(ctx, manualID)
	if err != nil {
		t.Fatal(err)
	}
	if manualContract.Label != "MyManualLabel" {
		t.Errorf("expected manual label 'MyManualLabel' to be preserved, got %q", manualContract.Label)
	}
	if manualContract.Token == nil || manualContract.Token.AssetCode != "EURC" {
		t.Errorf("expected token metadata to be enriched, got %+v", manualContract.Token)
	}

	// 4. Test SearchContracts by token symbol/asset_code
	searchResults, err := ms.SearchContracts(ctx, "usdc", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(searchResults) == 0 || searchResults[0].ID != sacContractID {
		t.Errorf("expected search for 'usdc' to find %s, got %v", sacContractID, searchResults)
	}
}
