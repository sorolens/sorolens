package pgstore

import (
	"context"
	"testing"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

// TestBatchInsertWithCursor_StoresFailureCategory checks the issue #806
// columns: a failed invocation keeps its category and raw code, and a
// successful one stores NULL for both.
func TestBatchInsertWithCursor_StoresFailureCategory(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	id := seedContracts(t, s, uniquePrefix(t), "a")[0]
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

	invs := []poller.Invocation{
		{
			TxHash: id + "-failed", ContractID: id, Network: "testnet", Ledger: 5, LedgerClosedAt: at, Status: "FAILED",
			FailureCategory: "budget_exceeded", FailureCode: "Error(Budget, ExceededLimit)",
		},
		{TxHash: id + "-ok", ContractID: id, Network: "testnet", Ledger: 6, LedgerClosedAt: at, Status: "SUCCESS"},
	}
	if _, err := s.BatchInsertWithCursor(ctx, "testnet", 6, nil, invs, poller.SyncState{ContractID: id, LastLedger: 6}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var cat, code *string
	row := `SELECT failure_category, failure_code FROM invocations WHERE tx_hash = $1`
	if err := s.pool.QueryRow(ctx, row, id+"-failed").Scan(&cat, &code); err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if cat == nil || *cat != "budget_exceeded" || code == nil || *code != "Error(Budget, ExceededLimit)" {
		t.Fatalf("failed row: category=%v code=%v", cat, code)
	}
	if err := s.pool.QueryRow(ctx, row, id+"-ok").Scan(&cat, &code); err != nil {
		t.Fatalf("read ok: %v", err)
	}
	if cat != nil || code != nil {
		t.Fatalf("successful invocation must store NULL: category=%v code=%v", cat, code)
	}
}
