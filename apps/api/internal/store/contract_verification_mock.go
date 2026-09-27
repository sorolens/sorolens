package store

import (
	"context"
	"time"
)

// Contract verification in-memory implementation for MockStore.

func (m *MockStore) UpsertContractVerification(_ context.Context, v ContractVerification) error {
	if m.UpsertVerificationErr != nil {
		return m.UpsertVerificationErr
	}
	if m.contractVerifications == nil {
		m.contractVerifications = make(map[string]ContractVerification)
	}
	if v.UpdatedAt.IsZero() {
		v.UpdatedAt = time.Now().UTC()
	}
	m.contractVerifications[v.ContractID] = v
	return nil
}

func (m *MockStore) GetContractVerification(_ context.Context, contractID string) (ContractVerification, error) {
	if m.GetVerificationErr != nil {
		return ContractVerification{}, m.GetVerificationErr
	}
	v, ok := m.contractVerifications[contractID]
	if !ok {
		return ContractVerification{}, ErrNotFound
	}
	return v, nil
}
