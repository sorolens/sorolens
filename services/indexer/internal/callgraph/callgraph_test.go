package callgraph

import (
	"encoding/base64"
	"encoding/binary"
	"testing"
)

// ---- XDR fixture builders -------------------------------------------------
//
// These encode the same wire layout the decoder reads. They are the mirror image
// of the production decoder, which keeps the tests self-consistent; the strkey
// test below is anchored to a published Stellar value instead.

func be32(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

func be64(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

func xdrVarOpaque(b []byte) []byte {
	out := append(be32(uint32(len(b))), b...)
	if pad := (4 - len(b)%4) % 4; pad > 0 {
		out = append(out, make([]byte, pad)...)
	}
	return out
}

func scSym(s string) []byte { return append(be32(scvSymbol), xdrVarOpaque([]byte(s))...) }

func scVoid() []byte { return be32(scvVoid) }

func scU64(v uint64) []byte { return append(be32(scvU64), be64(v)...) }

func scVec(vals ...[]byte) []byte {
	out := append(be32(scvVec), be32(1)...)
	out = append(out, be32(uint32(len(vals)))...)
	for _, v := range vals {
		out = append(out, v...)
	}
	return out
}

func scMap(keys, vals [][]byte) []byte {
	out := append(be32(scvMap), be32(1)...)
	out = append(out, be32(uint32(len(keys)))...)
	for i := range keys {
		out = append(out, keys[i]...)
		out = append(out, vals[i]...)
	}
	return out
}

func scContract(id [32]byte) []byte {
	out := append(be32(scvAddress), be32(scAddressContract)...)
	return append(out, id[:]...)
}

type diagOpts struct {
	contractEventType uint32
	topics            [][]byte
	data              []byte
}

func encodeDiagEvent(o diagOpts) string {
	var out []byte
	out = append(out, be32(1)...) // inSuccessfulContractCall
	out = append(out, be32(0)...) // ContractEvent.ext
	out = append(out, be32(0)...) // ContractEvent.contractID (absent)
	out = append(out, be32(o.contractEventType)...)
	out = append(out, be32(0)...) // body union: v0
	out = append(out, be32(uint32(len(o.topics)))...)
	for _, t := range o.topics {
		out = append(out, t...)
	}
	out = append(out, o.data...)
	return base64.StdEncoding.EncodeToString(out)
}

func fnCall(contract [32]byte, fn string) string {
	return encodeDiagEvent(diagOpts{
		contractEventType: contractEventDiagnostic,
		topics:            [][]byte{scSym(topicFnCall), scContract(contract), scSym(fn)},
		data:              scVec(),
	})
}

func fnReturn(fn string) string {
	return encodeDiagEvent(diagOpts{
		contractEventType: contractEventDiagnostic,
		topics:            [][]byte{scSym(topicFnReturn), scSym(fn)},
		data:              scVoid(),
	})
}

func coreMetrics(name string, value uint64) string {
	return encodeDiagEvent(diagOpts{
		contractEventType: contractEventDiagnostic,
		topics:            [][]byte{scSym(topicCoreMetrics)},
		data:              scMap([][]byte{scSym(name)}, [][]byte{scU64(value)}),
	})
}

func contractID(b byte) [32]byte {
	var id [32]byte
	for i := range id {
		id[i] = b
	}
	return id
}

// ---- tests ----------------------------------------------------------------

func TestParse_EmptyStreamYieldsNoEdges(t *testing.T) {
	t.Parallel()
	if got := Parse("tx", nil); len(got) != 0 {
		t.Fatalf("expected no edges, got %+v", got)
	}
}

func TestParse_RootCallOnlyYieldsNoEdges(t *testing.T) {
	t.Parallel()

	// A transaction that invokes a single contract with no cross-contract
	// calls: the root lives in the invocations table, so there is nothing to
	// materialise in call_edges.
	events := []string{
		fnCall(contractID(0xAA), "increment"),
		fnReturn("increment"),
	}

	if got := Parse("tx", events); len(got) != 0 {
		t.Fatalf("want no edges for a root-only call, got %+v", got)
	}
}

func TestParse_DirectChildCall(t *testing.T) {
	t.Parallel()

	root := contractID(0xAA)
	callee := contractID(0xBB)
	events := []string{
		fnCall(root, "swap"),
		fnCall(callee, "transfer"),
		fnReturn("transfer"),
		fnReturn("swap"),
	}

	edges := Parse("tx1", events)
	if len(edges) != 1 {
		t.Fatalf("want 1 edge, got %d: %+v", len(edges), edges)
	}

	want := Edge{
		TxHash:           "tx1",
		ParentSpanID:     RootSpanID,
		ChildSpanID:      "0.0",
		CalleeContractID: contractStrkey(callee[:]),
		FunctionName:     "transfer",
		Depth:            1,
	}
	if edges[0] != want {
		t.Errorf("edge mismatch:\n got %+v\nwant %+v", edges[0], want)
	}
	if edges[0].CalleeContractID == "" || edges[0].CalleeContractID[0] != 'C' {
		t.Errorf("callee contract id %q is not a contract strkey", edges[0].CalleeContractID)
	}
}

func TestParse_NestedCallsAndSiblings(t *testing.T) {
	t.Parallel()

	root := contractID(0x01)
	caller := contractID(0x02)
	leaf := contractID(0x03)
	sibling := contractID(0x04)

	events := []string{
		fnCall(root, "orchestrate"),
		fnCall(caller, "middle"),
		fnCall(leaf, "leaf_fn"),
		fnReturn("leaf_fn"),
		fnReturn("middle"),
		fnCall(sibling, "other_top_level"),
		fnReturn("other_top_level"),
		fnReturn("orchestrate"),
	}

	edges := Parse("tx2", events)
	if len(edges) != 3 {
		t.Fatalf("want 3 edges, got %d: %+v", len(edges), edges)
	}

	// Path-based span ids: the leaf sits under "0.0", the second top-level
	// frame is "0.1" (sibling of the first, not a child).
	bySpan := make(map[string]Edge, len(edges))
	for _, e := range edges {
		bySpan[e.ChildSpanID] = e
	}

	mid, ok := bySpan["0.0"]
	if !ok {
		t.Fatalf("missing edge 0.0 in %+v", edges)
	}
	if mid.ParentSpanID != "0" || mid.Depth != 1 || mid.FunctionName != "middle" {
		t.Errorf("unexpected 0.0 edge: %+v", mid)
	}

	deep, ok := bySpan["0.0.0"]
	if !ok {
		t.Fatalf("missing edge 0.0.0 in %+v", edges)
	}
	if deep.ParentSpanID != "0.0" || deep.Depth != 2 || deep.FunctionName != "leaf_fn" {
		t.Errorf("unexpected 0.0.0 edge: %+v", deep)
	}
	if deep.CalleeContractID != contractStrkey(leaf[:]) {
		t.Errorf("leaf callee = %q, want %q", deep.CalleeContractID, contractStrkey(leaf[:]))
	}

	sib, ok := bySpan["0.1"]
	if !ok {
		t.Fatalf("missing edge 0.1 in %+v", edges)
	}
	if sib.ParentSpanID != "0" || sib.Depth != 1 || sib.FunctionName != "other_top_level" {
		t.Errorf("unexpected 0.1 edge: %+v", sib)
	}
}

func TestParse_DeterministicAcrossRuns(t *testing.T) {
	t.Parallel()

	events := []string{
		fnCall(contractID(0x01), "root"),
		fnCall(contractID(0x02), "a"),
		fnReturn("a"),
		fnCall(contractID(0x03), "b"),
		fnReturn("b"),
		fnReturn("root"),
	}

	first := Parse("tx3", events)
	second := Parse("tx3", events)
	if len(first) != len(second) {
		t.Fatalf("edge count differs between runs: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("edge %d differs between runs: %+v vs %+v", i, first[i], second[i])
		}
	}
}

func TestParse_CoreMetricsAttributedToOpenFrame(t *testing.T) {
	t.Parallel()

	events := []string{
		fnCall(contractID(0x01), "root"),
		fnCall(contractID(0x02), "child"),
		coreMetrics("cpu_insn", 12345),
		coreMetrics("mem_byte", 678),
		fnReturn("child"),
		fnReturn("root"),
	}

	edges := Parse("tx4", events)
	if len(edges) != 1 {
		t.Fatalf("want 1 edge, got %+v", edges)
	}
	if edges[0].CPU != 12345 || edges[0].Mem != 678 {
		t.Errorf("resource counters not attributed: %+v", edges[0])
	}
}

func TestParse_CoreMetricsAfterReturnAttributedToClosingFrame(t *testing.T) {
	t.Parallel()

	// Some protocol versions emit the frame's counters immediately *after*
	// its fn_return; they must still land on the frame that just closed and
	// not on the still-open parent.
	events := []string{
		fnCall(contractID(0x01), "root"),
		fnCall(contractID(0x02), "child"),
		fnReturn("child"),
		coreMetrics("cpu_insn", 42),
		fnReturn("root"),
	}

	edges := Parse("tx5", events)
	if len(edges) != 1 {
		t.Fatalf("want 1 edge, got %+v", edges)
	}
	if edges[0].CPU != 42 {
		t.Errorf("cpu = %d, want 42", edges[0].CPU)
	}
}

func TestParse_IgnoresNonDiagnosticEvents(t *testing.T) {
	t.Parallel()

	// Contract (type 1) and system (type 0) events must not be read as frames.
	contractEvent := encodeDiagEvent(diagOpts{
		contractEventType: 1,
		topics:            [][]byte{scSym(topicFnCall), scContract(contractID(0xEE)), scSym("bogus")},
		data:              scVec(),
	})

	events := []string{
		fnCall(contractID(0x01), "root"),
		contractEvent,
		fnReturn("root"),
	}
	if got := Parse("tx6", events); len(got) != 0 {
		t.Fatalf("non-diagnostic events produced edges: %+v", got)
	}
}

func TestParse_MalformedInputYieldsNoEdges(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"not base64":        {"!!!! not base64 !!!!"},
		"truncated":         {base64.StdEncoding.EncodeToString([]byte{0, 0, 0})},
		"empty string":      {""},
		"garbage after hdr": {base64.StdEncoding.EncodeToString(be32(1))},
		"unknown ext":       {base64.StdEncoding.EncodeToString(append(append(be32(1), be32(9)...), be32(0)...))},
		"fn_call bad topic": {encodeDiagEvent(diagOpts{contractEventDiagnostic, [][]byte{scSym(topicFnCall), be32(99)}, scVoid()})},
	}

	for name, events := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Must not panic; may legitimately produce no edges.
			_ = Parse("tx7", events)
		})
	}
}

