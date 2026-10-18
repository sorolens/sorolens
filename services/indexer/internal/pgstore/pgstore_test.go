package pgstore

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

// testStore connects to POSTGRES_TEST_URL, which must already have the
// apps/api migrations applied (CI does this). Tests are skipped without it.
func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_URL")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_URL not set, skipping DB test")
	}
	ctx := context.Background()
	s, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(s.Close)

	var reg *string
	if err := s.pool.QueryRow(ctx, `SELECT to_regclass('public.sync_state')::text`).Scan(&reg); err != nil {
		t.Fatalf("schema check: %v", err)
	}
	if reg == nil {
		t.Skip("schema not migrated (apply apps/api/internal/db/migrations/*.up.sql), skipping DB test")
	}
	return s
}

// seedContracts inserts contracts with ids "<prefix><suffix>" and removes them
// (and their indexed rows) when the test ends.
func seedContracts(t *testing.T, s *Store, prefix string, suffixes ...string) []string {
	t.Helper()
	ctx := context.Background()
	ids := make([]string, len(suffixes))
	for i, sfx := range suffixes {
		ids[i] = prefix + sfx
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO contracts (id, network, status, wasm_hash) VALUES ($1, 'testnet', 'active', $2)`,
			ids[i], "hash-"+sfx); err != nil {
			t.Fatalf("seed contract %s: %v", ids[i], err)
		}
	}
	t.Cleanup(func() {
		for _, table := range []string{"events", "invocations", "sync_state"} {
			s.pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE contract_id LIKE $1`, table), prefix+"%") //nolint:errcheck
		}
		s.pool.Exec(ctx, `DELETE FROM contracts WHERE id LIKE $1`, prefix+"%") //nolint:errcheck
	})
	return ids
}

// uniquePrefix sorts after any real contract id ("C...") so seeded rows form
// one contiguous run in id order.
func uniquePrefix(t *testing.T) string {
	return fmt.Sprintf("ZZPGSTORE_%s_%d_", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
}

func TestConnect_FailsFastWhenUnreachable(t *testing.T) {
	start := time.Now()
	_, err := Connect(context.Background(), "postgres://u:p@127.0.0.1:1/none?sslmode=disable&connect_timeout=2")
	if err == nil {
		t.Fatal("want error for unreachable database")
	}
	if !strings.Contains(err.Error(), "database unreachable") {
		t.Fatalf("error should say the database is unreachable, got %v", err)
	}
	if time.Since(start) > 15*time.Second {
		t.Fatalf("Connect took %v; startup check must be bounded", time.Since(start))
	}

	if _, err := Connect(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("empty DSN: want DATABASE_URL error, got %v", err)
	}
}

func TestListContracts_CursorPagination(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	prefix := uniquePrefix(t)
	ids := seedContracts(t, s, prefix, "a", "b", "c", "d", "e")

	var got []poller.Contract
	cursor := prefix // every seeded id sorts after the bare prefix
	pages := 0
	for {
		page, next, err := s.ListContracts(ctx, cursor, 2)
		if err != nil {
			t.Fatalf("ListContracts: %v", err)
		}
		pages++
		if len(page) > 2 {
			t.Fatalf("page of %d rows exceeds limit 2", len(page))
		}
		for _, c := range page {
			if strings.HasPrefix(c.ID, prefix) {
				got = append(got, c)
			}
		}
		if next == "" || len(got) == len(ids) {
			break
		}
		if next != page[len(page)-1].ID {
			t.Fatalf("cursor %q should be the last id of the page %q", next, page[len(page)-1].ID)
		}
		cursor = next
	}

	if len(got) != len(ids) {
		t.Fatalf("got %d seeded contracts over %d pages, want %d", len(got), pages, len(ids))
	}
	for i, c := range got {
		if c.ID != ids[i] {
			t.Errorf("row %d: id %s, want %s (ascending id order)", i, c.ID, ids[i])
		}
		if c.Status != "active" || c.Network != "testnet" || c.WasmHash != "hash-"+strings.TrimPrefix(ids[i], prefix) {
			t.Errorf("row %d: %+v", i, c)
		}
	}
	if pages < 3 {
		t.Errorf("5 rows at limit 2 should need at least 3 pages, took %d", pages)
	}
}

func TestBatchInserts_AreIdempotent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	prefix := uniquePrefix(t)
	ids := seedContracts(t, s, prefix, "x")
	cid := ids[0]
	closedAt := time.Now().UTC().Truncate(time.Second)

	events := []poller.Event{
		{ID: prefix + "ev1", ContractID: cid, Network: "testnet", Ledger: 10, LedgerClosedAt: closedAt,
			TxHash: prefix + "tx1", Type: "contract", TopicXDR: []string{"AAAADw=="}, ValueXDR: "AAAAAQ==", InSuccessfulCall: true},
		{ID: prefix + "ev2", ContractID: cid, Ledger: 11, LedgerClosedAt: closedAt,
			TxHash: prefix + "tx2", Type: "contract", ValueXDR: "AAAAAQ=="},
	}
	invocations := []poller.Invocation{
		{TxHash: prefix + "tx1", ContractID: cid, Network: "testnet", Ledger: 10, LedgerClosedAt: closedAt,
			Status: "SUCCESS", ResultXDR: "AAAA", ApplicationOrder: 1},
		{TxHash: prefix + "tx2", ContractID: cid, Ledger: 11, LedgerClosedAt: closedAt, Status: "FAILED"},
	}

	for round := 1; round <= 2; round++ {
		if err := s.BatchInsertEvents(ctx, events); err != nil {
			t.Fatalf("round %d: BatchInsertEvents: %v", round, err)
		}
		if err := s.BatchInsertInvocations(ctx, invocations); err != nil {
			t.Fatalf("round %d: BatchInsertInvocations: %v", round, err)
		}
	}

	var nEvents, nInv int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM events WHERE contract_id = $1`, cid).Scan(&nEvents); err != nil {
		t.Fatal(err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM invocations WHERE contract_id = $1`, cid).Scan(&nInv); err != nil {
		t.Fatal(err)
	}
	if nEvents != 2 || nInv != 2 {
		t.Fatalf("after inserting the same batch twice: %d events, %d invocations; want 2 and 2", nEvents, nInv)
	}

	// Columns land where the API reads them; an empty network defaults to testnet.
	var network, valueXDR string
	var ledger int64
	if err := s.pool.QueryRow(ctx, `SELECT network, ledger, value_xdr FROM events WHERE id = $1`, prefix+"ev2").
		Scan(&network, &ledger, &valueXDR); err != nil {
		t.Fatal(err)
	}
	if network != "testnet" || ledger != 11 || valueXDR != "AAAAAQ==" {
		t.Fatalf("event row: network=%s ledger=%d value=%s", network, ledger, valueXDR)
	}

	// Empty batches are a no-op.
	if err := s.BatchInsertEvents(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.BatchInsertInvocations(ctx, nil); err != nil {
		t.Fatal(err)
	}
}

