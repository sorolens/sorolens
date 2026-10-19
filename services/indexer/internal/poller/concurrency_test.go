package poller

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ---- test fake -------------------------------------------------------------

// concurrencyRPC is an RPC fake for the fan-out tests. It records how many
// GetEvents calls are in flight at once so a test can assert both halves of
// the semaphore contract: never more than the limit, and genuinely more than
// one. When want > 1 each call parks until `want` callers are inside the fake
// (or the wait times out), which makes "does it really run in parallel?"
// deterministic instead of wall-clock dependent.
type concurrencyRPC struct {
	mu          sync.Mutex
	inFlight    int
	peak        int
	calls       int
	want        int
	waitTimeout time.Duration
	failFor     string // contract ID whose GetEvents returns an error
}

func (f *concurrencyRPC) GetLatestLedger(context.Context) (*LatestLedger, error) {
	return &LatestLedger{Sequence: 500000, ProtocolVersion: 22}, nil
}

func (f *concurrencyRPC) GetLedgerEntries(context.Context, []string) (*GetLedgerEntriesResult, error) {
	return &GetLedgerEntriesResult{LatestLedger: 500000}, nil
}

func (f *concurrencyRPC) GetTransaction(context.Context, string) (*TransactionResult, error) {
	return &TransactionResult{Status: "SUCCESS"}, nil
}

func (f *concurrencyRPC) GetEvents(ctx context.Context, _, _ uint32, filters []EventFilter) (*GetEventsResult, error) {
	contractID := ""
	if len(filters) > 0 && len(filters[0].ContractIDs) > 0 {
		contractID = filters[0].ContractIDs[0]
	}

	f.mu.Lock()
	f.calls++
	f.inFlight++
	if f.inFlight > f.peak {
		f.peak = f.inFlight
	}
	target, timeout := f.want, f.waitTimeout
	f.mu.Unlock()

	if target > 1 {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			f.mu.Lock()
			n := f.inFlight
			f.mu.Unlock()
			if n >= target {
				break
			}
			time.Sleep(time.Millisecond)
		}
	}

	f.mu.Lock()
	f.inFlight--
	f.mu.Unlock()

	if contractID == f.failFor {
		return nil, errors.New("simulated RPC failure")
	}
	return &GetEventsResult{LatestLedger: 500000}, nil
}

// stats returns the peak number of concurrent GetEvents calls and the total
// number of GetEvents calls made.
func (f *concurrencyRPC) stats() (peak, calls int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.peak, f.calls
}

// ---- helpers ---------------------------------------------------------------

// concurrencyConfig runs a pass with `limit` workers and RPC rate limiting
// switched off, so the test measures scheduling alone.
func concurrencyConfig(limit int) Config {
	return Config{
		LedgerWindow:  1000,
		Concurrency:   limit,
		RPCRateLimits: map[string]float64{"": 0}, // unlimited
	}
}

// activeContracts returns n active contracts with a known sync state so each
// one reaches GetEvents immediately.
func activeContracts(n int) []Contract {
	out := make([]Contract, n)
	for i := range out {
		out[i] = Contract{ID: fmt.Sprintf("CCON%02d", i), Status: "active"}
	}
	return out
}

// seededStore returns a fake store whose contracts are all mid-sync at ledger
// 499000; a successful pass advances each of them to 500000.
func seededStore(contracts []Contract) *fakeStore {
	store := newFakeStore(contracts)
	for _, c := range contracts {
		store.syncStates[c.ID] = SyncState{ContractID: c.ID, LastLedger: 499000}
	}
	return store
}

// ---- tests -----------------------------------------------------------------

// TestPoller_ConcurrencyRespectsLimit covers acceptance criterion 1: N
// contracts process in parallel, up to the configured limit and no further.
func TestPoller_ConcurrencyRespectsLimit(t *testing.T) {
	t.Parallel()

	const (
		limit     = 3
		contracts = 9 // multiple of the limit, so every wave fills it
	)

	list := activeContracts(contracts)
	store := seededStore(list)
	// want == limit: each wave parks until `limit` callers are inside, which
	// only a real fan-out can satisfy.
	rpc := &concurrencyRPC{want: limit, waitTimeout: 500 * time.Millisecond}

	p := New(rpc, store, newFakeRedis(), concurrencyConfig(limit), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	peak, calls := rpc.stats()
	if peak > limit {
		t.Errorf("in-flight contracts = %d, want <= %d (semaphore not enforced)", peak, limit)
	}
	if peak < limit {
		t.Errorf("in-flight contracts = %d, want %d (contracts did not run in parallel)", peak, limit)
	}
	if calls != contracts {
		t.Errorf("GetEvents calls = %d, want %d", calls, contracts)
	}

	// Every contract must have completed: a pass advances LastLedger from
	// 499000 to 500000 (499001 + window - 1, capped at latest = 500000).
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, c := range list {
		if got := store.syncStates[c.ID].LastLedger; got != 500000 {
			t.Errorf("contract %s LastLedger = %d, want 500000", c.ID, got)
		}
	}
}

// TestPoller_FailingContractDoesNotStopOthers covers acceptance criterion 2:
// one broken contract is logged and skipped while the rest still complete.
func TestPoller_FailingContractDoesNotStopOthers(t *testing.T) {
	t.Parallel()

	list := activeContracts(4)
	failing := list[1].ID

	store := seededStore(list)
	// want == 2: prove the pass stays concurrent rather than collapsing to
	// one contract at a time after the first error.
	rpc := &concurrencyRPC{failFor: failing, want: 2, waitTimeout: 500 * time.Millisecond}

	p := New(rpc, store, newFakeRedis(), concurrencyConfig(4), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("processAll must stay clean when a single contract fails, got: %v", err)
	}

	peak, calls := rpc.stats()
	if peak < 2 {
		t.Errorf("in-flight contracts = %d, want > 1 (failure must not serialize the pass)", peak)
	}
	if calls != len(list) {
		t.Errorf("GetEvents calls = %d, want %d (other contracts must still run)", calls, len(list))
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	for _, c := range list {
		last := store.syncStates[c.ID].LastLedger
		if c.ID == failing {
			if last == 500000 {
				t.Errorf("failing contract %s was recorded as indexed", c.ID)
			}
			continue
		}
		if last != 500000 {
			t.Errorf("contract %s LastLedger = %d, want 500000", c.ID, last)
		}
	}
}

// TestPoller_UnsetConcurrencyRunsSequentially guards the zero-value fallback:
// Config without Concurrency keeps the historical one-at-a-time behavior, so
// every pre-existing test and embedder stays deterministic.
func TestPoller_UnsetConcurrencyRunsSequentially(t *testing.T) {
	t.Parallel()

	list := activeContracts(4)
	store := seededStore(list)
	// want == 2: only two overlapping callers would clear the barrier, and
	// sequential execution never produces them.
	rpc := &concurrencyRPC{want: 2, waitTimeout: 50 * time.Millisecond}

	p := New(rpc, store, newFakeRedis(), concurrencyConfig(0), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	peak, calls := rpc.stats()
	if peak != 1 {
		t.Errorf("in-flight contracts = %d, want 1 when Concurrency is unset", peak)
	}
	if calls != len(list) {
		t.Errorf("GetEvents calls = %d, want %d", calls, len(list))
	}
}
