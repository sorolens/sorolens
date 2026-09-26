// Package callgraph reconstructs the cross-contract call tree of a Soroban
// transaction from the host diagnostic event stream, so the dashboard can render
// a full invocation trace instead of only the top-level invocation.
//
// # Source data
//
// The host emits one diagnostic event per executed frame. The events we use are:
//
//	fn_call       topics ["fn_call", <contract address>, <function symbol>]
//	fn_return     topics ["fn_return", <function symbol>]
//	core_metrics  topics ["core_metrics", ...], data carrying a single
//	              resource counter (cpu_insn, mem_byte, ...)
//
// They are exposed by the Soroban RPC as `diagnosticEventsXdr` on getTransaction,
// which is the same stream as `TransactionMetaV3.sorobanMeta.diagnosticEvents`
// inside `resultMetaXdr` (see RESEARCH.md section 4). The RPC only populates the
// top-level array when the node has ENABLE_SOROBAN_DIAGNOSTIC_EVENTS enabled;
// when the stream is empty the parser yields no edges and the indexer simply
// records the top-level invocation as it does today.
//
// # Conventions
//
// Consistent with the rest of the monorepo (apps/api/internal/soroban/scval.go,
// services/indexer/internal/wasm), this package hand-decodes the minimal XDR it
// needs instead of importing the stellar/go SDK. Decoding is defensive: every
// discriminant is validated, array lengths are bounded, and any mismatch makes
// the event (or the whole parse) yield nothing rather than panic or fail a poll
// pass. Recursion depth and the number of emitted edges are capped.
//
// # Span ids
//
// Span ids are deterministic call-path strings: the root invocation is "0" (its
// row lives in the invocations table, not in call_edges) and the nth child of a
// frame is "<parent>.<n>". Re-parsing the same transaction therefore produces
// identical ids, which is what makes re-indexing and the historical backfill
// idempotent.
package callgraph

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strconv"
)

// RootSpanID is the span id of a transaction's root invocation. The root row is
// stored in the invocations table; call_edges only carries the frames below it.
const RootSpanID = "0"

const (
	// maxDepth caps the reconstructed call depth so a malformed or looping
	// diagnostic stream cannot exhaust memory.
	maxDepth = 64
	// maxEdges caps the number of edges produced for one transaction.
	maxEdges = 1024
	// maxTopics and maxVecLen bound the XDR arrays we walk.
	maxTopics = 64
	maxVecLen = 4096
	maxMapLen = 256
)

// Diagnostic event topic names emitted by the Soroban host.
const (
	topicFnCall      = "fn_call"
	topicFnReturn    = "fn_return"
	topicCoreMetrics = "core_metrics"
)

// ContractEventType values (XDR). Only DIAGNOSTIC events carry the call tree.
const contractEventDiagnostic = 2

// SCAddressType values (XDR).
const (
	scAddressAccount  = 0
	scAddressContract = 1
)

// SCValType discriminants (XDR, Protocol 20+).
const (
	scvBool      = 0
	scvVoid      = 1
	scvU32       = 3
	scvI32       = 4
	scvU64       = 5
	scvI64       = 6
	scvTimePoint = 7
	scvDuration  = 8
	scvU128      = 9
	scvI128      = 10
	scvU256      = 11
	scvI256      = 12
	scvBytes     = 13
	scvString    = 14
	scvSymbol    = 15
	scvVec       = 16
	scvMap       = 17
	scvAddress   = 18

	// contractVersionByte is the strkey version byte for contract ids
	// (2 << 3), which is what makes an encoded id start with "C".
	contractVersionByte = 2 << 3
)

// Edge is one parent -> child invocation edge inside a transaction. It mirrors
// the store.CallEdge row written to the call_edges table.
type Edge struct {
	TxHash           string
	ParentSpanID     string
	ChildSpanID      string
	CalleeContractID string
	FunctionName     string
	CPU              int64
	Mem              int64
	FeeShare         int64
	Depth            int
}

