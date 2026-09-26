package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func TestPerformanceBaselinesAndRegressions(t *testing.T) {
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

	// Ensure tables are clean for this test
	_, _ = pool.Exec(ctx, "TRUNCATE TABLE contracts, monitored_contracts, invocations, performance_baselines, contract_alerts CASCADE")

	s := store.NewFullStore(pool)

	// Create a test contract
	err = s.UpsertContract(ctx, store.Contract{
		ID:      "C_TEST_REGRESSION",
		Network: "testnet",
		Status:  "active",
	})
	if err != nil {
		t.Fatalf("UpsertContract: %v", err)
	}

	// Monitored contract so alerts can be inserted
	// We need to write to monitored_contracts directly or via WatchdogStore if available.
	// We'll just insert it directly for test setup to keep it simple.
	_, err = pool.Exec(ctx, "INSERT INTO monitored_contracts (contract_id, name, owner, registered_at) VALUES ($1, 'test', 'test', NOW())", "C_TEST_REGRESSION")
	if err != nil {
		t.Fatalf("Insert monitored: %v", err)
	}

	snapshotDate := time.Date(2023, 10, 8, 0, 0, 0, 0, time.UTC)

	// Insert old invocations (before snapshotDate) for baseline
	// 7 days before snapshotDate is Oct 1 to Oct 7.
	oldInvs := []store.Invocation{
		{TxHash: "tx1", ContractID: "C_TEST_REGRESSION", Ledger: 1, LedgerClosedAt: time.Date(2023, 10, 5, 0, 0, 0, 0, time.UTC), Status: "SUCCESS", FunctionName: "my_func", CPUInsn: 100, MemByte: 200, ResourceFeeCharged: 10},
		{TxHash: "tx2", ContractID: "C_TEST_REGRESSION", Ledger: 2, LedgerClosedAt: time.Date(2023, 10, 6, 0, 0, 0, 0, time.UTC), Status: "SUCCESS", FunctionName: "my_func", CPUInsn: 150, MemByte: 200, ResourceFeeCharged: 10},
	}
	err = s.BatchInsertInvocations(ctx, oldInvs)
	if err != nil {
		t.Fatalf("BatchInsertInvocations old: %v", err)
	}

	// Compute baseline for Oct 8
	err = s.ComputeAndStoreBaselines(ctx, snapshotDate)
	if err != nil {
		t.Fatalf("ComputeAndStoreBaselines: %v", err)
	}

	// Now insert new invocations for the current 7d window (Oct 8 to Oct 14).
	// We will snapshot at Oct 15.
	// Average before was CPU=125, Mem=200, Fee=10.
	// Let's create a 25% regression on CPU. New avg > 125 * 1.25 = 156.25.
	newSnapshotDate := time.Date(2023, 10, 15, 0, 0, 0, 0, time.UTC)
	newInvs := []store.Invocation{
		{TxHash: "tx3", ContractID: "C_TEST_REGRESSION", Ledger: 3, LedgerClosedAt: time.Date(2023, 10, 12, 0, 0, 0, 0, time.UTC), Status: "SUCCESS", FunctionName: "my_func", CPUInsn: 200, MemByte: 200, ResourceFeeCharged: 10},
	}
	err = s.BatchInsertInvocations(ctx, newInvs)
	if err != nil {
		t.Fatalf("BatchInsertInvocations new: %v", err)
	}

	// Check regressions using new snapshot date
	alertsCount, err := s.CheckAndEmitRegressions(ctx, newSnapshotDate)
	if err != nil {
		t.Fatalf("CheckAndEmitRegressions: %v", err)
	}

	if alertsCount != 1 {
		t.Errorf("Expected 1 alert, got %d", alertsCount)
	}

	// Check idempotency of baselines
	err = s.ComputeAndStoreBaselines(ctx, snapshotDate)
	if err != nil {
		t.Fatalf("Idempotent ComputeAndStoreBaselines failed: %v", err)
	}
}
