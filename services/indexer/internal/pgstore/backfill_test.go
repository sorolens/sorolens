package pgstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

func contractState(t *testing.T, s *Store, id string) (status string, completed bool) {
	t.Helper()
	var at *time.Time
	if err := s.pool.QueryRow(context.Background(),
		`SELECT status, backfill_complete_at FROM contracts WHERE id = $1`, id,
	).Scan(&status, &at); err != nil {
		t.Fatalf("read contract: %v", err)
	}
	return status, at != nil
}

func TestBackfill_LifecyclePersistsAndResumes(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	prefix := uniquePrefix(t)
	cid := seedContracts(t, s, prefix, "a")[0]
	if _, err := s.pool.Exec(ctx, `UPDATE contracts SET status = 'pending' WHERE id = $1`, cid); err != nil {
		t.Fatal(err)
	}

	start := poller.BackfillProgress{ContractID: cid, OldestLedger: 100, NewestLedger: 300, NextLedger: 300}
	got, err := s.BeginBackfill(ctx, start)
	if err != nil {
		t.Fatalf("BeginBackfill: %v", err)
	}
	if got != start {
		t.Fatalf("position: want %+v, got %+v", start, got)
	}
	if status, _ := contractState(t, s, cid); status != "backfilling" {
		t.Fatalf("status: want backfilling, got %q", status)
	}
	sync, err := s.GetSyncState(ctx, cid)
	if err != nil || sync.LastLedger != 300 {
		t.Fatalf("live cursor: want 300, got %+v (err %v)", sync, err)
	}

	closedAt := time.Now().UTC().Truncate(time.Second)
	mid := poller.BackfillProgress{ContractID: cid, OldestLedger: 100, NewestLedger: 300, NextLedger: 199}
	if _, err := s.CommitBackfillWindow(ctx, poller.BackfillWindow{
		Progress: mid,
		Events: []poller.Event{{ID: prefix + "ev1", ContractID: cid, Network: "testnet", Ledger: 250,
			LedgerClosedAt: closedAt, TxHash: prefix + "tx1", Type: "contract", ValueXDR: "AAAAAQ=="}},
		Invocations: []poller.Invocation{{TxHash: prefix + "tx1", ContractID: cid, Network: "testnet",
			Ledger: 250, LedgerClosedAt: closedAt, Status: "SUCCESS"}},
	}); err != nil {
		t.Fatalf("CommitBackfillWindow: %v", err)
	}

	// A restart asks to begin again with a newer head: the saved range wins.
	resumed, err := s.BeginBackfill(ctx, poller.BackfillProgress{ContractID: cid, OldestLedger: 500, NewestLedger: 900, NextLedger: 900})
	if err != nil {
		t.Fatalf("BeginBackfill (resume): %v", err)
	}
	if resumed != mid {
		t.Fatalf("resume: want %+v, got %+v", mid, resumed)
	}
	sync, err = s.GetSyncState(ctx, cid)
	if err != nil || sync.LastLedger != 300 {
		t.Fatalf("resume must not rewind the live cursor: %+v (err %v)", sync, err)
	}

	done := poller.BackfillProgress{ContractID: cid, OldestLedger: 100, NewestLedger: 300, NextLedger: 99}
	if _, err := s.CommitBackfillWindow(ctx, poller.BackfillWindow{Progress: done, Complete: true}); err != nil {
		t.Fatalf("CommitBackfillWindow (final): %v", err)
	}
	status, completed := contractState(t, s, cid)
	if status != "active" || !completed {
		t.Fatalf("after completion: status %q, backfill_complete_at set %v", status, completed)
	}
	var finished *time.Time
	if err := s.pool.QueryRow(ctx, `SELECT completed_at FROM contract_backfills WHERE contract_id = $1`, cid).Scan(&finished); err != nil {
		t.Fatal(err)
	}
	if finished == nil {
		t.Error("completed_at should be set")
	}
}

func TestBackfill_WindowRollsBackAsAUnit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	prefix := uniquePrefix(t)
	cid := seedContracts(t, s, prefix, "a")[0]

	start := poller.BackfillProgress{ContractID: cid, OldestLedger: 1, NewestLedger: 100, NextLedger: 100}
	if _, err := s.BeginBackfill(ctx, start); err != nil {
		t.Fatalf("BeginBackfill: %v", err)
	}

	// An invocation for an untracked contract violates the foreign key.
	_, err := s.CommitBackfillWindow(ctx, poller.BackfillWindow{
		Progress: poller.BackfillProgress{ContractID: cid, OldestLedger: 1, NewestLedger: 100, NextLedger: 50},
		Invocations: []poller.Invocation{{TxHash: prefix + "bad", ContractID: prefix + "missing",
			Network: "testnet", Ledger: 60, LedgerClosedAt: time.Now().UTC(), Status: "SUCCESS"}},
	})
	if err == nil {
		t.Fatal("want an error for the invalid invocation")
	}

	again, err := s.BeginBackfill(ctx, start)
	if err != nil {
		t.Fatalf("BeginBackfill: %v", err)
	}
	if again.NextLedger != 100 {
		t.Fatalf("position moved despite the rollback: %+v", again)
	}
}

