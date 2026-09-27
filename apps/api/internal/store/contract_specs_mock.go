package store

import "context"

// UpsertContractSpec implements ContractSpecStore for the in-memory mock.
func (m *MockStore) UpsertContractSpec(_ context.Context, spec ContractSpec) error {
	if m.UpsertContractSpecErr != nil {
		return m.UpsertContractSpecErr
	}
	if m.contractSpecs == nil {
		m.contractSpecs = make(map[string]ContractSpec)
	}
	m.contractSpecs[spec.ContractID] = spec
	return nil
}

// GetContractSpec implements ContractSpecStore for the in-memory mock.
func (m *MockStore) GetContractSpec(_ context.Context, contractID string) (ContractSpec, error) {
	if m.GetContractSpecErr != nil {
		return ContractSpec{}, m.GetContractSpecErr
	}
	spec, ok := m.contractSpecs[contractID]
	if !ok {
		return ContractSpec{}, ErrNotFound
	}
	return spec, nil
}