func TestParse_DepthIsBounded(t *testing.T) {
	t.Parallel()

	events := []string{fnCall(contractID(0x01), "root")}
	// maxDepth+10 nested frames.
	for i := 0; i < maxDepth+10; i++ {
		events = append(events, fnCall(contractID(byte(i%250+1)), "f"))
	}
	for i := 0; i < maxDepth+11; i++ {
		events = append(events, fnReturn("f"))
	}

	edges := Parse("tx8", events)
	if len(edges) > maxDepth {
		t.Errorf("edge count %d exceeds the depth bound %d", len(edges), maxDepth)
	}
	for _, e := range edges {
		if e.Depth > maxDepth {
			t.Errorf("edge depth %d exceeds the bound %d: %+v", e.Depth, maxDepth, e)
		}
	}
}

func TestDistributeFee_ProportionalToCPU(t *testing.T) {
	t.Parallel()

	edges := []Edge{
		{ChildSpanID: "0.0", Depth: 1, CPU: 100},
		{ChildSpanID: "0.1", Depth: 1, CPU: 300},
		{ChildSpanID: "0.0.0", Depth: 2, CPU: 100},
	}

	got := DistributeFee(1000, edges)
	if got[0].FeeShare != 250 {
		t.Errorf("0.0 fee share = %d, want 250", got[0].FeeShare)
	}
	if got[1].FeeShare != 750 {
		t.Errorf("0.1 fee share = %d, want 750", got[1].FeeShare)
	}
	if got[2].FeeShare != 0 {
		t.Errorf("nested edge must not receive a share, got %d", got[2].FeeShare)
	}
	if total := got[0].FeeShare + got[1].FeeShare; total != 1000 {
		t.Errorf("shares sum to %d, want the full 1000", total)
	}
}

