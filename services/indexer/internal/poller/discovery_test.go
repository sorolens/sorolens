package poller

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// Envelope for a CREATE_CONTRACT_V2 operation submitted (and deployed from)
// discoveryAccount on testnet, generated with the Python stellar-sdk. See
// internal/discovery for the full fixture set.
const (
	discoveryAccount     = "GCFIRY65OQE7DFP5KLNS2PF2LVZMUZYJX4OZIEQ36N2IQANUB5XVYOJR"
	discoveryOther       = "GCATS5YOVB6ROX2WUNKGNQ2MP3GMXDMKSG2O4N5CLX3A6W4PZGZZI55U"
	discoveredContract   = "CASLIQNFCMW3UXPTVA5KDQZK6NEBPK4K35XY2SEQ3D7JGRCOVLP2BKBK"
	createContractEnvXDR = "AAAAAgAAAACKiOPddAnxlf1S2y08ul1yymcJvx2UEhvzdIgBtA9vXAAAAGQAAAAAAAAAZQAAAAEAAAAAAAAAAAAAAABqtc/XAAAAAAAAAAEAAAAAAAAAGAAAAAMAAAAAAAAAAAAAAACKiOPddAnxlf1S2y08ul1yymcJvx2UEhvzdIgBtA9vXAkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJAAAAAAcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHAAAAAAAAAAAAAAAAAAAAAbQPb1wAAABA5aONoYGOc+6cgzNer55szSaCepvoisKlMKivkWC20HBojwVgCjY2NkjZkN21ICcW7t8IQQQoLqd3BH3CER4fDA=="
)

// discoveryRPC is a fakeRPC that also serves getTransactions pages keyed by
// pagination cursor ("" is the first page).
type discoveryRPC struct {
	*fakeRPC
	mu       sync.Mutex
	pages    map[string]*GetTransactionsResult
	txErr    error
	txStarts []uint32
}

func (d *discoveryRPC) GetTransactions(_ context.Context, start uint32, cursor string, _ int) (*GetTransactionsResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.txStarts = append(d.txStarts, start)
	if d.txErr != nil {
		return nil, d.txErr
	}
	if p, ok := d.pages[cursor]; ok {
		return p, nil
	}
	return &GetTransactionsResult{LatestLedger: 500000}, nil
}

// decoratorRPC mimics the watchdog interceptor in main.go: it embeds the
// RPCClient interface, hiding optional methods unless unwrapped.
type decoratorRPC struct{ RPCClient }

func (d decoratorRPC) Unwrap() RPCClient { return d.RPCClient }

// discoveryStore is a fakeStore that supports contract discovery. A recorded
// contract is appended to the tracked contract list so the same pass
// indexes it, mirroring the postgres implementation.
type discoveryStore struct {
	*fakeStore
	watched    []WatchedAccount
	recorded   []DiscoveredContract
	recordErr  error
	listErrAcc error
}

func (s *discoveryStore) ListWatchedAccounts(_ context.Context) ([]WatchedAccount, error) {
	return s.watched, s.listErrAcc
}

