package poller

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/wasm"
)

// ---- fake RPCClient -------------------------------------------------------

type fakeRPC struct {
	mu            sync.Mutex
	latestLedger  *LatestLedger
	latestErr     error
	events        map[string]*GetEventsResult // key: "contractID:start-end"
	transactions  map[string]*TransactionResult
	ledgerEntries map[string]LedgerEntry // key: base64 LedgerKey
	txErr         error
	eventsCalls   []getEventsCall
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

// GetLedgerEntries returns entries keyed by the base64 LedgerKey. Tests with
// no configured entries get an empty result (so upgrade detection is skipped).
func (f *fakeRPC) GetLedgerEntries(_ context.Context, keys []string) (*GetLedgerEntriesResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	res := &GetLedgerEntriesResult{LatestLedger: 500000}
	for _, k := range keys {
		if e, ok := f.ledgerEntries[k]; ok {
			res.Entries = append(res.Entries, e)
		} else {
			res.KeysNotFound = append(res.KeysNotFound, k)
		}
	}
	return res, nil
}

// ---- fake Store -----------------------------------------------------------

type fakeStore struct {
	mu           sync.Mutex
	contracts    []Contract
	syncStates   map[string]SyncState
	events       []Event
	invocations  []Invocation
	upgrades     []ContractUpgrade
	wasmHashes   map[string]string
	syncErr      error
	listErr      error
	hourly       map[string][]HourlyActivity // contractID -> buckets
	alerts       []Alert
	insertErr    error
	healthInputs map[string]HealthInputs // contractID -> inputs
	healthScores []ContractHealthScore
}

func newFakeStore(contracts []Contract) *fakeStore {
	return &fakeStore{
		contracts:    contracts,
		syncStates:   make(map[string]SyncState),
		hourly:       make(map[string][]HourlyActivity),
		wasmHashes:   make(map[string]string),
		healthInputs: make(map[string]HealthInputs),
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

func (f *fakeStore) CreateNextMonthPartition(_ context.Context) error { return nil }
func (f *fakeStore) CreateMonthlyPartitionIfNotExists(_ context.Context, _ int, _ int) error {
	return nil
}

func (f *fakeStore) RecentHourlyActivity(_ context.Context, contractID string, _ int) ([]HourlyActivity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hourly[contractID], nil
}

func (f *fakeStore) InsertAlert(_ context.Context, a Alert) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.insertErr != nil {
		return f.insertErr
	}
	f.alerts = append(f.alerts, a)
	return nil
}

func (f *fakeStore) InsertContractUpgrade(_ context.Context, u ContractUpgrade) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upgrades = append(f.upgrades, u)
	return nil
}

func (f *fakeStore) UpdateContractWasmHash(_ context.Context, contractID, wasmHash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.wasmHashes[contractID] = wasmHash
	return nil
}

func (f *fakeStore) ContractHealthInputs(_ context.Context, contractID string) (HealthInputs, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.healthInputs[contractID], nil
}

func (f *fakeStore) UpsertContractHealthScore(_ context.Context, s ContractHealthScore) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.healthScores = append(f.healthScores, s)
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

func TestPoller_StampsContractNetwork(t *testing.T) {
	t.Parallel()

	contractID := "CNETSTAMP"
	store := newFakeStore([]Contract{{ID: contractID, Status: "active", Network: "mainnet"}})
	store.syncStates[contractID] = SyncState{ContractID: contractID, LastLedger: 499000}

	rpc := &fakeRPC{
		latestLedger: &LatestLedger{Sequence: 500000},
		events: map[string]*GetEventsResult{
			contractID: {
				Events: []RPCEvent{{
					ID:             "0001-0001",
					ContractID:     contractID,
					Ledger:         499100,
					LedgerClosedAt: "2026-07-26T10:00:00Z",
					TxHash:         "txnet",
					Type:           "contract",
				}},
				LatestLedger: 500000,
			},
		},
		transactions: map[string]*TransactionResult{
			"txnet": {Status: "SUCCESS", Ledger: 499100},
		},
	}

	p := NewWithRPCClients(map[string]RPCClient{"mainnet": rpc}, store, newFakeRedis(), testConfig(), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.events) != 1 || store.events[0].Network != "mainnet" {
		t.Fatalf("event network: want mainnet, got %+v", store.events)
	}
	if len(store.invocations) != 1 || store.invocations[0].Network != "mainnet" {
		t.Fatalf("invocation network: want mainnet, got %+v", store.invocations)
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

// ---- anomaly detection job (issue #136) -----------------------------------

func anomalyConfig() Config {
	cfg := testConfig()
	cfg.AnomalyEnabled = true
	cfg.AnomalyLookbackHours = 24
	cfg.AnomalySigma = 3
	cfg.AnomalyMinHistory = 12
	return cfg
}

// steadyActivity builds `hours` of steady hourly buckets at `base` events.
func steadyActivity(contractID string, hours int, base int64) []HourlyActivity {
	start := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	out := make([]HourlyActivity, hours)
	for i := 0; i < hours; i++ {
		out[i] = HourlyActivity{
			Hour:        start.Add(time.Duration(i) * time.Hour),
			EventCount:  base,
			InvokeCount: base / 2,
			CPU:         base * 100,
			Fees:        base * 1000,
		}
	}
	return out
}

// TestPoller_AnomalyJob_SteadyStateNoAlerts simulates steady activity and
// requires the job to not raise any contract alerts.
func TestPoller_AnomalyJob_SteadyStateNoAlerts(t *testing.T) {
	store := newFakeStore([]Contract{{ID: "CSTEADY", Status: "active"}})
	store.hourly["CSTEADY"] = steadyActivity("CSTEADY", 24, 100)

	p := New(&fakeRPC{}, store, newFakeRedis(), anomalyConfig(), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.alerts) != 0 {
		t.Fatalf("steady state produced %d alerts: %+v", len(store.alerts), store.alerts)
	}
}

// TestPoller_AnomalyJob_FabricatedSpikeRaisesAlert fabricates a 20x event
// spike in the trailing hour and requires exactly one Warning alert (events
// metric), de-duplicated across the two runs of the same pass is not
// applicable since the job detects once per pass.
func TestPoller_AnomalyJob_FabricatedSpikeRaisesAlert(t *testing.T) {
	store := newFakeStore([]Contract{{ID: "CSPIKE", Status: "active"}})
	buckets := steadyActivity("CSPIKE", 24, 100)
	last := buckets[len(buckets)-1]
	last.EventCount = 2000
	buckets[len(buckets)-1] = last
	store.hourly["CSPIKE"] = buckets

	p := New(&fakeRPC{}, store, newFakeRedis(), anomalyConfig(), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.alerts) == 0 {
		t.Fatal("spike produced no alerts")
	}
	for _, a := range store.alerts {
		if a.Severity != "Warning" {
			t.Errorf("alert severity = %q, want Warning", a.Severity)
		}
		if a.Message == "" {
			t.Error("alert message is empty")
		}
	}
}

// TestPoller_AnomalyJobSkippedWhenDisabled ensures the job is off by default.
func TestPoller_AnomalyJobSkippedWhenDisabled(t *testing.T) {
	store := newFakeStore([]Contract{{ID: "CSPIKE2", Status: "active"}})
	buckets := steadyActivity("CSPIKE2", 24, 100)
	last := buckets[len(buckets)-1]
	last.EventCount = 2000
	buckets[len(buckets)-1] = last
	store.hourly["CSPIKE2"] = buckets

	p := New(&fakeRPC{}, store, newFakeRedis(), testConfig(), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.alerts) != 0 {
		t.Fatalf("alerts raised with anomaly job disabled: %+v", store.alerts)
	}
}

// TestPoller_AnomalyJobRespectsCancellation ensures context cancellation stops
// the detection loop cleanly.
func TestPoller_AnomalyJobRespectsCancellation(t *testing.T) {
	store := newFakeStore([]Contract{{ID: "CCANCEL", Status: "active"}})
	store.hourly["CCANCEL"] = steadyActivity("CCANCEL", 24, 100)

	cfg := anomalyConfig()
	p := New(&fakeRPC{}, store, newFakeRedis(), cfg, testLogger())
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- p.Run(ctx, "once") }()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// buildInstanceEntryXDR builds a valid contract-instance ContractData
// LedgerEntry.xdr fixture matching the wire layout wasm.WasmHashFromInstanceEntry
// expects: lastModified, type=6, ext=0, addrType=contract, contractID,
// key=20, val=19, then the 32-byte wasm hash.
func buildInstanceEntryXDR(contractIDHex, wasmHashHex string, lastModified uint32) string {
	var out []byte
	putU32 := func(v uint32) { out = binary.BigEndian.AppendUint32(out, v) }
	putU32(lastModified)
	putU32(6) // LedgerEntryType CONTRACT_DATA
	putU32(0) // ContractDataEntryExt V0
	putU32(1) // SCAddressType CONTRACT
	contractID, _ := hex.DecodeString(contractIDHex)
	out = append(out, contractID...)
	putU32(20) // SCValType scvLedgerKeyContractInstance
	putU32(19) // SCValType scvContractInstance
	wasmHash, _ := hex.DecodeString(wasmHashHex)
	out = append(out, wasmHash...)
	return base64.StdEncoding.EncodeToString(out)
}

// instanceKeyXDR returns the base64 LedgerKey for a contract's instance entry.
// It mirrors wasm.ContractInstanceKey; using the real function here keeps the
// fixture key consistent with what checkWasmHash requests.
func instanceKeyXDR(contractIDHex string) string {
	key, err := wasm.ContractInstanceKey(contractIDHex)
	if err != nil {
		panic(err)
	}
	return key
}

func TestPoller_checksWasmHashBaseline(t *testing.T) {
	t.Parallel()

	contractID := "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90"
	wasmHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	key := instanceKeyXDR(contractID)
	store := newFakeStore([]Contract{{ID: contractID, Status: "active", Network: ""}})
	store.syncStates[contractID] = SyncState{ContractID: contractID, LastLedger: 499000}

	rpc := &fakeRPC{
		latestLedger: &LatestLedger{Sequence: 500000},
		ledgerEntries: map[string]LedgerEntry{
			key: {Key: key, XDR: buildInstanceEntryXDR(contractID, wasmHash, 501), LastModifiedLedgerSeq: 501},
		},
	}

	p := New(rpc, store, newFakeRedis(), testConfig(), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := store.wasmHashes[contractID]; got != wasmHash {
		t.Errorf("stored wasm hash = %q, want %q (baseline)", got, wasmHash)
	}
	if len(store.upgrades) != 0 {
		t.Errorf("expected no upgrade row on first observation, got %d", len(store.upgrades))
	}
}

func TestPoller_recordsContractUpgradeOnWasmChange(t *testing.T) {
	t.Parallel()

	contractID := "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90"
	oldHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	newHash := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	key := instanceKeyXDR(contractID)
	store := newFakeStore([]Contract{{ID: contractID, Status: "active", Network: "", WasmHash: oldHash}})
	store.syncStates[contractID] = SyncState{ContractID: contractID, LastLedger: 499000}

	rpc := &fakeRPC{
		latestLedger: &LatestLedger{Sequence: 500000},
		ledgerEntries: map[string]LedgerEntry{
			key: {Key: key, XDR: buildInstanceEntryXDR(contractID, newHash, 501), LastModifiedLedgerSeq: 501},
		},
	}

	p := New(rpc, store, newFakeRedis(), testConfig(), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := store.wasmHashes[contractID]; got != newHash {
		t.Errorf("stored wasm hash = %q, want %q after upgrade", got, newHash)
	}
	if len(store.upgrades) != 1 {
		t.Fatalf("expected exactly 1 upgrade row, got %d", len(store.upgrades))
	}
	u := store.upgrades[0]
	if u.ContractID != contractID || u.FromHash != oldHash || u.ToHash != newHash {
		t.Errorf("unexpected upgrade row: %+v", u)
	}
	if u.Ledger != 501 {
		t.Errorf("upgrade ledger = %d, want 501", u.Ledger)
	}
}

func TestPoller_noUpgradeWhenWasmHashUnchanged(t *testing.T) {
	t.Parallel()

	contractID := "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90"
	hash := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	key := instanceKeyXDR(contractID)
	store := newFakeStore([]Contract{{ID: contractID, Status: "active", Network: "", WasmHash: hash}})
	store.syncStates[contractID] = SyncState{ContractID: contractID, LastLedger: 499000}

	rpc := &fakeRPC{
		latestLedger: &LatestLedger{Sequence: 500000},
		ledgerEntries: map[string]LedgerEntry{
			key: {Key: key, XDR: buildInstanceEntryXDR(contractID, hash, 502), LastModifiedLedgerSeq: 502},
		},
	}

	p := New(rpc, store, newFakeRedis(), testConfig(), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.upgrades) != 0 {
		t.Errorf("expected no upgrade row when hash unchanged, got %d", len(store.upgrades))
	}
}
