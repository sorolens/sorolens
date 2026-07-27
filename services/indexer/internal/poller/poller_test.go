package poller

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

// ---- fake RPCClient -------------------------------------------------------

type fakeRPC struct {
	mu           sync.Mutex
	latestLedger *LatestLedger
	latestErr    error
	events       map[string]*GetEventsResult // key: "contractID:start-end"
	transactions map[string]*TransactionResult
	txErr        error
	eventsCalls  []getEventsCall
}

type getEventsCall struct {
	StartLedger uint32
	EndLedger   uint32
	Filters     []EventFilter
}

func (f *fakeRPC) GetLatestLedger(ctx context.Context) (*LatestLedger, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.latestErr != nil {
		return nil, f.latestErr
	}
	if f.latestLedger == nil {
		return &LatestLedger{Sequence: 500000, ProtocolVersion: 22}, nil
	}
	return f.latestLedger, nil
}

func (f *fakeRPC) GetEvents(_ context.Context, start, end uint32, filters []EventFilter) (*GetEventsResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.eventsCalls = append(f.eventsCalls, getEventsCall{start, end, filters})
	key := ""
	if len(filters) > 0 && len(filters[0].ContractIDs) > 0 {
		key = filters[0].ContractIDs[0]
	}
	if r, ok := f.events[key]; ok {
		return r, nil
	}
	return &GetEventsResult{LatestLedger: 500000}, nil
}

func (f *fakeRPC) GetTransaction(_ context.Context, hash string) (*TransactionResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.txErr != nil {
		return nil, f.txErr
	}
	if tx, ok := f.transactions[hash]; ok {
		return tx, nil
	}
	return &TransactionResult{Status: "SUCCESS", Ledger: 490000}, nil
}

// ---- fake Store -----------------------------------------------------------

type fakeStore struct {
	mu          sync.Mutex
	contracts   []Contract
	syncStates  map[string]SyncState
	events      []Event
	invocations []Invocation
	syncErr     error
	listErr     error
}

func newFakeStore(contracts []Contract) *fakeStore {
	return &fakeStore{
		contracts:  contracts,
		syncStates: make(map[string]SyncState),
	}
}

func (f *fakeStore) ListContracts(_ context.Context, cursor string, limit int) ([]Contract, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	return f.contracts, "", nil
}

func (f *fakeStore) BatchInsertEvents(_ context.Context, events []Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, events...)
	return nil
}

func (f *fakeStore) BatchInsertInvocations(_ context.Context, invs []Invocation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.invocations = append(f.invocations, invs...)
	return nil
}

func (f *fakeStore) GetSyncState(_ context.Context, contractID string) (SyncState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.syncStates[contractID]; ok {
		return s, nil
	}
	return SyncState{ContractID: contractID}, nil
}

func (f *fakeStore) UpsertSyncState(_ context.Context, s SyncState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.syncErr != nil {
		return f.syncErr
	}
	f.syncStates[s.ContractID] = s
	return nil
}

// ---- fake RedisClient -----------------------------------------------------

type fakeRedis struct {
	mu   sync.Mutex
	held map[string]struct{}
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{held: make(map[string]struct{})}
}

func (r *fakeRedis) SetNX(_ context.Context, key, _ string, _ time.Duration) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.held[key]; exists {
		return false, nil
	}
	r.held[key] = struct{}{}
	return true, nil
}

func (r *fakeRedis) Del(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.held, key)
	return nil
}

// ---- helpers --------------------------------------------------------------

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func testConfig() Config {
	return Config{
		LedgerWindow: 1000,
		PollInterval: 10 * time.Millisecond,
		MaxDuration:  5 * time.Second,
	}
}

// ---- tests ----------------------------------------------------------------

