package store

import (
	"context"
	"sort"
)

// In-memory ContractNoteStore implementation for MockStore.

func (m *MockStore) CreateContractNote(_ context.Context, n ContractNote) error {
	if m.contractNotes == nil {
		m.contractNotes = make([]ContractNote, 0)
	}
	m.contractNotes = append(m.contractNotes, n)
	return nil
}

func (m *MockStore) ListContractNotes(_ context.Context, contractID string, limit int) ([]ContractNote, error) {
	if limit <= 0 {
		limit = 100
	}
	out := make([]ContractNote, 0)
	for _, n := range m.contractNotes {
		if n.ContractID == contractID {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MockStore) GetContractNote(_ context.Context, id string) (ContractNote, error) {
	for _, n := range m.contractNotes {
		if n.ID == id {
			return n, nil
		}
	}
	return ContractNote{}, ErrNotFound
}

func (m *MockStore) DeleteContractNote(_ context.Context, contractID, id string) error {
	for i, n := range m.contractNotes {
		if n.ID == id && n.ContractID == contractID {
			m.contractNotes = append(m.contractNotes[:i], m.contractNotes[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}