// Parse reconstructs the call graph below a transaction's root invocation from
// the base64 diagnostic events returned by the Soroban RPC.
//
// It never returns an error: a transaction without diagnostic events, or with a
// stream this decoder does not understand, yields no edges. Callers must treat
// that as "no cross-contract calls recorded" and continue indexing.
func Parse(txHash string, diagnosticEventsXDR []string) []Edge {
	var (
		frames []*frame
		// created keeps every frame ever pushed so its counters can be folded
		// back onto the matching edge at the end.
		created []*frame
		// owner is the frame the next core_metrics event belongs to: the frame
		// most recently entered or left. Diagnostic counters are emitted at the
		// end of a frame's body on some protocol versions and immediately after
		// its fn_return on others, so neither "top of stack" nor "last closed"
		// alone is correct.
		owner      *frame
		childCount = make(map[string]int)
		edges      []Edge
		rootSeen   bool
	)

	for _, raw := range diagnosticEventsXDR {
		ev, ok := decodeDiagnosticEvent(raw)
		if !ok || ev.typ != contractEventDiagnostic {
			continue
		}

		switch ev.topicName() {
		case topicFnCall:
			contract, fn := ev.callTarget()
			if !rootSeen {
				// The first fn_call is the transaction's root invocation.
				// It is represented by the invocations row, not by an edge.
				rootSeen = true
				f := &frame{spanID: RootSpanID, contract: contract, fn: fn, edgeIdx: -1}
				frames = append(frames, f)
				created = append(created, f)
				owner = f
				continue
			}
			if len(frames) == 0 {
				// fn_call with no open frame: the stream is truncated or the
				// root was filtered out. Skip rather than invent a parent.
				continue
			}
			parent := frames[len(frames)-1]
			idx := childCount[parent.spanID]
			childCount[parent.spanID] = idx + 1
			if len(frames) >= maxDepth || len(edges) >= maxEdges {
				// Still push the frame so its fn_return is matched, but drop
				// the edge: the tree is intentionally truncated.
				f := &frame{spanID: parent.spanID + "." + strconv.Itoa(idx), edgeIdx: -1}
				frames = append(frames, f)
				created = append(created, f)
				owner = f
				continue
			}
			edge := Edge{
				TxHash:           txHash,
				ParentSpanID:     parent.spanID,
				ChildSpanID:      parent.spanID + "." + strconv.Itoa(idx),
				CalleeContractID: contract,
				FunctionName:     fn,
				Depth:            len(frames),
			}
			edges = append(edges, edge)
			f := &frame{
				spanID:   edge.ChildSpanID,
				contract: contract,
				fn:       fn,
				edgeIdx:  len(edges) - 1,
			}
			frames = append(frames, f)
			created = append(created, f)
			owner = f

		case topicFnReturn:
			f := popFrame(&frames)
			if f != nil {
				owner = f
			}

		case topicCoreMetrics:
			// core_metrics is emitted per frame; see the owner field above for
			// why the frame is tracked rather than read off the stack here.
			if owner != nil {
				applyCoreMetrics(owner, ev)
			}
		}
	}

	// Fold the per-frame resource counters back onto their edges.
	for _, f := range created {
		if f.edgeIdx < 0 || f.edgeIdx >= len(edges) {
			continue
		}
		edges[f.edgeIdx].CPU = f.cpu
		edges[f.edgeIdx].Mem = f.mem
	}
	return edges
}

