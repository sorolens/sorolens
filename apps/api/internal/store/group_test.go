package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// seedGroupStore builds a mock with three tracked contracts, activity for two
// of them, and cached health scores for the same two. The third contract is a
// group member with no indexed data, exercising the LEFT JOIN / zero-fill path.
func seedGroupStore(t *testing.T) *store.MockStore {
	t.Helper()
	ctx := context.Background()
	ms := store.NewMockStore()

	for _, id := range []string{"CONTRACT_A", "CONTRACT_B", "CONTRACT_C"} {
		if err := ms.UpsertContract(ctx, store.Contract{ID: id, Network: "testnet", Status: "active"}); err != nil {
			t.Fatalf("UpsertContract(%s): %v", id, err)
		}
	}
	// CONTRACT_A: 2 events, 1 invocation, 2 storage entries, score 80.
	// CONTRACT_B: 1 event, 1 invocation, 1 storage entry, score 60.
	// CONTRACT_C: nothing.
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	err := ms.BatchInsertEvents(ctx, []store.Event{
		{ID: "ev-a1", ContractID: "CONTRACT_A", LedgerClosedAt: at},
		{ID: "ev-a2", ContractID: "CONTRACT_A", LedgerClosedAt: at.Add(time.Hour)},
		{ID: "ev-b1", ContractID: "CONTRACT_B", LedgerClosedAt: at.Add(2 * time.Hour)},
	})
	if err != nil {
		t.Fatalf("BatchInsertEvents: %v", err)
	}
	err = ms.BatchInsertInvocations(ctx, []store.Invocation{
		{TxHash: "tx-a1", ContractID: "CONTRACT_A", LedgerClosedAt: at},
		{TxHash: "tx-b1", ContractID: "CONTRACT_B", LedgerClosedAt: at.Add(3 * time.Hour)},
	})
	if err != nil {
		t.Fatalf("BatchInsertInvocations: %v", err)
	}
	err = ms.UpsertStorageEntries(ctx, []store.StorageEntry{
		{ContractID: "CONTRACT_A", KeyXDR: "k1"},
		{ContractID: "CONTRACT_A", KeyXDR: "k2"},
		{ContractID: "CONTRACT_B", KeyXDR: "k3"},
	})
	if err != nil {
		t.Fatalf("UpsertStorageEntries: %v", err)
	}
	for contractID, score := range map[string]int32{"CONTRACT_A": 80, "CONTRACT_B": 60} {
		err := ms.UpsertContractHealthScore(ctx, store.ContractHealthScore{
			ContractID: contractID,
			Score:      score,
		})
		if err != nil {
			t.Fatalf("UpsertContractHealthScore(%s): %v", contractID, err)
		}
	}
	return ms
}

func addMembers(t *testing.T, ms *store.MockStore, ownerID, groupID string, contractIDs ...string) {
	t.Helper()
	for _, contractID := range contractIDs {
		if err := ms.AddContractToGroup(context.Background(), ownerID, groupID, contractID); err != nil {
			t.Fatalf("AddContractToGroup(%s): %v", contractID, err)
		}
	}
}

