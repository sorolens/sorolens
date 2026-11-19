package callgraph

import (
	"encoding/json"
	"os"
	"testing"
)

// diagnosticFixture is testdata/diagnostic_events.json. Its XDR was encoded
// with github.com/stellar/go-stellar-sdk/xdr rather than with this package's
// test helpers, so these tests check the hand-written decoder against an
// independent encoder of the real wire format (issue #504).
type diagnosticFixture struct {
	GetTransaction struct {
		DiagnosticEventsXdr []string `json:"diagnosticEventsXdr"`
	} `json:"getTransaction"`
	SplitByteCounters  []string `json:"splitByteCounters"`
	FnCallTopics       []string `json:"fnCallTopics"`
	NestedFnCallTopics []string `json:"nestedFnCallTopics"`
	TransferTopics     []string `json:"transferTopics"`
}

func loadDiagnosticFixture(t *testing.T) diagnosticFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/diagnostic_events.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var f diagnosticFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return f
}

// fixtureRootContract is the strkey of the 0x11-filled contract id used as
// the root invocation target in the fixture.
var fixtureRootContract = contractStrkey(func() []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = 0x11
	}
	return b
}())

func wantCounter(t *testing.T, name string, got *int64, want int64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: got nil, want %d", name, want)
	}
	if *got != want {
		t.Fatalf("%s: got %d, want %d", name, *got, want)
	}
}

func TestSummarize_RealDiagnosticStream(t *testing.T) {
	f := loadDiagnosticFixture(t)
	s := Summarize(f.GetTransaction.DiagnosticEventsXdr)

	if s.FunctionName != "transfer" {
		t.Fatalf("function name: got %q, want transfer (the root call, not the nested on_transfer)", s.FunctionName)
	}
	if s.ContractID != fixtureRootContract {
		t.Fatalf("contract: got %q, want %q", s.ContractID, fixtureRootContract)
	}
	wantCounter(t, "cpu_insn", s.CPUInsn, 2837152)
	wantCounter(t, "mem_byte", s.MemByte, 1209434)
	wantCounter(t, "ledger_read_byte", s.LedgerReadByte, 1324)
	wantCounter(t, "ledger_write_byte", s.LedgerWriteByte, 412)
}

func TestSummarize_SumsSplitByteCountersWhenAggregatesMissing(t *testing.T) {
	f := loadDiagnosticFixture(t)
	s := Summarize(f.SplitByteCounters)

	// read_key + read_data + read_code = 288 + 236 + 800
	wantCounter(t, "ledger_read_byte", s.LedgerReadByte, 1324)
	// write_key + write_data + write_code = 144 + 268 + 0
	wantCounter(t, "ledger_write_byte", s.LedgerWriteByte, 412)
	wantCounter(t, "cpu_insn", s.CPUInsn, 2837152)
}

func TestSummarize_LegacyMapPayload(t *testing.T) {
	stream := []string{
		fnCall(contractID(0xAA), "swap"),
		coreMetrics("cpu_insn", 99),
		coreMetrics("mem_byte", 7),
	}
	s := Summarize(stream)
	if s.FunctionName != "swap" {
		t.Fatalf("function name: got %q", s.FunctionName)
	}
	wantCounter(t, "cpu_insn", s.CPUInsn, 99)
	wantCounter(t, "mem_byte", s.MemByte, 7)
	if s.LedgerReadByte != nil || s.LedgerWriteByte != nil {
		t.Fatalf("unreported ledger bytes must stay nil, got %v / %v", s.LedgerReadByte, s.LedgerWriteByte)
	}
}

func TestSummarize_EmptyAndMalformedStreams(t *testing.T) {
	for name, stream := range map[string][]string{
		"nil":       nil,
		"empty":     {},
		"garbage":   {"not base64!!", "AAAA"},
		"truncated": {fnCall(contractID(1), "x")[:12]},
	} {
		t.Run(name, func(t *testing.T) {
			s := Summarize(stream)
			if !s.Empty() {
				t.Fatalf("want empty summary, got %+v", s)
			}
		})
	}
}

func TestSummarize_IgnoresNonDiagnosticEvents(t *testing.T) {
	stream := []string{encodeDiagEvent(diagOpts{
		contractEventType: 1, // CONTRACT, not DIAGNOSTIC
		topics:            [][]byte{scSym(topicFnCall), scContract(contractID(1)), scSym("evil")},
		data:              scVec(),
	})}
	if s := Summarize(stream); !s.Empty() {
		t.Fatalf("contract events must not feed the summary, got %+v", s)
	}
}

func TestFunctionNameFromTopics(t *testing.T) {
	f := loadDiagnosticFixture(t)

	if fn, ok := FunctionNameFromTopics(f.FnCallTopics, ""); !ok || fn != "transfer" {
		t.Fatalf("fn_call topics: got %q, %v", fn, ok)
	}
	if fn, ok := FunctionNameFromTopics(f.FnCallTopics, fixtureRootContract); !ok || fn != "transfer" {
		t.Fatalf("fn_call topics scoped to root contract: got %q, %v", fn, ok)
	}
	if fn, ok := FunctionNameFromTopics(f.NestedFnCallTopics, fixtureRootContract); ok {
		t.Fatalf("nested call into another contract must not match, got %q", fn)
	}
	if fn, ok := FunctionNameFromTopics(f.TransferTopics, ""); ok {
		t.Fatalf("a plain contract event is not an fn_call, got %q", fn)
	}
	if _, ok := FunctionNameFromTopics([]string{"%%%", "x"}, ""); ok {
		t.Fatal("undecodable topics must not match")
	}
	if _, ok := FunctionNameFromTopics(nil, ""); ok {
		t.Fatal("no topics must not match")
	}
}
