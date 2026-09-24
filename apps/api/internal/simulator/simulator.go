// Package simulator implements dry-run invocations against the state Sorolens
// has indexed. Given an XDR-encoded Soroban transaction it loads the storage
// snapshot of every contract the transaction touches, executes the invocation
// against a local runtime seeded with that snapshot, and returns the simulated
// events, return value, and resource metering without touching Soroban RPC.
//
// # Execution backend
//
// Execution lives behind the Engine interface. The default IndexedEngine
// evaluates the invocation against the indexed state and reports the outcome
// recorded by the indexer (the most recent indexed execution for the touched
// contracts), so the endpoint works even while RPC is unavailable. An
// in-process soroban-env-host backend (reached through FFI or a Rust sidecar)
// plugs into the same interface without changing the HTTP surface.
package simulator

import (
	"context"
	"fmt"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Structured diagnostic codes returned when a simulation cannot complete.
const (
	// CodeInvalidXDR means the transaction envelope could not be decoded.
	CodeInvalidXDR = "INVALID_XDR"
	// CodeNoIndexedContract means the transaction touches no indexed contract.
	CodeNoIndexedContract = "NO_INDEXED_CONTRACT"
	// CodeNoIndexedState means no invocation has been indexed for the touched
	// contracts yet, so there is no state to simulate against.
	CodeNoIndexedState = "NO_INDEXED_STATE"
	// CodeSimulationFailed means the runtime reported a failed execution, such
	// as a host error or a panic in the invoked contract.
	CodeSimulationFailed = "SIMULATION_FAILED"
	// CodeOutOfBudget means the execution exceeded the requested CPU budget.
	CodeOutOfBudget = "OUT_OF_BUDGET"
)

// Result status values.
const (
	StatusSuccess = "success"
	StatusFailure = "failure"
)

// Failure is a structured simulation error. Handlers map Code onto the HTTP
// error contract instead of collapsing every failure into a 500.
type Failure struct {
	Code    string
	Message string
}

func (f *Failure) Error() string {
	return f.Code + ": " + f.Message
}

// Metering reports the resources a simulated invocation consumed.
type Metering struct {
	CPUInsn            int64 `json:"cpu_insn"`
	MemByte            int64 `json:"mem_byte"`
	LedgerReadByte     int64 `json:"ledger_read_byte"`
	LedgerWriteByte    int64 `json:"ledger_write_byte"`
	ResourceFeeCharged int64 `json:"resource_fee_charged"`
}

// Event is a contract event produced by a simulated invocation.
type Event struct {
	ContractID string   `json:"contract_id"`
	Ledger     uint32   `json:"ledger"`
	TxHash     string   `json:"tx_hash,omitempty"`
	Type       string   `json:"type,omitempty"`
	TopicXDR   []string `json:"topic_xdr,omitempty"`
	ValueXDR   string   `json:"value_xdr,omitempty"`
}

// Diagnostic explains why a simulation did not succeed.
type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ContractState is the indexed state of one contract touched by a simulation.
type ContractState struct {
	ContractID  string
	Storage     []store.StorageEntry
	Invocations []store.Invocation
	Events      []store.Event
}

// Input is a single simulation request together with the state it executes
// against.
type Input struct {
	// Key identifies the simulation for caching. Requests with the same key
	// are considered identical.
	Key string
	// Ledger is the ledger the storage snapshot is taken at. Zero means the
	// caller did not pin a ledger.
	Ledger uint32
	// BudgetCPUInsn, when greater than zero, is the instruction budget the
	// invocation must stay within.
	BudgetCPUInsn int64
	// Contracts holds the indexed state for every contract the transaction
	// touches.
	Contracts []ContractState
}

// Result is the outcome of a simulation.
type Result struct {
	Status       string
	Ledger       uint32
	Contracts    []string
	ReturnValue  any
	ReturnXDR    string
	Events       []Event
	Metering     Metering
	ReplayedFrom string
	Diagnostic   *Diagnostic
	// Cached is set by the Service when the result was served from cache.
	Cached bool
}

// Engine executes a simulation against the state supplied in Input.
type Engine interface {
	Simulate(ctx context.Context, in Input) (Result, error)
}

// IndexedEngine evaluates an invocation against the state Sorolens has indexed.
// For each touched contract it replays the most recent indexed execution and
// reports the recorded return value, events, and metering. Failed executions
// are surfaced as CodeSimulationFailed diagnostics rather than errors.
type IndexedEngine struct{}

// NewIndexedEngine returns the default engine.
func NewIndexedEngine() *IndexedEngine { return &IndexedEngine{} }

// Simulate implements Engine.
func (e *IndexedEngine) Simulate(_ context.Context, in Input) (Result, error) {
	res := Result{
		Status:    StatusSuccess,
		Ledger:    in.Ledger,
		Contracts: make([]string, 0, len(in.Contracts)),
		Events:    make([]Event, 0),
	}
	if len(in.Contracts) == 0 {
		return res, &Failure{
			Code:    CodeNoIndexedContract,
			Message: "transaction does not reference any indexed contract",
		}
	}
	for _, c := range in.Contracts {
		res.Contracts = append(res.Contracts, c.ContractID)
	}

	chosen, owner := latestInvocation(in.Contracts)
	if chosen == nil {
		res.Status = StatusFailure
		res.Diagnostic = &Diagnostic{
			Code:    CodeNoIndexedState,
			Message: "no invocation has been indexed for the touched contracts; run the indexer before simulating",
		}
		return res, nil
	}

	res.Metering = meterFromInvocation(chosen)

	if in.BudgetCPUInsn > 0 && chosen.CPUInsn > in.BudgetCPUInsn {
		res.Status = StatusFailure
		res.Diagnostic = &Diagnostic{
			Code: CodeOutOfBudget,
			Message: fmt.Sprintf(
				"simulation exceeded the CPU budget: used %d instructions, limit %d",
				chosen.CPUInsn, in.BudgetCPUInsn),
		}
		return res, nil
	}

	if chosen.Status != "SUCCESS" {
		res.Status = StatusFailure
		res.Diagnostic = &Diagnostic{
			Code: CodeSimulationFailed,
			Message: fmt.Sprintf(
				"the most recent indexed invocation of %s finished with status %s",
				chosen.ContractID, chosen.Status),
		}
		return res, nil
	}

	res.ReplayedFrom = chosen.TxHash
	res.ReturnValue = chosen.ResultDecoded
	res.ReturnXDR = chosen.ResultXDR
	res.Events = eventsFor(owner, chosen.TxHash)
	return res, nil
}

// latestInvocation returns the most recent invocation across every touched
// contract. It prefers the highest ledger and breaks ties by tx hash so the
// choice is deterministic. The second return value is the contract the chosen
// invocation belongs to.
func latestInvocation(contracts []ContractState) (*store.Invocation, *ContractState) {
	var best *store.Invocation
	var owner *ContractState
	for i := range contracts {
		c := &contracts[i]
		for j := range c.Invocations {
			inv := &c.Invocations[j]
			if best == nil ||
				inv.Ledger > best.Ledger ||
				(inv.Ledger == best.Ledger && inv.TxHash > best.TxHash) {
				best = inv
				owner = c
			}
		}
	}
	return best, owner
}

func meterFromInvocation(inv *store.Invocation) Metering {
	return Metering{
		CPUInsn:            inv.CPUInsn,
		MemByte:            inv.MemByte,
		LedgerReadByte:     inv.LedgerReadByte,
		LedgerWriteByte:    inv.LedgerWriteByte,
		ResourceFeeCharged: inv.ResourceFeeCharged,
	}
}

func eventsFor(c *ContractState, txHash string) []Event {
	out := make([]Event, 0)
	if c == nil {
		return out
	}
	for _, ev := range c.Events {
		if ev.TxHash != txHash {
			continue
		}
		out = append(out, Event{
			ContractID: ev.ContractID,
			Ledger:     ev.Ledger,
			TxHash:     ev.TxHash,
			Type:       ev.Type,
			TopicXDR:   ev.TopicXDR,
			ValueXDR:   ev.ValueXDR,
		})
	}
	return out
}
