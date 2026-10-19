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

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_URL")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_URL not set, skipping DB test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestWatchedAccountsAndDiscoveryPostgres(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE watched_accounts, contracts CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	s := store.NewFullStore(pool)

	const (
		account  = "GAAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQDZ7H"
		deployed = "CABQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGCK3"
		manual   = "CAAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQDZ7H"
	)

	a, created, err := s.AddWatchedAccount(ctx, store.WatchedAccount{AccountID: account, AddedBy: "alice"})
	if err != nil || !created || a.AddedBy != "alice" || a.CreatedAt.IsZero() {
		t.Fatalf("add: created=%v err=%v row=%+v", created, err, a)
	}
	a, created, err = s.AddWatchedAccount(ctx, store.WatchedAccount{AccountID: account, AddedBy: "bob"})
	if err != nil || created || a.AddedBy != "alice" {
		t.Fatalf("re-add must return the existing row: created=%v err=%v row=%+v", created, err, a)
	}

	tracked, err := s.RecordDiscoveredContract(ctx, store.DiscoveredContract{
		ContractID: deployed, Network: "testnet", AccountID: account, Ledger: 5000,
	})
	if err != nil || !tracked {
		t.Fatalf("record: tracked=%v err=%v", tracked, err)
	}
	c, err := s.GetContract(ctx, deployed)
	if err != nil {
		t.Fatalf("get discovered contract: %v", err)
	}
	if c.Label != "discovered_by:"+account || c.Status != "active" || c.CreatedAtLedger != 5000 {
		t.Fatalf("unexpected discovered contract: %+v", c)
	}
	ss, err := s.GetSyncState(ctx, deployed)
	if err != nil || ss.LastLedger != 4999 {
		t.Fatalf("sync state must start before the deploy ledger: %+v err=%v", ss, err)
	}

	// Recording the same contract again is a no-op.
	tracked, err = s.RecordDiscoveredContract(ctx, store.DiscoveredContract{
		ContractID: deployed, Network: "testnet", AccountID: account, Ledger: 5000,
	})
	if err != nil || tracked {
		t.Fatalf("re-record: tracked=%v err=%v", tracked, err)
	}

	// A manually tracked contract keeps its label.
	if err := s.UpsertContract(ctx, store.Contract{ID: manual, Network: "testnet", Label: "mine", Status: "pending"}); err != nil {
		t.Fatal(err)
	}
	tracked, err = s.RecordDiscoveredContract(ctx, store.DiscoveredContract{
		ContractID: manual, Network: "testnet", AccountID: account, Ledger: 6000,
	})
	if err != nil || tracked {
		t.Fatalf("manual contract: tracked=%v err=%v", tracked, err)
	}
	if c, _ := s.GetContract(ctx, manual); c.Label != "mine" {
		t.Fatalf("manual label overwritten: %q", c.Label)
	}

	accounts, err := s.ListWatchedAccounts(ctx)
	if err != nil || len(accounts) != 1 || accounts[0].DiscoveredCount != 1 {
		t.Fatalf("list: %+v err=%v", accounts, err)
	}

	if err := s.DeleteWatchedAccount(ctx, account); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.DeleteWatchedAccount(ctx, account); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second delete: want ErrNotFound, got %v", err)
	}
	if _, err := s.GetContract(ctx, deployed); err != nil {
		t.Fatalf("discovered contract must survive account removal: %v", err)
	}
}

func TestAuditEventsPostgres(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE audit_events"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	s := store.NewFullStore(pool)

	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		if err := s.InsertAuditEvent(ctx, store.AuditEvent{
			Actor: "alice", Action: "POST /api/v1/contracts", ResourceType: "contracts",
			IP: "10.0.0.1", UserAgent: "test", RequestBodyHash: "abc", Status: 201,
			At: base.Add(time.Duration(i) * time.Hour),
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	page1, next, err := s.ListAuditEvents(ctx, time.Time{}, 0, 2)
	if err != nil || len(page1) != 2 || next == 0 {
		t.Fatalf("page 1: %d rows next=%d err=%v", len(page1), next, err)
	}
	if !page1[0].At.After(page1[1].At) || page1[0].Actor != "alice" || page1[0].Status != 201 {
		t.Fatalf("page 1 must be newest first: %+v", page1)
	}
	page2, next2, err := s.ListAuditEvents(ctx, time.Time{}, next, 2)
	if err != nil || len(page2) != 2 || page2[0].ID >= page1[1].ID {
		t.Fatalf("page 2: %+v next=%d err=%v", page2, next2, err)
	}
	page3, next3, err := s.ListAuditEvents(ctx, time.Time{}, next2, 2)
	if err != nil || len(page3) != 1 || next3 != 0 {
		t.Fatalf("page 3: %+v next=%d err=%v", page3, next3, err)
	}

	since, _, err := s.ListAuditEvents(ctx, base.Add(3*time.Hour), 0, 10)
	if err != nil || len(since) != 2 {
		t.Fatalf("since: want 2 rows, got %d err=%v", len(since), err)
	}
}