func TestBackfill_PausedContractStaysPaused(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	prefix := uniquePrefix(t)
	cid := seedContracts(t, s, prefix, "a")[0]

	if _, err := s.BeginBackfill(ctx, poller.BackfillProgress{ContractID: cid, OldestLedger: 1, NewestLedger: 10, NextLedger: 10}); err != nil {
		t.Fatalf("BeginBackfill: %v", err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE contracts SET status = 'paused' WHERE id = $1`, cid); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CommitBackfillWindow(ctx, poller.BackfillWindow{
		Progress: poller.BackfillProgress{ContractID: cid, OldestLedger: 1, NewestLedger: 10, NextLedger: 0},
		Complete: true,
	}); err != nil {
		t.Fatalf("CommitBackfillWindow: %v", err)
	}
	if status, _ := contractState(t, s, cid); status != "paused" {
		t.Fatalf("status: want paused, got %q", status)
	}
}

func TestCommitHistoryBatch_IsIdempotentAndMovesNoCursor(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	prefix := uniquePrefix(t)
	cid := seedContracts(t, s, prefix, "a")[0]

	closedAt := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	if err := s.CreateMonthlyPartitionIfNotExists(ctx, 2024, 3); err != nil {
		t.Fatalf("create partition: %v", err)
	}
	events := []poller.Event{{ID: prefix + "ev1", ContractID: cid, Network: "mainnet", Ledger: 51_000_000,
		LedgerClosedAt: closedAt, TxHash: prefix + "tx1", Type: "contract",
		TopicXDR: []string{"AAAADwAAAAh0cmFuc2Zlcg=="}, ValueXDR: "AAAACgAAAAAAAAAAAAAAAAAAA+g=", InSuccessfulCall: true}}
	invocations := []poller.Invocation{{TxHash: prefix + "tx1", ContractID: cid, Network: "mainnet",
		Ledger: 51_000_000, LedgerClosedAt: closedAt, Status: "SUCCESS", ResultXDR: "AAAA", ApplicationOrder: 3}}

	for run := 0; run < 2; run++ {
		parked, err := s.CommitHistoryBatch(ctx, events, invocations)
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if len(parked) != 0 {
			t.Fatalf("run %d: parked %+v", run, parked)
		}
	}

	var eventRows, invocationRows int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM events WHERE contract_id = $1`, cid).Scan(&eventRows); err != nil {
		t.Fatal(err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM invocations WHERE contract_id = $1`, cid).Scan(&invocationRows); err != nil {
		t.Fatal(err)
	}
	if eventRows != 1 || invocationRows != 1 {
		t.Fatalf("re-run duplicated rows: events %d, invocations %d", eventRows, invocationRows)
	}

	// Decoded columns are filled exactly as they are for live events.
	var decoded *string
	if err := s.pool.QueryRow(ctx, `SELECT topic_decoded::text FROM events WHERE id = $1`, prefix+"ev1").Scan(&decoded); err != nil {
		t.Fatal(err)
	}
	if decoded == nil || *decoded != `["transfer"]` {
		t.Fatalf("topic_decoded: got %v", decoded)
	}

	sync, err := s.GetSyncState(ctx, cid)
	if err != nil {
		t.Fatal(err)
	}
	if sync.LastLedger != 0 {
		t.Fatalf("an import must not move the live cursor: %+v", sync)
	}
}

func TestImportTarget(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	prefix := uniquePrefix(t)
	cid := seedContracts(t, s, prefix, "a")[0]

	got, err := s.ImportTarget(ctx, cid)
	if err != nil {
		t.Fatalf("ImportTarget: %v", err)
	}
	if got.Network != "testnet" || got.RPCFloorLedger != 0 {
		t.Fatalf("no backfill yet: %+v", got)
	}

	if _, err := s.BeginBackfill(ctx, poller.BackfillProgress{ContractID: cid, OldestLedger: 700, NewestLedger: 900, NextLedger: 900}); err != nil {
		t.Fatalf("BeginBackfill: %v", err)
	}
	got, err = s.ImportTarget(ctx, cid)
	if err != nil {
		t.Fatalf("ImportTarget: %v", err)
	}
	if got.RPCFloorLedger != 700 {
		t.Fatalf("floor: want 700, got %d", got.RPCFloorLedger)
	}

	if _, err := s.ImportTarget(ctx, prefix+"missing"); !errors.Is(err, ErrContractNotTracked) {
		t.Fatalf("missing contract: got %v", err)
	}
}