func TestSyncState_RoundTrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	prefix := uniquePrefix(t)
	cid := seedContracts(t, s, prefix, "s")[0]

	st, err := s.GetSyncState(ctx, cid)
	if err != nil {
		t.Fatalf("GetSyncState (new): %v", err)
	}
	if st.ContractID != cid || st.LastLedger != 0 {
		t.Fatalf("new contract sync state = %+v, want zero ledger", st)
	}

	for _, ledger := range []uint32{100, 250} {
		if err := s.UpsertSyncState(ctx, poller.SyncState{ContractID: cid, LastLedger: ledger}); err != nil {
			t.Fatalf("UpsertSyncState(%d): %v", ledger, err)
		}
		st, err = s.GetSyncState(ctx, cid)
		if err != nil {
			t.Fatalf("GetSyncState: %v", err)
		}
		if st.LastLedger != ledger {
			t.Fatalf("LastLedger = %d, want %d", st.LastLedger, ledger)
		}
	}

	var lastRun *time.Time
	if err := s.pool.QueryRow(ctx, `SELECT last_run_at FROM sync_state WHERE contract_id = $1`, cid).Scan(&lastRun); err != nil {
		t.Fatal(err)
	}
	if lastRun == nil {
		t.Fatal("last_run_at should be stamped on upsert")
	}
}

func TestIndexerCursor_NeverMovesBackwards(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	network := uniquePrefix(t)
	t.Cleanup(func() {
		s.pool.Exec(ctx, `DELETE FROM indexer_cursors WHERE network = $1`, network) //nolint:errcheck
	})

	if got, err := s.GetIndexerCursor(ctx, network); err != nil || got != 0 {
		t.Fatalf("unset cursor = %d, %v; want 0, nil", got, err)
	}
	for _, step := range []struct{ set, want uint32 }{{500, 500}, {400, 500}, {700, 700}} {
		if err := s.SetIndexerCursor(ctx, network, step.set); err != nil {
			t.Fatalf("SetIndexerCursor(%d): %v", step.set, err)
		}
		got, err := s.GetIndexerCursor(ctx, network)
		if err != nil || got != step.want {
			t.Fatalf("after set %d: cursor = %d, %v; want %d", step.set, got, err, step.want)
		}
	}
}

func TestUpdateContractWasmHash(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	prefix := uniquePrefix(t)
	cid := seedContracts(t, s, prefix, "w")[0]

	if err := s.UpdateContractWasmHash(ctx, cid, "newhash"); err != nil {
		t.Fatalf("UpdateContractWasmHash: %v", err)
	}
	page, _, err := s.ListContracts(ctx, prefix, 1)
	if err != nil || len(page) != 1 || page[0].WasmHash != "newhash" {
		t.Fatalf("after update: %+v, %v", page, err)
	}
	if err := s.UpdateContractWasmHash(ctx, prefix+"missing", "h"); err == nil {
		t.Fatal("updating an unknown contract should fail")
	}
}