func TestDistributeFee_EvenSplitWhenCPUUnknown(t *testing.T) {
	t.Parallel()

	edges := []Edge{
		{ChildSpanID: "0.0", Depth: 1},
		{ChildSpanID: "0.1", Depth: 1},
		{ChildSpanID: "0.2", Depth: 1},
	}

	got := DistributeFee(100, edges)
	total := int64(0)
	for _, e := range got {
		if e.FeeShare < 33 || e.FeeShare > 34 {
			t.Errorf("unexpected even share %d", e.FeeShare)
		}
		total += e.FeeShare
	}
	if total != 100 {
		t.Errorf("shares sum to %d, want 100", total)
	}
}

func TestDistributeFee_NoOpCases(t *testing.T) {
	t.Parallel()

	edges := []Edge{{ChildSpanID: "0.0", Depth: 1}}
	if got := DistributeFee(0, edges); got[0].FeeShare != 0 {
		t.Errorf("zero fee must not be distributed, got %d", got[0].FeeShare)
	}
	if got := DistributeFee(1000, nil); got != nil {
		t.Errorf("nil edges must be returned unchanged, got %+v", got)
	}
	nested := []Edge{{ChildSpanID: "0.0.0", Depth: 2}}
	got := DistributeFee(1000, nested)
	if got[0].FeeShare != 0 {
		t.Errorf("a tree with no top-level edge must not distribute, got %d", got[0].FeeShare)
	}
}

func TestContractStrkey_MatchesPublishedValue(t *testing.T) {
	t.Parallel()

	// The all-zero contract id is the published decoding example of the
	// official Stellar strkey library (stellar/rs-stellar-strkey,
	// docs.rs/stellar-strkey), and pins both the base32 encoding and the
	// CRC-16/XMODEM checksum.
	const want = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4"
	if got := contractStrkey(make([]byte, 32)); got != want {
		t.Fatalf("contractStrkey(zeros) = %q, want %q", got, want)
	}
}

func TestContractStrkey_RejectsWrongLength(t *testing.T) {
	t.Parallel()
	if got := contractStrkey([]byte{1, 2, 3}); got != "" {
		t.Errorf("expected empty string for a short id, got %q", got)
	}
}

func TestCrc16XModem_KnownVector(t *testing.T) {
	t.Parallel()

	// "123456789" is the canonical CRC-16/XMODEM check value.
	if got := crc16XModem([]byte("123456789")); got != 0x31C3 {
		t.Errorf("crc16XModem(\"123456789\") = 0x%04X, want 0x31C3", got)
	}
}
