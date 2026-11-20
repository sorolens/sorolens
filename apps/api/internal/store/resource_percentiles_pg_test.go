package store_test

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestGetContractResourcePercentilesPostgres checks the SQL against a fixture
// whose percentiles are known: 1..101 interpolates to exact integers.
func TestGetContractResourcePercentilesPostgres(t *testing.T) {
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

	const contractID = "C_TEST_RESOURCE_PERCENTILES"
	cleanup := func() {
		_, _ = pool.Exec(ctx, "DELETE FROM invocations WHERE contract_id = $1", contractID)
		_, _ = pool.Exec(ctx, "DELETE FROM contracts WHERE id = $1", contractID)
	}
	cleanup()
	t.Cleanup(cleanup)

	s := store.NewFullStore(pool)
	if err := s.UpsertContract(ctx, store.Contract{ID: contractID, Network: "testnet", Status: "active"}); err != nil {
		t.Fatalf("UpsertContract: %v", err)
	}

	empty, err := s.GetContractResourcePercentiles(ctx, contractID, "24h")
	if err != nil {
		t.Fatalf("empty window: %v", err)
	}
	if empty != (store.ResourcePercentiles{}) {
		t.Fatalf("empty window = %+v, want zero values", empty)
	}

	now := time.Now().UTC()
	invocations := make([]store.Invocation, 0, 102)
	for i := int64(1); i <= 101; i++ {
		invocations = append(invocations, store.Invocation{
			TxHash:             fmt.Sprintf("percentile-tx-%d", i),
			ContractID:         contractID,
			Ledger:             uint32(i),
			LedgerClosedAt:     now.Add(-time.Hour),
			Status:             "SUCCESS",
			CPUInsn:            i * 100,
			MemByte:            i * 10,
			ResourceFeeCharged: i,
		})
	}
	// Outside the 24h window, inside 7d: only the wider window may see it.
	invocations = append(invocations, store.Invocation{
		TxHash:             "percentile-tx-old",
		ContractID:         contractID,
		Ledger:             102,
		LedgerClosedAt:     now.Add(-72 * time.Hour),
		Status:             "SUCCESS",
		CPUInsn:            1_000_000,
		MemByte:            1_000_000,
		ResourceFeeCharged: 1_000_000,
	})
	if err := s.BatchInsertInvocations(ctx, invocations); err != nil {
		t.Fatalf("BatchInsertInvocations: %v", err)
	}

	got, err := s.GetContractResourcePercentiles(ctx, contractID, "24h")
	if err != nil {
		t.Fatalf("24h window: %v", err)
	}
	want := store.ResourcePercentiles{
		CPUInsn: store.PercentileValues{P50: 5100, P95: 9600, P99: 10000},
		MemByte: store.PercentileValues{P50: 510, P95: 960, P99: 1000},
		Fee:     store.PercentileValues{P50: 51, P95: 96, P99: 100},
	}
	assertPercentiles(t, "cpu_insn", got.CPUInsn, want.CPUInsn)
	assertPercentiles(t, "mem_byte", got.MemByte, want.MemByte)
	assertPercentiles(t, "fee", got.Fee, want.Fee)

	wider, err := s.GetContractResourcePercentiles(ctx, contractID, "7d")
	if err != nil {
		t.Fatalf("7d window: %v", err)
	}
	if wider.CPUInsn.P99 <= got.CPUInsn.P99 {
		t.Errorf("7d p99 = %v, want it above the 24h p99 %v", wider.CPUInsn.P99, got.CPUInsn.P99)
	}
}

func assertPercentiles(t *testing.T, metric string, got, want store.PercentileValues) {
	t.Helper()
	const tolerance = 1e-6
	if math.Abs(got.P50-want.P50) > tolerance || math.Abs(got.P95-want.P95) > tolerance || math.Abs(got.P99-want.P99) > tolerance {
		t.Errorf("%s = %+v, want %+v", metric, got, want)
	}
}