func (s *discoveryStore) RecordDiscoveredContract(_ context.Context, d DiscoveredContract) (bool, error) {
	if s.recordErr != nil {
		return false, s.recordErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.contracts {
		if c.ID == d.ContractID {
			return false, nil
		}
	}
	s.recorded = append(s.recorded, d)
	s.contracts = append(s.contracts, Contract{ID: d.ContractID, Status: "active", Network: d.Network})
	s.syncStates[d.ContractID] = SyncState{ContractID: d.ContractID, LastLedger: uint32(d.Ledger - 1)}
	return true, nil
}

func newDiscoveryFixture(watched ...string) (*discoveryRPC, *discoveryStore) {
	rpc := &discoveryRPC{
		fakeRPC: &fakeRPC{latestLedger: &LatestLedger{Sequence: 500000}},
		pages: map[string]*GetTransactionsResult{
			"": {
				LatestLedger: 500000,
				Cursor:       "end",
				Transactions: []LedgerTransaction{{
					Status:      "SUCCESS",
					Ledger:      499990,
					TxHash:      "deploytx",
					EnvelopeXDR: createContractEnvXDR,
				}},
			},
		},
	}
	st := &discoveryStore{fakeStore: newFakeStore(nil)}
	for _, a := range watched {
		st.watched = append(st.watched, WatchedAccount{AccountID: a})
	}
	return rpc, st
}

func TestDiscoveryTracksContractDeployedByWatchedAccount(t *testing.T) {
	rpc, st := newDiscoveryFixture(discoveryAccount)
	p := NewWithRPCClients(map[string]RPCClient{"testnet": rpc}, st, newFakeRedis(), testConfig(), testLogger())

	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(st.recorded) != 1 {
		t.Fatalf("want 1 discovered contract, got %d", len(st.recorded))
	}
	got := st.recorded[0]
	want := DiscoveredContract{ContractID: discoveredContract, Network: "testnet", AccountID: discoveryAccount, Ledger: 499990}
	if got != want {
		t.Fatalf("want %+v, got %+v", want, got)
	}

	// The first scan of a network starts discoveryInitialLookback ledgers
	// back, and the cursor ends at the tip.
	if rpc.txStarts[0] != 500000-discoveryInitialLookback {
		t.Errorf("first scan start: want %d, got %d", 500000-discoveryInitialLookback, rpc.txStarts[0])
	}
	if c := st.indexerCursors["discovery:testnet"]; c != 500000 {
		t.Errorf("discovery cursor: want 500000, got %d", c)
	}

	// Same pass: the discovered contract is indexed from its deploy ledger.
	var indexed bool
	for _, call := range rpc.eventsCalls {
		if len(call.Filters) > 0 && len(call.Filters[0].ContractIDs) > 0 &&
			call.Filters[0].ContractIDs[0] == discoveredContract {
			indexed = true
			if call.StartLedger != 499990 {
				t.Errorf("index start: want deploy ledger 499990, got %d", call.StartLedger)
			}
		}
	}
	if !indexed {
		t.Fatal("discovered contract was not indexed in the same pass")
	}

	// A second pass resumes after the cursor and does not re-record.
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if len(st.recorded) != 1 {
		t.Fatalf("second pass must not re-record, got %d", len(st.recorded))
	}
}

func TestDiscoveryIgnoresUnwatchedAndFailed(t *testing.T) {
	t.Run("unwatched account", func(t *testing.T) {
		rpc, st := newDiscoveryFixture(discoveryOther)
		p := NewWithRPCClients(map[string]RPCClient{"testnet": rpc}, st, newFakeRedis(), testConfig(), testLogger())
		p.runDiscovery(context.Background())
		if len(st.recorded) != 0 {
			t.Fatalf("want nothing tracked, got %+v", st.recorded)
		}
		if c := st.indexerCursors["discovery:testnet"]; c != 500000 {
			t.Errorf("cursor must still advance, got %d", c)
		}
	})
	t.Run("failed transaction", func(t *testing.T) {
		rpc, st := newDiscoveryFixture(discoveryAccount)
		rpc.pages[""].Transactions[0].Status = "FAILED"
		p := NewWithRPCClients(map[string]RPCClient{"testnet": rpc}, st, newFakeRedis(), testConfig(), testLogger())
		p.runDiscovery(context.Background())
		if len(st.recorded) != 0 {
			t.Fatalf("failed tx must not be tracked, got %+v", st.recorded)
		}
	})
}

func TestDiscoveryNoWatchedAccountsSkipsScan(t *testing.T) {
	rpc, st := newDiscoveryFixture()
	p := NewWithRPCClients(map[string]RPCClient{"testnet": rpc}, st, newFakeRedis(), testConfig(), testLogger())
	p.runDiscovery(context.Background())

	if len(rpc.txStarts) != 0 {
		t.Fatalf("no watched accounts: getTransactions must not be called, got %d calls", len(rpc.txStarts))
	}
	if c := st.indexerCursors["discovery:testnet"]; c != 500000 {
		t.Errorf("cursor must move to the tip, got %d", c)
	}
}

func TestDiscoveryReachesClientThroughDecorator(t *testing.T) {
	rpc, st := newDiscoveryFixture(discoveryAccount)
	wrapped := decoratorRPC{RPCClient: rpc}
	p := NewWithRPCClients(map[string]RPCClient{"testnet": wrapped}, st, newFakeRedis(), testConfig(), testLogger())
	p.runDiscovery(context.Background())

	if len(st.recorded) != 1 {
		t.Fatalf("want discovery through Unwrap, got %d recorded", len(st.recorded))
	}
}

func TestDiscoveryPaginates(t *testing.T) {
	rpc, st := newDiscoveryFixture(discoveryAccount)
	full := make([]LedgerTransaction, discoveryPageSize)
	for i := range full {
		full[i] = LedgerTransaction{Status: "FAILED", Ledger: 499800, EnvelopeXDR: createContractEnvXDR}
	}
	last := rpc.pages[""]
	rpc.pages = map[string]*GetTransactionsResult{
		"":   {LatestLedger: 500000, Cursor: "p2", Transactions: full},
		"p2": last,
	}
	p := NewWithRPCClients(map[string]RPCClient{"testnet": rpc}, st, newFakeRedis(), testConfig(), testLogger())
	p.runDiscovery(context.Background())

	if len(rpc.txStarts) != 2 {
		t.Fatalf("want 2 pages fetched, got %d", len(rpc.txStarts))
	}
	if len(st.recorded) != 1 {
		t.Fatalf("want the deployment on page 2 tracked, got %d", len(st.recorded))
	}
}

func TestDiscoveryRecordErrorKeepsLedgerForRetry(t *testing.T) {
	rpc, st := newDiscoveryFixture(discoveryAccount)
	st.indexerCursors["discovery:testnet"] = 499900
	st.recordErr = errors.New("db down")
	p := NewWithRPCClients(map[string]RPCClient{"testnet": rpc}, st, newFakeRedis(), testConfig(), testLogger())
	p.runDiscovery(context.Background())

	if rpc.txStarts[0] != 499901 {
		t.Errorf("scan must resume after the cursor: want 499901, got %d", rpc.txStarts[0])
	}
	// The cursor stops before the deploy ledger so the next pass retries it.
	if c := st.indexerCursors["discovery:testnet"]; c != 499989 {
		t.Fatalf("cursor: want 499989, got %d", c)
	}
}

func TestDiscoverySkippedWithoutCapabilities(t *testing.T) {
	// Plain fakeRPC has no GetTransactions, plain fakeStore no discovery
	// methods: the pass must run as before.
	st := newFakeStore(nil)
	p := NewWithRPCClients(map[string]RPCClient{"testnet": &fakeRPC{}}, st, newFakeRedis(), testConfig(), testLogger())
	p.runDiscovery(context.Background())
	if _, ok := st.indexerCursors["discovery:testnet"]; ok {
		t.Fatal("discovery cursor must not be written without discovery support")
	}

	rpc, dst := newDiscoveryFixture(discoveryAccount)
	p = NewWithRPCClients(map[string]RPCClient{"testnet": rpc.fakeRPC}, dst, newFakeRedis(), testConfig(), testLogger())
	p.runDiscovery(context.Background())
	if len(dst.recorded) != 0 {
		t.Fatal("client without GetTransactions must be skipped")
	}
}
