package store

import (
	"context"
	"time"
)

// This file holds the in-memory implementation of the ContractNoteStore
// surface (issue #164). It is kept out of mock.go so the feature stays
// reviewable as a unit, the same way the LiveStore and GroupStore mocks are.

// ---- store.ContractNoteStore (in-memory) ------------------------------------

func (m *MockStore) AddContractNote(_ context.Context, contractID, author, body string) (ContractNote, error) {
	if m.contractNotes == nil {
		m.contractNotes = make(map[string][]ContractNote)
	}
	now := time.Now().UTC()
	note := ContractNote{
		ID:         newUUID(),
		ContractID: contractID,
		Author:     author,
		Body:       body,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	m.contractNotes[contractID] = append(m.contractNotes[contractID], note)
	return note, nil
}

func (m *MockStore) GetContractNote(_ context.Context, contractID, noteID string) (ContractNote, error) {
	for _, n := range m.contractNotes[contractID] {
		if n.ID == noteID {
			return n, nil
		}
	}
	return ContractNote{}, ErrNotFound
}

func (m *MockStore) ListContractNotes(_ context.Context, contractID string) ([]ContractNote, error) {
	stored := m.contractNotes[contractID]
	out := make([]ContractNote, 0, len(stored))
	// Notes are appended in the order they were written, so walking the slice
	// backwards yields newest first without relying on timestamp resolution.
	for i := len(stored) - 1; i >= 0; i-- {
		out = append(out, stored[i])
	}
	return out, nil
}

func (m *MockStore) UpdateContractNote(_ context.Context, contractID, noteID, body string) (ContractNote, error) {
	stored := m.contractNotes[contractID]
	for i, n := range stored {
		if n.ID != noteID {
			continue
		}
		// The author and the creation time are carried over: editing a note
		// rewrites its body, nothing else.
		n.Body = body
		n.UpdatedAt = time.Now().UTC()
		stored[i] = n
		return n, nil
	}
	return ContractNote{}, ErrNotFound
}

func (m *MockStore) DeleteContractNote(_ context.Context, contractID, noteID string) error {
	stored := m.contractNotes[contractID]
	kept := make([]ContractNote, 0, len(stored))
	for _, n := range stored {
		if n.ID == noteID {
			continue
		}
		kept = append(kept, n)
	}
	if len(kept) == 0 {
		// Deleting the last note drops the contract's entry so the map does not
		// accumulate empty slices. delete on a nil map is a no-op.
		delete(m.contractNotes, contractID)
		return nil
	}
	m.contractNotes[contractID] = kept
	return nil
}