// DistributeFee attributes rootFee stroops of resource fee to the transaction's
// top-level edges (depth 1), proportionally to their CPU, falling back to an
// even split when no CPU is known. Sub-call frames inherit their ancestor's
// share implicitly through the tree, so only direct children of the root are
// weighted.
//
// The input slice is returned with FeeShare populated; rootFee <= 0 or a
// transaction with no top-level edges leaves every FeeShare at 0.
func DistributeFee(rootFee int64, edges []Edge) []Edge {
	if rootFee <= 0 || len(edges) == 0 {
		return edges
	}

	var top []int
	var totalCPU int64
	for i := range edges {
		if edges[i].Depth != 1 {
			continue
		}
		top = append(top, i)
		if edges[i].CPU > 0 {
			totalCPU += edges[i].CPU
		}
	}
	if len(top) == 0 {
		return edges
	}

	var remaining = rootFee
	for n, i := range top {
		var share int64
		if n == len(top)-1 {
			// The last child absorbs the rounding remainder so the shares add
			// up to exactly rootFee.
			share = remaining
		} else if totalCPU > 0 {
			share = rootFee * edges[i].CPU / totalCPU
		} else {
			share = rootFee / int64(len(top))
		}
		if share < 0 {
			share = 0
		}
		edges[i].FeeShare = share
		remaining -= share
	}
	return edges
}

// ---- tree building helpers -------------------------------------------------

type frame struct {
	spanID   string
	contract string
	fn       string
	edgeIdx  int
	cpu      int64
	mem      int64
}

func popFrame(frames *[]*frame) *frame {
	if len(*frames) == 0 {
		return nil
	}
	f := (*frames)[len(*frames)-1]
	*frames = (*frames)[:len(*frames)-1]
	return f
}

func applyCoreMetrics(f *frame, ev diagnosticEvent) {
	// The counter name is either a second topic symbol or, when the host packs
	// the counter into the data, the single map key.
	if name := firstSymbol(ev.topics[1:]); name != "" && ev.data.hasNum {
		addMetric(f, name, ev.data.num)
	}
	for k, v := range ev.data.entries {
		if v.hasNum {
			addMetric(f, k, v.num)
		}
	}
}

func addMetric(f *frame, name string, value int64) {
	switch name {
	case "cpu_insn":
		f.cpu = value
	case "mem_byte":
		f.mem = value
	}
}

func firstSymbol(topics []scval) string {
	for _, t := range topics {
		if t.typ == scvSymbol && t.sym != "" {
			return t.sym
		}
	}
	return ""
}

// ---- diagnostic event decoding --------------------------------------------

type diagnosticEvent struct {
	typ    uint32
	topics []scval
	data   scval
}

func (e diagnosticEvent) topicName() string {
	if len(e.topics) == 0 {
		return ""
	}
	t := e.topics[0]
	if t.typ != scvSymbol {
		return ""
	}
	return t.sym
}

// callTarget extracts the callee contract id and function name from an fn_call
// event's topics, tolerating both the documented shape
// ["fn_call", address, symbol] and topic orders where the address is absent.
func (e diagnosticEvent) callTarget() (contract, fn string) {
	for _, t := range e.topics[1:] {
		if contract == "" && t.typ == scvAddress && t.contract != "" {
			contract = t.contract
			continue
		}
		if fn == "" && t.typ == scvSymbol {
			fn = t.sym
		}
	}
	return contract, fn
}

