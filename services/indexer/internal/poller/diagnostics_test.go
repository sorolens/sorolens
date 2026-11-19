package poller

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

// diagnosticsFixture reads the go-stellar-sdk encoded samples shared with the
// callgraph package (issue #504).
type diagnosticsFixture struct {
	GetTransaction struct {
		DiagnosticEventsXdr []string `json:"diagnosticEventsXdr"`
	} `json:"getTransaction"`
	FnCallTopics       []string `json:"fnCallTopics"`
	NestedFnCallTopics []string `json:"nestedFnCallTopics"`
	TransferTopics     []string `json:"transferTopics"`
}

func loadDiagnosticsFixture(t *testing.T) diagnosticsFixture {
	t.Helper()
	raw, err := os.ReadFile("../callgraph/testdata/diagnostic_events.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var f diagnosticsFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return f
}

// fixtureContractID is the strkey of the all-0x11 contract id the fixture's
// root fn_call targets.
const fixtureContractID = "CAIRCEIRCEIRCEIRCEIRCEIRCEIRCEIRCEIRCEIRCEIRCEIRCEIRDB3V"

// runDiagnosticsPass indexes one window containing a single event for txHash
// and returns the stored invocation.
func runDiagnosticsPass(t *testing.T, cfg Config, eventTopics []string, tx *TransactionResult) Invocation {
	t.Helper()
	store := newFakeStore([]Contract{{ID: fixtureContractID, Status: "active"}})
	store.syncStates[fixtureContractID] = SyncState{ContractID: fixtureContractID, LastLedger: 499000}
	rpc := &fakeRPC{
		latestLedger: &LatestLedger{Sequence: 500000},
		events: map[string]*GetEventsResult{
			fixtureContractID: {
				Events: []RPCEvent{{
					ID:                       "0001-0001",
					ContractID:               fixtureContractID,
					Ledger:                   499100,
					LedgerClosedAt:           "2026-07-26T10:00:00Z",
					TxHash:                   "tx1",
					Type:                     "contract",
					Topic:                    eventTopics,
					Value:                    "AAAAAQ==",
					InSuccessfulContractCall: true,
				}},
				LatestLedger: 500000,
			},
		},
		transactions: map[string]*TransactionResult{"tx1": tx},
	}
	p := New(rpc, store, newFakeRedis(), cfg, testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("run: %v", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.invocations) != 1 {
		t.Fatalf("invocations: want 1, got %d", len(store.invocations))
	}
	return store.invocations[0]
}

func wantMetric(t *testing.T, name string, got *int64, want int64) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s: got %v, want %d", name, got, want)
	}
}

func TestPoller_PopulatesInvocationFromDiagnosticEvents(t *testing.T) {
	t.Parallel()
	f := loadDiagnosticsFixture(t)

	inv := runDiagnosticsPass(t, testConfig(), f.TransferTopics, &TransactionResult{
		Status:              "SUCCESS",
		Ledger:              499100,
		DiagnosticEventsXDR: f.GetTransaction.DiagnosticEventsXdr,
	})

	if inv.FunctionName != "transfer" {
		t.Fatalf("function name: got %q, want transfer", inv.FunctionName)
	}
	wantMetric(t, "cpu_insn", inv.CPUInsn, 2837152)
	wantMetric(t, "mem_byte", inv.MemByte, 1209434)
	wantMetric(t, "ledger_read_byte", inv.LedgerReadByte, 1324)
	wantMetric(t, "ledger_write_byte", inv.LedgerWriteByte, 412)
}

func TestPoller_FallsBackToFnCallTopicWithoutDiagnostics(t *testing.T) {
	t.Parallel()
	f := loadDiagnosticsFixture(t)

	inv := runDiagnosticsPass(t, testConfig(), f.FnCallTopics, &TransactionResult{
		Status: "SUCCESS",
		Ledger: 499100,
	})

	if inv.FunctionName != "transfer" {
		t.Fatalf("function name from fn_call fallback: got %q, want transfer", inv.FunctionName)
	}
	if inv.CPUInsn != nil || inv.MemByte != nil || inv.LedgerReadByte != nil || inv.LedgerWriteByte != nil {
		t.Fatalf("metrics must stay nil without diagnostic events, got %+v", inv)
	}
}

func TestPoller_FallbackIgnoresCallsIntoOtherContracts(t *testing.T) {
	t.Parallel()
	f := loadDiagnosticsFixture(t)

	inv := runDiagnosticsPass(t, testConfig(), f.NestedFnCallTopics, &TransactionResult{Status: "SUCCESS", Ledger: 499100})
	if inv.FunctionName != "" {
		t.Fatalf("fn_call into another contract must not name this invocation, got %q", inv.FunctionName)
	}
}

func TestPoller_FallbackCanBeDisabled(t *testing.T) {
	t.Parallel()
	f := loadDiagnosticsFixture(t)

	cfg := testConfig()
	cfg.DisableFnCallFallback = true
	inv := runDiagnosticsPass(t, cfg, f.FnCallTopics, &TransactionResult{Status: "SUCCESS", Ledger: 499100})
	if inv.FunctionName != "" {
		t.Fatalf("disabled fallback still set function name %q", inv.FunctionName)
	}
}

func TestPoller_MissingOrBadDiagnosticsDoNotFailThePass(t *testing.T) {
	t.Parallel()
	f := loadDiagnosticsFixture(t)

	for name, diag := range map[string][]string{
		"none":      nil,
		"malformed": {"!!not-xdr!!", "AAAA"},
	} {
		t.Run(name, func(t *testing.T) {
			inv := runDiagnosticsPass(t, testConfig(), f.TransferTopics, &TransactionResult{
				Status:              "SUCCESS",
				Ledger:              499100,
				DiagnosticEventsXDR: diag,
			})
			if inv.FunctionName != "" || inv.CPUInsn != nil {
				t.Fatalf("want empty diagnostics, got %+v", inv)
			}
			if inv.Status != "SUCCESS" {
				t.Fatalf("invocation should still be stored, got %+v", inv)
			}
		})
	}
}
