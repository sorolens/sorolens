package store_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestHealthScoreStorePostgres verifies the four input aggregations (watchdog
// history, 24h error rate, hourly activity, storage TTL headroom) and the
// cached score upsert/read path against real SQL.
func TestHealthScoreStorePostgres(t *testing.T) {
	pool := pgPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `TRUNCATE TABLE contracts, monitored_contracts,
		invocations, storage_entries, sync_state, events CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	s := store.NewFullStore(pool)

	const contractID = "CHS-CONTRACT"
	if err := s.UpsertContract(ctx, store.Contract{ID: contractID, Network: "testnet", Status: "active"}); err != nil {
		t.Fatalf("UpsertContract: %v", err)
	}
	if err := s.UpsertMonitoredContract(ctx, store.MonitoredContract{
		ContractID: contractID, Network: "testnet", Name: "scored", Owner: "GOWNER",
		Status: "Degraded", RegisteredAt: time.Now().UTC().Add(-30 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("UpsertMonitoredContract: %v", err)
	}

	// 105 checks: the query must count only the 100 newest, all Healthy, and
	// ignore the 5 older Unresponsive ones.
	now := time.Now().UTC()
	for i := 0; i < 105; i++ {
		status := "Healthy"
		if i >= 100 {
			status = "Unresponsive"
		}
		if err := s.InsertHealthCheck(ctx, store.HealthCheck{
			ContractID: contractID, Status: status, Ledger: int64(i),
			TxHash: fmt.Sprintf("hscore-check-%d", i), Timestamp: now.Add(-time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatalf("InsertHealthCheck(%d): %v", i, err)
		}
	}

	// Error rate covers the trailing 24 hours only: three recent invocations
	// (one failed) and one failed invocation two days ago.
	invocations := []store.Invocation{
		{TxHash: "hscore-inv-1", ContractID: contractID, Network: "testnet", Ledger: 1,
			LedgerClosedAt: now.Add(-2 * time.Hour), Status: "SUCCESS", FunctionName: "f", CPUInsn: 100, ResourceFeeCharged: 10},
		{TxHash: "hscore-inv-2", ContractID: contractID, Network: "testnet", Ledger: 2,
			LedgerClosedAt: now.Add(-90 * time.Minute), Status: "SUCCESS", FunctionName: "f", CPUInsn: 200, ResourceFeeCharged: 20},
		{TxHash: "hscore-inv-3", ContractID: contractID, Network: "testnet", Ledger: 3,
			LedgerClosedAt: now.Add(-30 * time.Minute), Status: "FAILED", FunctionName: "f"},
		{TxHash: "hscore-inv-old", ContractID: contractID, Network: "testnet", Ledger: 4,
			LedgerClosedAt: now.Add(-48 * time.Hour), Status: "FAILED", FunctionName: "f"},
	}
	if err := s.BatchInsertInvocations(ctx, invocations); err != nil {
		t.Fatalf("BatchInsertInvocations: %v", err)
	}
	if err := s.BatchInsertEvents(ctx, []store.Event{{
		ID: "hscore-event-1", ContractID: contractID, Network: "testnet", Ledger: 3,
		LedgerClosedAt: now.Add(-30 * time.Minute), TxHash: "hscore-inv-3", Type: "contract",
	}}); err != nil {
		t.Fatalf("BatchInsertEvents: %v", err)
	}

	// TTL headroom compares live_until_ledger to last_ledger + 17280.
	if err := s.UpsertSyncState(ctx, store.SyncState{ContractID: contractID, LastLedger: 1000}); err != nil {
		t.Fatalf("UpsertSyncState: %v", err)
	}
	entries := []store.StorageEntry{
		{ContractID: contractID, Network: "testnet", KeyXDR: "k-expiring", Durability: "temporary",
			LiveUntilLedger: 500, LastModifiedLedger: 10, Status: "live"},
		{ContractID: contractID, Network: "testnet", KeyXDR: "k-healthy", Durability: "persistent",
			LiveUntilLedger: 20000, LastModifiedLedger: 10, Status: "live"},
		{ContractID: contractID, Network: "testnet", KeyXDR: "k-at-horizon", Durability: "temporary",
			LiveUntilLedger: 1000 + 17280, LastModifiedLedger: 10, Status: "live"},
		{ContractID: contractID, Network: "testnet", KeyXDR: "k-archived", Durability: "temporary",
			LiveUntilLedger: 100, LastModifiedLedger: 10, Status: "archived"},
	}
	if err := s.UpsertStorageEntries(ctx, entries); err != nil {
		t.Fatalf("UpsertStorageEntries: %v", err)
	}

	inputs, err := s.ContractHealthInputs(ctx, contractID)
	if err != nil {
		t.Fatalf("ContractHealthInputs: %v", err)
	}
	if inputs.HealthyChecks != 100 || inputs.TotalChecks != 100 {
		t.Errorf("checks = %d/%d, want 100/100 (last 100 window)", inputs.HealthyChecks, inputs.TotalChecks)
	}
	if inputs.WatchdogStatus != "Degraded" {
		t.Errorf("watchdog status = %q, want Degraded", inputs.WatchdogStatus)
	}
	if inputs.TotalInvocations != 3 || inputs.FailedInvocations != 1 {
		t.Errorf("invocations = %d/%d failed, want 3/1 (trailing 24h)", inputs.TotalInvocations, inputs.FailedInvocations)
	}
	if inputs.TotalStorage != 3 || inputs.ExpiringStorage != 2 {
		t.Errorf("storage = %d/%d expiring, want 3 live/2 expiring", inputs.TotalStorage, inputs.ExpiringStorage)
	}
	if len(inputs.Activity) < 24 {
		t.Fatalf("activity has %d buckets, want at least 24", len(inputs.Activity))
	}
	var events, invoke int64
	for _, bucket := range inputs.Activity {
		events += bucket.EventCount
		invoke += bucket.InvokeCount
	}
	if events != 1 || invoke != 3 {
		t.Errorf("activity totals = %d events / %d invocations, want 1/3", events, invoke)
	}

	// A contract with no signals still yields a zero-value input struct.
	empty, err := s.ContractHealthInputs(ctx, "CHS-NOTHING")
	if err != nil {
		t.Fatalf("ContractHealthInputs(empty): %v", err)
	}
	if empty.HealthyChecks != 0 || empty.TotalChecks != 0 || empty.WatchdogStatus != "" ||
		empty.TotalInvocations != 0 || empty.FailedInvocations != 0 ||
		empty.TotalStorage != 0 || empty.ExpiringStorage != 0 {
		t.Errorf("empty inputs = %+v", empty)
	}

	computedAt := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	score := store.ContractHealthScore{
		ContractID: contractID, Score: 82, ComponentUptime: 90, ComponentErrorRate: 70,
		ComponentPerformance: 88, ComponentStorageTTL: 80, ComputedAt: computedAt,
	}
	if err := s.UpsertContractHealthScore(ctx, score); err != nil {
		t.Fatalf("UpsertContractHealthScore: %v", err)
	}
	stored, err := s.GetContractHealthScore(ctx, contractID)
	if err != nil {
		t.Fatalf("GetContractHealthScore: %v", err)
	}
	if stored.Score != 82 || stored.ComponentUptime != 90 || stored.ComponentErrorRate != 70 ||
		stored.ComponentPerformance != 88 || stored.ComponentStorageTTL != 80 ||
		!stored.ComputedAt.Equal(computedAt) {
		t.Errorf("cached score = %+v", stored)
	}

	// The upsert replaces an existing cached value.
	score.Score, score.ComputedAt = 64, computedAt.Add(time.Hour)
	if err := s.UpsertContractHealthScore(ctx, score); err != nil {
		t.Fatalf("second UpsertContractHealthScore: %v", err)
	}
	stored, err = s.GetContractHealthScore(ctx, contractID)
	if err != nil {
		t.Fatalf("GetContractHealthScore after upsert: %v", err)
	}
	if stored.Score != 64 || !stored.ComputedAt.Equal(score.ComputedAt) {
		t.Errorf("cached score after upsert = %+v", stored)
	}
	if _, err := s.GetContractHealthScore(ctx, "CHS-NOTHING"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetContractHealthScore(unknown): want ErrNotFound, got %v", err)
	}
}
