package store_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestListContractsSortIntegration exercises the real SQL against PostgreSQL.
// It is skipped unless POSTGRES_TEST_URL is set, matching the performance
// baseline test. CI sets the variable and applies the migrations first.
func TestListContractsSortIntegration(t *testing.T) {
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

	if _, err := pool.Exec(ctx, "TRUNCATE TABLE contracts CASCADE"); err != nil {
		t.Fatalf("truncate contracts: %v", err)
	}

	s := store.NewFullStore(pool)
	// Anchor all timestamps to the first of the current month so the rows land
	// in the current-month partition if the events table is partitioned. The
	// values stay distinct enough to assert every ordering.
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	at := func(hours int) time.Time { return monthStart.Add(time.Duration(hours) * time.Hour) }

	contractA := "C" + strings.Repeat("A", 55)
	contractB := "C" + strings.Repeat("B", 55)
	contractC := "C" + strings.Repeat("C", 55)

	for _, c := range []store.Contract{
		{ID: contractA, Network: "testnet", Label: "A", Status: "active", AddedAt: at(1)},
		{ID: contractB, Network: "testnet", Label: "B", Status: "active", AddedAt: at(2)},
		{ID: contractC, Network: "testnet", Label: "C", Status: "active", AddedAt: at(3)},
	} {
		if err := s.UpsertContract(ctx, c); err != nil {
			t.Fatalf("upsert contract %s: %v", c.ID, err)
		}
	}

	if err := s.BatchInsertEvents(ctx, []store.Event{
		{ID: "ev-a1", ContractID: contractA, Network: "testnet", Ledger: 10, LedgerClosedAt: at(4), TxHash: "txa1", Type: "contract"},
		{ID: "ev-b1", ContractID: contractB, Network: "testnet", Ledger: 20, LedgerClosedAt: at(5), TxHash: "txb1", Type: "contract"},
		{ID: "ev-b2", ContractID: contractB, Network: "testnet", Ledger: 21, LedgerClosedAt: at(6), TxHash: "txb2", Type: "contract"},
		{ID: "ev-b3", ContractID: contractB, Network: "testnet", Ledger: 22, LedgerClosedAt: at(7), TxHash: "txb3", Type: "contract"},
	}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	if err := s.BatchInsertInvocations(ctx, []store.Invocation{
		{TxHash: "inv-b1", ContractID: contractB, Network: "testnet", Ledger: 30, LedgerClosedAt: at(8), Status: "SUCCESS"},
	}); err != nil {
		t.Fatalf("insert invocations: %v", err)
	}

	cases := []struct {
		sort, order string
		want        []string
	}{
		{store.ContractSortAddedAt, store.SortAsc, []string{contractA, contractB, contractC}},
		{store.ContractSortAddedAt, store.SortDesc, []string{contractC, contractB, contractA}},
		{store.ContractSortLastActivity, store.SortAsc, []string{contractC, contractA, contractB}},
		{store.ContractSortLastActivity, store.SortDesc, []string{contractB, contractA, contractC}},
		{store.ContractSortEventsCount, store.SortAsc, []string{contractC, contractA, contractB}},
		{store.ContractSortEventsCount, store.SortDesc, []string{contractB, contractA, contractC}},
	}
	for _, tc := range cases {
		got, _, err := s.ListContracts(ctx, "", 50, store.ContractFilters{Sort: tc.sort, Order: tc.order})
		if err != nil {
			t.Fatalf("ListContracts(%s, %s): %v", tc.sort, tc.order, err)
		}
		ids := make([]string, len(got))
		for i, c := range got {
			ids[i] = c.ID
		}
		if strings.Join(ids, ",") != strings.Join(tc.want, ",") {
			t.Errorf("ListContracts(%s, %s): want %v, got %v", tc.sort, tc.order, tc.want, ids)
		}
	}

	// Walk a descending events_count list one row at a time through the cursor.
	wantPages := [][]string{{contractB}, {contractA}, {contractC}}
	cursor := ""
	for i, want := range wantPages {
		page, next, err := s.ListContracts(ctx, cursor, 1, store.ContractFilters{
			Sort:  store.ContractSortEventsCount,
			Order: store.SortDesc,
		})
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		if len(page) != 1 || page[0].ID != want[0] {
			t.Fatalf("page %d: want %v, got %+v", i, want, page)
		}
		cursor = next
	}
}
