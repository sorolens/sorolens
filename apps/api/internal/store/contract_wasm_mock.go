package store

import (
	"context"
	"time"
)

// Contract Wasm in-memory implementation for MockStore.

func (m *MockStore) UpsertContractWasm(_ context.Context, wasmHash string, code []byte) error {
	if m.wasmBinaries == nil {
		m.wasmBinaries = make(map[string]ContractWasm)
	}
	if _, ok := m.wasmBinaries[wasmHash]; ok {
		return nil
	}
	cp := append([]byte(nil), code...)
	m.wasmBinaries[wasmHash] = ContractWasm{
		WasmHash:  wasmHash,
		Code:      cp,
		SizeBytes: len(cp),
		FetchedAt: time.Now().UTC(),
	}
	return nil
}

func (m *MockStore) HasContractWasm(_ context.Context, wasmHash string) (bool, error) {
	if m.wasmBinaries == nil {
		return false, nil
	}
	_, ok := m.wasmBinaries[wasmHash]
	return ok, nil
}

func (m *MockStore) GetContractWasmByHash(_ context.Context, wasmHash string) (ContractWasm, error) {
	if m.GetWasmErr != nil {
		return ContractWasm{}, m.GetWasmErr
	}
	w, ok := m.wasmBinaries[wasmHash]
	if !ok {
		return ContractWasm{}, ErrNotFound
	}
	return w, nil
}

func (m *MockStore) GetContractWasm(ctx context.Context, contractID string) (ContractWasm, error) {
	if m.GetWasmErr != nil {
		return ContractWasm{}, m.GetWasmErr
	}
	c, err := m.GetContract(ctx, contractID)
	if err != nil {
		return ContractWasm{}, err
	}
	if c.WasmHash == "" {
		return ContractWasm{}, ErrNotFound
	}
	return m.GetContractWasmByHash(ctx, c.WasmHash)
}

func (m *MockStore) UpdateContractWasmHash(_ context.Context, contractID, wasmHash string) error {
	c, ok := m.contracts[contractID]
	if !ok {
		return ErrNotFound
	}
	c.WasmHash = wasmHash
	m.contracts[contractID] = c
	return nil
}
