package pgstore

import (
	"context"
	"testing"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

func int64p(v int64) *int64 { return &v }

// TestBatchInsertWithCursor_StoresDiagnosticColumns checks the issue #504
// columns round-trip, and that unreported values are stored as NULL.
func TestBatchInsertWithCursor_StoresDiagnosticColumns(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	id := seedContracts(t, s, uniquePrefix(t), "a")[0]
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

	invs := []poller.Invocation{
		{
			TxHash: id + "-full", ContractID: id, Network: "testnet", Ledger: 5, LedgerClosedAt: at, Status: "SUCCESS",
			FunctionName: "transfer", CPUInsn: int64p(2837152), MemByte: int64p(1209434),
			LedgerReadByte: int64p(1324), LedgerWriteByte: int64p(412),
		},
		{TxHash: id + "-bare", ContractID: id, Network: "testnet", Ledger: 6, LedgerClosedAt: at, Status: "SUCCESS"},
	}
	if _, err := s.BatchInsertWithCursor(ctx, "testnet", 6, nil, invs, poller.SyncState{ContractID: id, LastLedger: 6}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var (
		fn               *string
		cpu, mem, rb, wb *int64
	)
	row := `SELECT function_name, cpu_insn, mem_byte, ledger_read_byte, ledger_write_byte FROM invocations WHERE tx_hash = $1`
	if err := s.pool.QueryRow(ctx, row, id+"-full").Scan(&fn, &cpu, &mem, &rb, &wb); err != nil {
		t.Fatalf("read full: %v", err)
	}
	if fn == nil || *fn != "transfer" || *cpu != 2837152 || *mem != 1209434 || *rb != 1324 || *wb != 412 {
		t.Fatalf("full row: fn=%v cpu=%v mem=%v rb=%v wb=%v", fn, cpu, mem, rb, wb)
	}
	if err := s.pool.QueryRow(ctx, row, id+"-bare").Scan(&fn, &cpu, &mem, &rb, &wb); err != nil {
		t.Fatalf("read bare: %v", err)
	}
	if fn != nil || cpu != nil || mem != nil || rb != nil || wb != nil {
		t.Fatalf("unreported diagnostics must be NULL: fn=%v cpu=%v mem=%v rb=%v wb=%v", fn, cpu, mem, rb, wb)
	}
}