// decodeDiagnosticEvent decodes the base64 XDR of one xdr.DiagnosticEvent:
//
//	struct DiagnosticEvent { bool inSuccessfulContractCall; ContractEvent event; }
//	struct ContractEvent {
//	    ExtensionPoint ext;
//	    Hash* contractID;
//	    ContractEventType type;
//	    union switch (int v) { case 0: struct { SCVal topics<>; SCVal data; } v0; } body;
//	};
func decodeDiagnosticEvent(b64 string) (diagnosticEvent, bool) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return diagnosticEvent{}, false
	}
	c := &cursor{buf: raw}

	// inSuccessfulContractCall (bool is a 4-byte enum in XDR).
	if v, ok := c.u32(); !ok || v > 1 {
		return diagnosticEvent{}, false
	}
	// ContractEvent.ext (ExtensionPoint): only the void case is understood.
	if v, ok := c.u32(); !ok || v != 0 {
		return diagnosticEvent{}, false
	}
	// ContractEvent.contractID (optional Hash).
	switch present, ok := c.u32(); {
	case !ok:
		return diagnosticEvent{}, false
	case present == 1:
		if _, ok := c.fixed(32); !ok {
			return diagnosticEvent{}, false
		}
	case present != 0:
		return diagnosticEvent{}, false
	}
	typ, ok := c.u32()
	if !ok {
		return diagnosticEvent{}, false
	}
	// ContractEvent.body union discriminant.
	if v, ok := c.u32(); !ok || v != 0 {
		return diagnosticEvent{}, false
	}
	n, ok := c.u32()
	if !ok || n > maxTopics {
		return diagnosticEvent{}, false
	}
	ev := diagnosticEvent{typ: typ, topics: make([]scval, 0, n)}
	for i := uint32(0); i < n; i++ {
		t, ok := decodeScVal(c)
		if !ok {
			return diagnosticEvent{}, false
		}
		ev.topics = append(ev.topics, t)
	}
	// The payload is only needed for core_metrics. A payload this decoder does
	// not understand must not invalidate the topics we already have.
	if d, ok := decodeScVal(c); ok {
		ev.data = d
	}
	return ev, true
}

// ---- SCVal decoding -------------------------------------------------------

type scval struct {
	typ      uint32
	sym      string
	contract string
	num      int64
	hasNum   bool
	entries  map[string]scval
}

// decodeScVal decodes one xdr.SCVal. It supports the primitive types plus
// vectors, maps and addresses; anything else reports ok=false so the caller can
// abandon the event instead of misreading the rest of the buffer.
func decodeScVal(c *cursor) (scval, bool) {
	disc, ok := c.u32()
	if !ok {
		return scval{}, false
	}
	switch disc {
	case scvVoid:
		return scval{typ: disc}, true

	case scvBool:
		v, ok := c.u32()
		if !ok || v > 1 {
			return scval{}, false
		}
		return scval{typ: disc}, true

	case scvU32:
		v, ok := c.u32()
		if !ok {
			return scval{}, false
		}
		return scval{typ: disc, num: int64(v), hasNum: true}, true

	case scvI32:
		v, ok := c.u32()
		if !ok {
			return scval{}, false
		}
		return scval{typ: disc, num: int64(int32(v)), hasNum: true}, true

	case scvU64, scvTimePoint, scvDuration:
		v, ok := c.u64()
		if !ok {
			return scval{}, false
		}
		if v > 1<<62 {
			// Larger than any counter we care about; keep it out of int64.
			return scval{typ: disc}, true
		}
		return scval{typ: disc, num: int64(v), hasNum: true}, true

	case scvI64:
		v, ok := c.u64()
		if !ok {
			return scval{}, false
		}
		return scval{typ: disc, num: int64(v), hasNum: true}, true

	case scvU128, scvI128, scvU256, scvI256:
		size := 16
		if disc == scvU256 || disc == scvI256 {
			size = 32
		}
		if _, ok := c.fixed(size); !ok {
			return scval{}, false
		}
		return scval{typ: disc}, true

	case scvBytes:
		if _, ok := c.varOpaque(); !ok {
			return scval{}, false
		}
		return scval{typ: disc}, true

	case scvString, scvSymbol:
		s, ok := c.varOpaque()
		if !ok {
			return scval{}, false
		}
		return scval{typ: disc, sym: string(s)}, true

	case scvVec:
		present, ok := c.u32()
		if !ok {
			return scval{}, false
		}
		if present == 0 {
			return scval{typ: disc}, true
		}
		if present != 1 {
			return scval{}, false
		}
		n, ok := c.u32()
		if !ok || n > maxVecLen {
			return scval{}, false
		}
		for i := uint32(0); i < n; i++ {
			if _, ok := decodeScVal(c); !ok {
				return scval{}, false
			}
		}
		return scval{typ: disc}, true

	case scvMap:
		present, ok := c.u32()
		if !ok {
			return scval{}, false
		}
		if present == 0 {
			return scval{typ: disc}, true
		}
		if present != 1 {
			return scval{}, false
		}
		n, ok := c.u32()
		if !ok || n > maxMapLen {
			return scval{}, false
		}
		out := scval{typ: disc, entries: make(map[string]scval, n)}
		for i := uint32(0); i < n; i++ {
			k, ok := decodeScVal(c)
			if !ok {
				return scval{}, false
			}
			v, ok := decodeScVal(c)
			if !ok {
				return scval{}, false
			}
			if k.typ == scvSymbol && k.sym != "" {
				out.entries[k.sym] = v
			}
		}
		return out, true

	case scvAddress:
		return decodeAddress(c)

	default:
		return scval{}, false
	}
}

