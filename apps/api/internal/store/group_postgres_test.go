package store_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestGroupPostgresAggregation exercises the real SQL backing GroupStore
// (single-query aggregation, membership idempotency, owner scoping, cascade
// deletes). It is gated on POSTGRES_TEST_URL, mirroring the performance test,
// and skips when no database is configured.
func TestGroupPostgresAggregation(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_URL")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_URL not set, skipping DB test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, `TRUNCATE TABLE group_contracts, groups, contract_health_scores,
		storage_entries, invocations, events, contracts, users CASCADE`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}

	s := store.NewFullStore(pool)
	if err := s.UpsertUser(ctx, store.User{ID: "user_1"}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := s.UpsertUser(ctx, store.User{ID: "user_2"}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}

	for _, id := range []string{"CONTRACT_A", "CONTRACT_B", "CONTRACT_C"} {
		if err := s.UpsertContract(ctx, store.Contract{ID: id, Network: "testnet", Status: "active"}); err != nil {
			t.Fatalf("UpsertContract(%s): %v", id, err)
		}
	}

	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	err = s.BatchInsertEvents(ctx, []store.Event{
		{ID: "ev-a1", ContractID: "CONTRACT_A", Network: "testnet", Ledger: 1, LedgerClosedAt: at, TxHash: "tx1", Type: "contract"},
		{ID: "ev-a2", ContractID: "CONTRACT_A", Network: "testnet", Ledger: 2, LedgerClosedAt: at.Add(time.Hour), TxHash: "tx2", Type: "contract"},
		{ID: "ev-b1", ContractID: "CONTRACT_B", Network: "testnet", Ledger: 3, LedgerClosedAt: at.Add(2 * time.Hour), TxHash: "tx3", Type: "contract"},
	})
	if err != nil {
		t.Fatalf("BatchInsertEvents: %v", err)
	}
	err = s.BatchInsertInvocations(ctx, []store.Invocation{
		{TxHash: "itx1", ContractID: "CONTRACT_A", Network: "testnet", Ledger: 4, LedgerClosedAt: at, Status: "SUCCESS"},
		{TxHash: "itx2", ContractID: "CONTRACT_B", Network: "testnet", Ledger: 5, LedgerClosedAt: at, Status: "SUCCESS"},
	})
	if err != nil {
		t.Fatalf("BatchInsertInvocations: %v", err)
	}
	err = s.UpsertStorageEntries(ctx, []store.StorageEntry{
		{ContractID: "CONTRACT_A", Network: "testnet", KeyXDR: "ka1", Durability: "persistent"},
		{ContractID: "CONTRACT_A", Network: "testnet", KeyXDR: "ka2", Durability: "persistent"},
		{ContractID: "CONTRACT_B", Network: "testnet", KeyXDR: "kb1", Durability: "persistent"},
	})
	if err != nil {
		t.Fatalf("UpsertStorageEntries: %v", err)
	}
	for contractID, score := range map[string]int32{"CONTRACT_A": 80, "CONTRACT_B": 60} {
		err := s.UpsertContractHealthScore(ctx, store.ContractHealthScore{ContractID: contractID, Score: score})
		if err != nil {
			t.Fatalf("UpsertContractHealthScore(%s): %v", contractID, err)
		}
	}

	g, err := s.CreateGroup(ctx, "user_1", "  Core  ")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if g.ID == "" || g.Name != "Core" {
		t.Fatalf("CreateGroup returned %+v, want a generated id and trimmed name", g)
	}
	for _, id := range []string{"CONTRACT_A", "CONTRACT_B", "CONTRACT_C", "CONTRACT_A"} {
		if err := s.AddContractToGroup(ctx, "user_1", g.ID, id); err != nil {
			t.Fatalf("AddContractToGroup(%s): %v", id, err)
		}
	}

	// Multiple groups can hold the same contract.
	other, err := s.CreateGroup(ctx, "user_1", "Empty")
	if err != nil {
		t.Fatalf("CreateGroup(empty): %v", err)
	}
	if err := s.AddContractToGroup(ctx, "user_1", other.ID, "CONTRACT_A"); err != nil {
		t.Fatalf("AddContractToGroup(other): %v", err)
	}

	stats, err := s.GetGroupStats(ctx, "user_1", g.ID)
	if err != nil {
		t.Fatalf("GetGroupStats: %v", err)
	}
	if stats.ContractCount != 3 || stats.EventCount != 3 || stats.InvocationCount != 2 ||
		stats.StorageEntryCount != 3 || stats.AverageHealthScore != 70 {
		t.Errorf("stats = %+v, want 3 contracts / 3 events / 2 invocations / 3 storage / avg 70", stats)
	}

	// The list endpoint carries the same aggregates plus each group's name.
	groups, err := s.ListGroups(ctx, "user_1")
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if groups[0].ID != other.ID {
		t.Errorf("ListGroups newest-first: got %q first, want %q", groups[0].ID, other.ID)
	}

	contracts, err := s.ListGroupContracts(ctx, "user_1", g.ID)
	if err != nil {
		t.Fatalf("ListGroupContracts: %v", err)
	}
	if len(contracts) != 3 {
		t.Fatalf("len(contracts) = %d, want 3", len(contracts))
	}
	if contracts[0].ContractID != "CONTRACT_A" || contracts[0].HealthScore == nil || *contracts[0].HealthScore != 80 {
		t.Errorf("CONTRACT_A row = %+v, want health score 80", contracts[0])
	}
	if contracts[0].LastActivityAt == nil {
		t.Errorf("CONTRACT_A row has no last activity, want the newest event/invocation")
	}
	if contracts[2].ContractID != "CONTRACT_C" || contracts[2].HealthScore != nil || contracts[2].LastActivityAt != nil {
		t.Errorf("CONTRACT_C row = %+v, want nil health and activity", contracts[2])
	}

	// Owner scoping: another user sees nothing.
	if _, err := s.GetGroup(ctx, "user_2", g.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetGroup(other user) = %v, want ErrNotFound", err)
	}
	if _, err := s.GetGroupStats(ctx, "user_2", g.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetGroupStats(other user) = %v, want ErrNotFound", err)
	}

	// Rename, then delete: memberships cascade, contracts survive.
	renamed, err := s.UpdateGroup(ctx, "user_1", g.ID, "Renamed")
	if err != nil || renamed.Name != "Renamed" {
		t.Fatalf("UpdateGroup = (%+v, %v), want name Renamed", renamed, err)
	}
	if err := s.RemoveContractFromGroup(ctx, "user_1", g.ID, "CONTRACT_C"); err != nil {
		t.Fatalf("RemoveContractFromGroup: %v", err)
	}
	after, err := s.GetGroupStats(ctx, "user_1", g.ID)
	if err != nil {
		t.Fatalf("GetGroupStats after remove: %v", err)
	}
	if after.ContractCount != 2 || after.EventCount != 3 || after.StorageEntryCount != 3 {
		t.Errorf("stats after removing CONTRACT_C = %+v, want 2 contracts / 3 events / 3 storage", after)
	}
	if err := s.DeleteGroup(ctx, "user_1", g.ID); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	if _, err := s.GetGroup(ctx, "user_1", g.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetGroup after delete = %v, want ErrNotFound", err)
	}
	if _, err := s.GetContract(ctx, "CONTRACT_A"); err != nil {
		t.Errorf("contracts must survive group deletion: %v", err)
	}

	// The self-asserted X-User-ID identity is provisioned on first group
	// creation, so a brand-new owner can use groups without an auth flow.
	fresh, err := s.CreateGroup(ctx, "user_fresh", "Fresh owner")
	if err != nil {
		t.Fatalf("CreateGroup for a new owner: %v", err)
	}
	if _, err := s.GetGroup(ctx, "user_fresh", fresh.ID); err != nil {
		t.Errorf("GetGroup for a new owner: %v", err)
	}

	// A malformed group ID is a clean not-found, not a uuid cast error.
	if _, err := s.GetGroup(ctx, "user_1", "not-a-uuid"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetGroup(malformed id) = %v, want ErrNotFound", err)
	}
	if _, err := s.GetGroupStats(ctx, "user_1", "not-a-uuid"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetGroupStats(malformed id) = %v, want ErrNotFound", err)
	}
}
