package poller

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sorolens/sorolens/services/indexer/internal/contractspec"
	"github.com/sorolens/sorolens/services/indexer/internal/wasm"
)

// ContractSpec is the cached, parsed SEP-48 interface description for one
// contract. Spec is the JSON tree produced by the contractspec package
// ({"functions":[...]}).
type ContractSpec struct {
	ContractID string
	Spec       []byte
	WasmHash   string
}

// ContractSpecStore is the optional persistence surface for cached contract
// specs (issue #130).
//
// It is asserted from Store at runtime rather than folded into the Store
// interface so that every existing Store implementation — and the test fakes
// in this package — keep compiling; a Store that does not implement it simply
// skips spec caching.
type ContractSpecStore interface {
	// GetContractSpec returns the cached spec and whether one exists.
	GetContractSpec(ctx context.Context, contractID string) (ContractSpec, bool, error)
	// UpsertContractSpec stores or replaces the cached spec.
	UpsertContractSpec(ctx context.Context, spec ContractSpec) error
}

// cacheContractSpec fetches, parses, and caches a contract's SEP-48 interface
// spec the first time the contract is indexed.
//
// It is best-effort by design, matching the acceptance criterion that a
// parsing failure "logs a warning and does not break indexing": every failure
// mode — an unsupported store, a cache lookup error, a missing Wasm entry, a
// contract with no spec section, or a store write error — logs and returns
// without surfacing an error to the caller.
//
// The work runs only once per contract (subsequent passes see the cached row
// and return immediately), at the cost of two extra getLedgerEntries calls on
// that first pass: one for the contract instance (to resolve the Wasm hash)
// and one for the ContractCode entry holding the Wasm itself.
func (p *Poller) cacheContractSpec(ctx context.Context, rpc RPCClient, contract Contract) {
	ss, ok := p.store.(ContractSpecStore)
	if !ok {
		return
	}

	if _, found, err := ss.GetContractSpec(ctx, contract.ID); err != nil {
		p.log.Warn("spec: cache lookup failed (continuing)",
			"contract_id", contract.ID, "err", err)
		return
	} else if found {
		return
	}

	wasmHash, err := p.currentWasmHash(ctx, rpc, contract.ID)
	if err != nil {
		p.log.Warn("spec: resolve wasm hash failed (continuing)",
			"contract_id", contract.ID, "err", err)
		return
	}

	code, err := p.fetchWasmCode(ctx, rpc, wasmHash)
	if err != nil {
		p.log.Warn("spec: fetch wasm failed (continuing)",
			"contract_id", contract.ID, "wasm_hash", wasmHash, "err", err)
		return
	}

	parsed, err := contractspec.ParseWasm(code)
	if err != nil {
		// A contract compiled without `contractspecv0` (or with a section a
		// future protocol extends unexpectedly) is not an indexing failure.
		p.log.Warn("spec: parse failed (continuing)",
			"contract_id", contract.ID, "wasm_hash", wasmHash, "err", err)
		return
	}

	raw, err := json.Marshal(parsed)
	if err != nil {
		p.log.Warn("spec: marshal failed (continuing)",
			"contract_id", contract.ID, "err", err)
		return
	}

	if err := ss.UpsertContractSpec(ctx, ContractSpec{
		ContractID: contract.ID,
		Spec:       raw,
		WasmHash:   wasmHash,
	}); err != nil {
		p.log.Warn("spec: store failed (continuing)",
			"contract_id", contract.ID, "err", err)
		return
	}

	p.log.Info("spec: cached contract interface",
		"contract_id", contract.ID,
		"wasm_hash", wasmHash,
		"functions", len(parsed.Functions),
	)
}

// currentWasmHash resolves the contract's current on-chain Wasm hash from its
// instance entry.
func (p *Poller) currentWasmHash(ctx context.Context, rpc RPCClient, contractID string) (string, error) {
	key, err := wasm.ContractInstanceKey(contractID)
	if err != nil {
		return "", fmt.Errorf("build instance key: %w", err)
	}

	res, err := rpc.GetLedgerEntries(ctx, []string{key})
	if err != nil {
		return "", fmt.Errorf("get instance entry: %w", err)
	}
	if res == nil {
		return "", fmt.Errorf("get instance entry: empty result")
	}
	for _, entry := range res.Entries {
		if hash, ok := wasm.WasmHashFromInstanceEntry(entry.XDR); ok {
			return hash, nil
		}
	}
	return "", fmt.Errorf("instance entry for %s carries no wasm hash", contractID)
}

// fetchWasmCode resolves the Wasm bytecode for a Wasm hash via its
// ContractCode ledger entry.
func (p *Poller) fetchWasmCode(ctx context.Context, rpc RPCClient, wasmHash string) ([]byte, error) {
	key, err := wasm.ContractCodeKey(wasmHash)
	if err != nil {
		return nil, fmt.Errorf("build code key: %w", err)
	}

	res, err := rpc.GetLedgerEntries(ctx, []string{key})
	if err != nil {
		return nil, fmt.Errorf("get code entry: %w", err)
	}
	if res == nil {
		return nil, fmt.Errorf("get code entry: empty result")
	}
	for _, entry := range res.Entries {
		if code, ok := wasm.WasmCodeFromCodeEntry(entry.XDR); ok {
			return code, nil
		}
	}
	return nil, fmt.Errorf("no contract code for wasm hash %s", wasmHash)
}
