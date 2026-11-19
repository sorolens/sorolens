package callgraph

import (
	"encoding/base64"
)

// Summary is the transaction-level view of a diagnostic event stream: the
// function the transaction invoked at the top level and the host's resource
// counters for the whole invocation (issue #504). It fills the
// function_name, cpu_insn, mem_byte, ledger_read_byte and ledger_write_byte
// columns of the invocations table.
//
// Every counter is a pointer so "the node did not report it" (nil) stays
// distinct from a genuine zero.
type Summary struct {
	// ContractID is the strkey of the root invocation's contract, when the
	// fn_call event carried it.
	ContractID string
	// FunctionName is the root invocation's function symbol.
	FunctionName string

	CPUInsn         *int64
	MemByte         *int64
	LedgerReadByte  *int64
	LedgerWriteByte *int64
}

// HasMetrics reports whether any resource counter was found.
func (s Summary) HasMetrics() bool {
	return s.CPUInsn != nil || s.MemByte != nil || s.LedgerReadByte != nil || s.LedgerWriteByte != nil
}

// Empty reports whether the stream yielded nothing usable at all.
func (s Summary) Empty() bool {
	return s.FunctionName == "" && !s.HasMetrics()
}

// Summarize extracts the root function name and the transaction's resource
// counters from the base64 diagnostic events returned by getTransaction.
//
// The host emits one core_metrics event per counter once the invocation
// finishes, e.g. topics ["core_metrics", "cpu_insn"] with a u64 payload. Older
// hosts packed the counter into a single-entry map payload instead; both shapes
// are accepted. When a counter is reported more than once the last value wins,
// matching the host's "final total" semantics.
//
// Ledger bytes prefer the aggregate ledger_read_byte / ledger_write_byte
// counters. Hosts that only report the split key/data/code byte counters get
// those summed instead.
//
// Like Parse, Summarize never fails: an empty or undecodable stream yields an
// empty Summary and the caller keeps indexing without these fields.
func Summarize(diagnosticEventsXDR []string) Summary {
	var (
		s        Summary
		rootSeen bool
		counters = make(map[string]int64)
	)
	for _, raw := range diagnosticEventsXDR {
		ev, ok := decodeDiagnosticEvent(raw)
		if !ok || ev.typ != contractEventDiagnostic {
			continue
		}
		switch ev.topicName() {
		case topicFnCall:
			if rootSeen {
				continue
			}
			rootSeen = true
			s.ContractID, s.FunctionName = ev.callTarget()
		case topicCoreMetrics:
			if name := firstSymbol(ev.topics[1:]); name != "" && ev.data.hasNum {
				counters[name] = ev.data.num
			}
			for k, v := range ev.data.entries {
				if v.hasNum {
					counters[k] = v.num
				}
			}
		}
	}

	s.CPUInsn = counter(counters, "cpu_insn")
	s.MemByte = counter(counters, "mem_byte")
	s.LedgerReadByte = counter(counters, "ledger_read_byte")
	if s.LedgerReadByte == nil {
		s.LedgerReadByte = sumCounters(counters, "read_key_byte", "read_data_byte", "read_code_byte")
	}
	s.LedgerWriteByte = counter(counters, "ledger_write_byte")
	if s.LedgerWriteByte == nil {
		s.LedgerWriteByte = sumCounters(counters, "write_key_byte", "write_data_byte", "write_code_byte")
	}
	return s
}

// FunctionNameFromTopics returns the function symbol of an fn_call event given
// its base64 SCVal topics, as they appear on a getEvents result. It is the
// fallback for nodes that do not return diagnosticEventsXdr on getTransaction
// but do surface fn_call events alongside contract events (ROADMAP v0.2).
//
// contractID, when non-empty, restricts the match to calls into that contract
// so a nested call into another contract is not mistaken for the root.
func FunctionNameFromTopics(topics []string, contractID string) (string, bool) {
	if len(topics) < 2 {
		return "", false
	}
	vals := make([]scval, 0, len(topics))
	for _, t := range topics {
		raw, err := base64.StdEncoding.DecodeString(t)
		if err != nil {
			return "", false
		}
		v, ok := decodeScVal(&cursor{buf: raw})
		if !ok {
			return "", false
		}
		vals = append(vals, v)
	}
	ev := diagnosticEvent{typ: contractEventDiagnostic, topics: vals}
	if ev.topicName() != topicFnCall {
		return "", false
	}
	contract, fn := ev.callTarget()
	if fn == "" {
		return "", false
	}
	if contractID != "" && contract != "" && contract != contractID {
		return "", false
	}
	return fn, true
}

func counter(m map[string]int64, name string) *int64 {
	v, ok := m[name]
	if !ok {
		return nil
	}
	return &v
}

func sumCounters(m map[string]int64, names ...string) *int64 {
	var (
		total int64
		found bool
	)
	for _, n := range names {
		if v, ok := m[n]; ok {
			total += v
			found = true
		}
	}
	if !found {
		return nil
	}
	return &total
}