func decodeAddress(c *cursor) (scval, bool) {
	kind, ok := c.u32()
	if !ok {
		return scval{}, false
	}
	switch kind {
	case scAddressAccount:
		// PublicKey union discriminant, then a 32-byte key.
		pk, ok := c.u32()
		if !ok || pk > 1 {
			return scval{}, false
		}
		if _, ok := c.fixed(32); !ok {
			return scval{}, false
		}
		return scval{typ: scvAddress}, true

	case scAddressContract:
		raw, ok := c.fixed(32)
		if !ok {
			return scval{}, false
		}
		return scval{typ: scvAddress, contract: contractStrkey(raw)}, true

	default:
		return scval{}, false
	}
}

// ---- strkey ---------------------------------------------------------------

// contractStrkey encodes 32 raw contract-id bytes as a Stellar "C..." strkey so
// call_edges.callee_contract_id joins the contracts and invocations tables, which
// both store strkeys.
func contractStrkey(raw []byte) string {
	if len(raw) != 32 {
		return ""
	}
	payload := make([]byte, 0, 35)
	payload = append(payload, contractVersionByte)
	payload = append(payload, raw...)
	payload = binary.LittleEndian.AppendUint16(payload, crc16XModem(payload))
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(payload)
}

// crc16XModem is the CRC-16/XMODEM (poly 0x1021, init 0x0000) checksum Stellar
// strkeys append to their payload.
func crc16XModem(b []byte) uint16 {
	var crc uint16
	for _, x := range b {
		crc ^= uint16(x) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// ---- tiny XDR cursor ------------------------------------------------------

// cursor walks a big-endian XDR buffer. Every read is bounds-checked and
// reports ok=false on a short buffer.
type cursor struct {
	buf []byte
	off int
}

func (c *cursor) u32() (uint32, bool) {
	if len(c.buf)-c.off < 4 {
		return 0, false
	}
	v := binary.BigEndian.Uint32(c.buf[c.off : c.off+4])
	c.off += 4
	return v, true
}

func (c *cursor) u64() (uint64, bool) {
	if len(c.buf)-c.off < 8 {
		return 0, false
	}
	v := binary.BigEndian.Uint64(c.buf[c.off : c.off+8])
	c.off += 8
	return v, true
}

func (c *cursor) fixed(n int) ([]byte, bool) {
	if len(c.buf)-c.off < n {
		return nil, false
	}
	out := c.buf[c.off : c.off+n]
	c.off += n
	return out, true
}

// varOpaque reads an XDR variable-length opaque/string: a 4-byte length followed
// by that many bytes, padded to a 4-byte boundary.
func (c *cursor) varOpaque() ([]byte, bool) {
	n, ok := c.u32()
	if !ok || n > maxVecLen {
		return nil, false
	}
	body, ok := c.fixed(int(n))
	if !ok {
		return nil, false
	}
	if pad := (4 - int(n)%4) % 4; pad > 0 {
		if _, ok := c.fixed(pad); !ok {
			return nil, false
		}
	}
	return body, true
}

// String renders the edge for logging.
func (e Edge) String() string {
	return fmt.Sprintf("%s %s->%s %s/%s", e.TxHash, e.ParentSpanID, e.ChildSpanID, e.CalleeContractID, e.FunctionName)
}
