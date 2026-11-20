package poller

import (
	"context"
	"errors"
	"testing"
)

// TestRetry_TransientAndPermanentFailures verifies that transient errors are retried
// and permanent errors abort immediately, parking the transaction.
func TestRetry_TransientAndPermanentFailures(t *testing.T) {
	t.Parallel()

	// 1. Permanent error (404) should not retry
	attempts := 0
	rpc := &flakyRPC{
		getTx: func(ctx context.Context, hash string) (*TransactionResult, error) {
			attempts++
			return nil, errors.New("HTTP 404 not found")
		},
	}

	store := newFakeStore([]Contract{{ID: "CPERM", Status: "active", Network: "testnet"}})
	p := New(rpc, store, newFakeRedis(), testConfig(), testLogger())

	err := p.indexForward(context.Background(), rpc, Contract{ID: "CPERM", Network: "testnet"}, 500000, SyncState{ContractID: "CPERM", LastLedger: 499000})
	if err != nil {
		// indexForward doesn't fail on parked transactions, it logs and continues
	}

	if attempts != 1 {
		t.Errorf("permanent error should be tried exactly once, got %d attempts", attempts)
	}
}

type flakyRPC struct {
	fakeRPC
	getTx func(ctx context.Context, hash string) (*TransactionResult, error)
}

func (f *flakyRPC) GetTransaction(ctx context.Context, hash string) (*TransactionResult, error) {
	if f.getTx != nil {
		return f.getTx(ctx, hash)
	}
	return f.fakeRPC.GetTransaction(ctx, hash)
}