func TestPoller_RunOnce_noContracts(t *testing.T) {
	t.Parallel()
	p := New(&fakeRPC{}, newFakeStore(nil), newFakeRedis(), testConfig(), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPoller_RunOnce_indexesActiveContract(t *testing.T) {
	t.Parallel()

	contractID := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	store := newFakeStore([]Contract{{ID: contractID, Status: "active"}})
	// Give it a known last ledger so we know the window precisely.
	store.syncStates[contractID] = SyncState{ContractID: contractID, LastLedger: 499000}

	rpc := &fakeRPC{
		latestLedger: &LatestLedger{Sequence: 500000},
		events: map[string]*GetEventsResult{
			contractID: {
				Events: []RPCEvent{
					{
						ID:                       "0001-0001",
						ContractID:               contractID,
						Ledger:                   499100,
						LedgerClosedAt:           "2026-07-26T10:00:00Z",
						TxHash:                   "abc123",
						Type:                     "contract",
						Topic:                    []string{"AAAA"},
						Value:                    "BBBB",
						InSuccessfulContractCall: true,
					},
				},
				LatestLedger: 500000,
			},
		},
		transactions: map[string]*TransactionResult{
			"abc123": {Status: "SUCCESS", Ledger: 499100},
		},
	}

	p := New(rpc, store, newFakeRedis(), testConfig(), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.events) != 1 {
		t.Errorf("events: want 1, got %d", len(store.events))
	}
	if len(store.invocations) != 1 {
		t.Errorf("invocations: want 1, got %d", len(store.invocations))
	}
	if store.events[0].ID != "0001-0001" {
		t.Errorf("event ID: want 0001-0001, got %s", store.events[0].ID)
	}
	if store.syncStates[contractID].LastLedger != 500000 {
		t.Errorf("sync state LastLedger: want 500000, got %d", store.syncStates[contractID].LastLedger)
	}
}

func TestPoller_SkipsPendingContract(t *testing.T) {
	t.Parallel()

	store := newFakeStore([]Contract{{ID: "CTEST", Status: "pending"}})
	rpc := &fakeRPC{}
	p := New(rpc, store, newFakeRedis(), testConfig(), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rpc.eventsCalls) != 0 {
		t.Errorf("expected no RPC calls for pending contract, got %d", len(rpc.eventsCalls))
	}
}

func TestPoller_NewContractStartsFromBackfillWindow(t *testing.T) {
	t.Parallel()

	contractID := "CNEW"
	store := newFakeStore([]Contract{{ID: contractID, Status: "active"}})
	// No sync state entry -> LastLedger == 0 -> triggers backfill window.

	rpc := &fakeRPC{
		latestLedger: &LatestLedger{Sequence: 200000},
	}
	p := New(rpc, store, newFakeRedis(), testConfig(), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected start: 200000 - 100000 = 100000.
	if len(rpc.eventsCalls) == 0 {
		t.Fatal("expected at least one GetEvents call")
	}
	wantStart := uint32(200000 - newContractBackfillWindow)
	if rpc.eventsCalls[0].StartLedger != wantStart {
		t.Errorf("start ledger: want %d, got %d", wantStart, rpc.eventsCalls[0].StartLedger)
	}
}

func TestPoller_RespectsRedisLock(t *testing.T) {
	t.Parallel()

	contractID := "CLOCKED"
	store := newFakeStore([]Contract{{ID: contractID, Status: "active"}})
	store.syncStates[contractID] = SyncState{ContractID: contractID, LastLedger: 499000}

	redis := newFakeRedis()
	// Pre-hold the lock.
	redis.held[lockKeyPrefix+contractID] = struct{}{}

	rpc := &fakeRPC{}
	p := New(rpc, store, redis, testConfig(), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Lock was held: no RPC calls should have been made.
	if len(rpc.eventsCalls) != 0 {
		t.Errorf("expected no GetEvents calls when locked, got %d", len(rpc.eventsCalls))
	}
}

func TestPoller_RPCErrorDoesNotPanic(t *testing.T) {
	t.Parallel()

	store := newFakeStore([]Contract{{ID: "CERR", Status: "active"}})
	store.syncStates["CERR"] = SyncState{ContractID: "CERR", LastLedger: 100}

	rpc := &fakeRPC{latestErr: errors.New("rpc unavailable")}
	p := New(rpc, store, newFakeRedis(), testConfig(), testLogger())

	// Should not panic; error is logged and Run returns nil (continues).
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPoller_UsesContractNetworkRPCClient(t *testing.T) {
	t.Parallel()

	contractID := "CNETWORK"
	store := newFakeStore([]Contract{{ID: contractID, Status: "active", Network: "testnet"}})
	store.syncStates[contractID] = SyncState{ContractID: contractID, LastLedger: 499000}

	testnetRPC := &fakeRPC{latestLedger: &LatestLedger{Sequence: 500000}}
	mainnetRPC := &fakeRPC{latestLedger: &LatestLedger{Sequence: 500000}}
	p := NewWithRPCClients(map[string]RPCClient{
		"testnet": testnetRPC,
		"mainnet": mainnetRPC,
	}, store, newFakeRedis(), testConfig(), testLogger())

	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(testnetRPC.eventsCalls) != 1 {
		t.Fatalf("expected one GetEvents call on testnet RPC, got %d", len(testnetRPC.eventsCalls))
	}
	if len(mainnetRPC.eventsCalls) != 0 {
		t.Fatalf("expected no GetEvents call on mainnet RPC, got %d", len(mainnetRPC.eventsCalls))
	}
}

func TestPoller_SkipsUnconfiguredNetwork(t *testing.T) {
	t.Parallel()

	contractID := "CUNCONFIGURED"
	store := newFakeStore([]Contract{{ID: contractID, Status: "active", Network: "mainnet"}})
	store.syncStates[contractID] = SyncState{ContractID: contractID, LastLedger: 499000}

	testnetRPC := &fakeRPC{latestLedger: &LatestLedger{Sequence: 500000}}
	p := NewWithRPCClients(map[string]RPCClient{"testnet": testnetRPC}, store, newFakeRedis(), testConfig(), testLogger())

	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(testnetRPC.eventsCalls) != 0 {
		t.Fatalf("expected no RPC calls for unconfigured network, got %d", len(testnetRPC.eventsCalls))
	}
}

func TestPoller_ListContractsErrorPropagates(t *testing.T) {
	t.Parallel()

	store := newFakeStore(nil)
	store.listErr = errors.New("db unavailable")

	p := New(&fakeRPC{}, store, newFakeRedis(), testConfig(), testLogger())
	err := p.Run(context.Background(), "once")
	if err == nil {
		t.Fatal("expected error when store.ListContracts fails")
	}
}

func TestPoller_ContinuousMode_shutsDownOnCancel(t *testing.T) {
	t.Parallel()

	p := New(&fakeRPC{}, newFakeStore(nil), newFakeRedis(), testConfig(), testLogger())
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- p.Run(ctx, "continuous") }()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error on shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for continuous mode to shut down")
	}
}

func TestPoller_UnknownModeReturnsError(t *testing.T) {
	t.Parallel()
	p := New(&fakeRPC{}, newFakeStore(nil), newFakeRedis(), testConfig(), testLogger())
	if err := p.Run(context.Background(), "invalid"); err == nil {
		t.Fatal("expected error for unknown mode")
	}
}