// TestGroupStatsAggregation covers the aggregation query contract: event,
// invocation, and storage counts summed across member contracts, and the
// average health score taken over members that have a cached score.
func TestGroupStatsAggregation(t *testing.T) {
	ctx := context.Background()
	ms := seedGroupStore(t)

	// CONTRACT_D has activity but is deliberately NOT a member, so its rows
	// must stay out of the group totals.
	if err := ms.UpsertContract(ctx, store.Contract{ID: "CONTRACT_D", Network: "testnet", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if err := ms.BatchInsertEvents(ctx, []store.Event{{ID: "ev-d1", ContractID: "CONTRACT_D"}}); err != nil {
		t.Fatal(err)
	}

	g, err := ms.CreateGroup(ctx, "user_1", "Core protocol")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	addMembers(t, ms, "user_1", g.ID, "CONTRACT_A", "CONTRACT_B", "CONTRACT_C")

	stats, err := ms.GetGroupStats(ctx, "user_1", g.ID)
	if err != nil {
		t.Fatalf("GetGroupStats: %v", err)
	}
	if stats.GroupID != g.ID {
		t.Errorf("group_id = %q, want %q", stats.GroupID, g.ID)
	}
	if stats.ContractCount != 3 {
		t.Errorf("contract_count = %d, want 3", stats.ContractCount)
	}
	if stats.EventCount != 3 {
		t.Errorf("event_count = %d, want 3", stats.EventCount)
	}
	if stats.InvocationCount != 2 {
		t.Errorf("invocation_count = %d, want 2", stats.InvocationCount)
	}
	if stats.StorageEntryCount != 3 {
		t.Errorf("storage_entry_count = %d, want 3", stats.StorageEntryCount)
	}
	if stats.AverageHealthScore != 70 {
		t.Errorf("average_health_score = %v, want 70", stats.AverageHealthScore)
	}
}

// TestListGroupsSummaries checks that each group in the list carries its own
// aggregates and that groups are returned newest first.
func TestListGroupsSummaries(t *testing.T) {
	ctx := context.Background()
	ms := seedGroupStore(t)

	first, err := ms.CreateGroup(ctx, "user_1", "Older")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	second, err := ms.CreateGroup(ctx, "user_1", "Newer")
	if err != nil {
		t.Fatal(err)
	}
	addMembers(t, ms, "user_1", first.ID, "CONTRACT_A")
	addMembers(t, ms, "user_1", second.ID, "CONTRACT_A", "CONTRACT_B", "CONTRACT_C")

	// A second user's group must not leak into the first user's list.
	if _, err := ms.CreateGroup(ctx, "user_2", "Other"); err != nil {
		t.Fatal(err)
	}

	groups, err := ms.ListGroups(ctx, "user_1")
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if groups[0].ID != second.ID || groups[1].ID != first.ID {
		t.Errorf("groups not newest-first: got [%s, %s]", groups[0].Name, groups[1].Name)
	}
	if groups[0].ContractCount != 3 || groups[0].EventCount != 3 || groups[0].InvocationCount != 2 || groups[0].StorageEntryCount != 3 {
		t.Errorf("newer group aggregates = %+v, want 3 contracts / 3 events / 2 invocations / 3 storage", groups[0])
	}
	if groups[0].AverageHealthScore != 70 {
		t.Errorf("newer group avg health = %v, want 70", groups[0].AverageHealthScore)
	}
	if groups[1].ContractCount != 1 || groups[1].EventCount != 2 {
		t.Errorf("older group aggregates = %+v, want 1 contract / 2 events", groups[1])
	}
	if groups[1].AverageHealthScore != 80 {
		t.Errorf("older group avg health = %v, want 80", groups[1].AverageHealthScore)
	}
}

// TestGroupStatsEmptyGroup ensures a group with no members aggregates to
// zeroes instead of erroring.
func TestGroupStatsEmptyGroup(t *testing.T) {
	ctx := context.Background()
	ms := seedGroupStore(t)
	g, err := ms.CreateGroup(ctx, "user_1", "Empty")
	if err != nil {
		t.Fatal(err)
	}
	stats, err := ms.GetGroupStats(ctx, "user_1", g.ID)
	if err != nil {
		t.Fatalf("GetGroupStats: %v", err)
	}
	if stats.ContractCount != 0 || stats.EventCount != 0 || stats.InvocationCount != 0 ||
		stats.StorageEntryCount != 0 || stats.AverageHealthScore != 0 {
		t.Errorf("empty group stats = %+v, want all zeroes", stats)
	}
}

// TestGroupMembershipManyToMany covers the acceptance criterion that a contract
// can belong to multiple groups, that add is idempotent, and that removing from
// one group leaves the other intact.
func TestGroupMembershipManyToMany(t *testing.T) {
	ctx := context.Background()
	ms := seedGroupStore(t)

	mainnet, err := ms.CreateGroup(ctx, "user_1", "Mainnet")
	if err != nil {
		t.Fatal(err)
	}
	critical, err := ms.CreateGroup(ctx, "user_1", "Critical")
	if err != nil {
		t.Fatal(err)
	}

	addMembers(t, ms, "user_1", mainnet.ID, "CONTRACT_A")
	addMembers(t, ms, "user_1", critical.ID, "CONTRACT_A")
	// Adding the same membership twice is a no-op.
	addMembers(t, ms, "user_1", critical.ID, "CONTRACT_A")

	if got := len(membersOf(t, ms, "user_1", critical.ID)); got != 1 {
		t.Fatalf("critical members = %d, want 1", got)
	}
	if got := len(membersOf(t, ms, "user_1", mainnet.ID)); got != 1 {
		t.Fatalf("mainnet members = %d, want 1", got)
	}

	if err := ms.RemoveContractFromGroup(ctx, "user_1", critical.ID, "CONTRACT_A"); err != nil {
		t.Fatalf("RemoveContractFromGroup: %v", err)
	}
	if got := len(membersOf(t, ms, "user_1", critical.ID)); got != 0 {
		t.Errorf("critical members after remove = %d, want 0", got)
	}
	if got := len(membersOf(t, ms, "user_1", mainnet.ID)); got != 1 {
		t.Errorf("mainnet members after removing from critical = %d, want 1", got)
	}

	// Removing a non-member is a no-op, not an error.
	if err := ms.RemoveContractFromGroup(ctx, "user_1", mainnet.ID, "CONTRACT_B"); err != nil {
		t.Errorf("RemoveContractFromGroup(non-member) = %v, want nil", err)
	}
}

// TestListGroupContracts covers the per-contract detail rows: cached health
// score, last-activity timestamp, and the blank-slate member.
func TestListGroupContracts(t *testing.T) {
	ctx := context.Background()
	ms := seedGroupStore(t)
	g, err := ms.CreateGroup(ctx, "user_1", "Portfolio")
	if err != nil {
		t.Fatal(err)
	}
	addMembers(t, ms, "user_1", g.ID, "CONTRACT_A", "CONTRACT_B", "CONTRACT_C")

	contracts, err := ms.ListGroupContracts(ctx, "user_1", g.ID)
	if err != nil {
		t.Fatalf("ListGroupContracts: %v", err)
	}
	if len(contracts) != 3 {
		t.Fatalf("len(contracts) = %d, want 3", len(contracts))
	}
	byID := map[string]store.GroupContract{}
	for _, c := range contracts {
		byID[c.ContractID] = c
	}
	if got := byID["CONTRACT_A"].HealthScore; got == nil || *got != 80 {
		t.Errorf("CONTRACT_A health score = %v, want 80", got)
	}
	if got := byID["CONTRACT_A"].LastActivityAt; got == nil || !got.Equal(time.Date(2026, 1, 2, 4, 4, 5, 0, time.UTC)) {
		t.Errorf("CONTRACT_A last activity = %v, want the latest event/invocation", got)
	}
	if got := byID["CONTRACT_C"].HealthScore; got != nil {
		t.Errorf("CONTRACT_C health score = %v, want nil", *got)
	}
	if got := byID["CONTRACT_C"].LastActivityAt; got != nil {
		t.Errorf("CONTRACT_C last activity = %v, want nil", *got)
	}
}

// TestGroupOwnerScoping verifies every read and mutation is scoped to the
// caller: another user's group is treated as missing.
func TestGroupOwnerScoping(t *testing.T) {
	ctx := context.Background()
	ms := seedGroupStore(t)
	g, err := ms.CreateGroup(ctx, "user_1", "Mine")
	if err != nil {
		t.Fatal(err)
	}
	addMembers(t, ms, "user_1", g.ID, "CONTRACT_A")

	if _, err := ms.GetGroup(ctx, "user_2", g.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetGroup as other user = %v, want ErrNotFound", err)
	}
	if _, err := ms.GetGroupStats(ctx, "user_2", g.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetGroupStats as other user = %v, want ErrNotFound", err)
	}
	if err := ms.AddContractToGroup(ctx, "user_2", g.ID, "CONTRACT_B"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("AddContractToGroup as other user = %v, want ErrNotFound", err)
	}
	if err := ms.DeleteGroup(ctx, "user_2", g.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("DeleteGroup as other user = %v, want ErrNotFound", err)
	}
	if groups, err := ms.ListGroups(ctx, "user_2"); err != nil || len(groups) != 0 {
		t.Errorf("ListGroups as other user = (%v, %v), want empty", groups, err)
	}

	// The owner still sees it.
	if _, err := ms.GetGroup(ctx, "user_1", g.ID); err != nil {
		t.Errorf("GetGroup as owner = %v, want nil", err)
	}
}

// TestGroupNameValidation rejects empty and whitespace-only names on create and
// rename.
func TestGroupNameValidation(t *testing.T) {
	ctx := context.Background()
	if _, err := store.NewMockStore().CreateGroup(ctx, "user_1", "   "); !errors.Is(err, store.ErrInvalidGroupName) {
		t.Errorf("CreateGroup(blank) = %v, want ErrInvalidGroupName", err)
	}

	ms := seedGroupStore(t)
	g, err := ms.CreateGroup(ctx, "user_1", "Valid")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ms.UpdateGroup(ctx, "user_1", g.ID, ""); !errors.Is(err, store.ErrInvalidGroupName) {
		t.Errorf("UpdateGroup(blank) = %v, want ErrInvalidGroupName", err)
	}
	updated, err := ms.UpdateGroup(ctx, "user_1", g.ID, "  Renamed  ")
	if err != nil {
		t.Fatalf("UpdateGroup: %v", err)
	}
	if updated.Name != "Renamed" {
		t.Errorf("name = %q, want trimmed %q", updated.Name, "Renamed")
	}
}

func membersOf(t *testing.T, ms *store.MockStore, ownerID, groupID string) []store.GroupContract {
	t.Helper()
	members, err := ms.ListGroupContracts(context.Background(), ownerID, groupID)
	if err != nil {
		t.Fatalf("ListGroupContracts: %v", err)
	}
	return members
}
