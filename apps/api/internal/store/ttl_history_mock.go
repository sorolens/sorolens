package store

import (
	"context"
	"sort"
	"sync"
)

// MockTTLHistoryStore is an in-memory TTLHistoryStore for tests.
type MockTTLHistoryStore struct {
	mu  sync.Mutex
	ops []TTLOperation
	seq int
}

func newMockTTLHistoryStore() *MockTTLHistoryStore {
	return &MockTTLHistoryStore{}
}

func (m *MockTTLHistoryStore) RecordTTLOperation(_ context.Context, op TTLOperation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// idempotency: skip duplicates
	for _, existing := range m.ops {
		if existing.ContractID == op.ContractID && existing.TxHash == op.TxHash && existing.KeyXDR == op.KeyXDR {
			return nil
		}
	}
	m.seq++
	op.ID = op.ContractID + "-" + op.TxHash
	m.ops = append(m.ops, op)
	return nil
}

func (m *MockTTLHistoryStore) ListTTLHistory(_ context.Context, contractID, cursor string, limit int) ([]TTLOperation, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}
	var matched []TTLOperation
	for _, op := range m.ops {
		if op.ContractID != contractID {
			continue
		}
		if cursor != "" && op.ID >= cursor {
			continue
		}
		matched = append(matched, op)
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].ID > matched[j].ID })
	var nextCursor string
	if len(matched) > limit {
		nextCursor = matched[limit-1].ID
		matched = matched[:limit]
	}
	return matched, nextCursor, nil
}

// embed into MockStore
func (m *MockStore) RecordTTLOperation(ctx context.Context, op TTLOperation) error {
	return m.ttlHistory().RecordTTLOperation(ctx, op)
}
func (m *MockStore) ListTTLHistory(ctx context.Context, contractID, cursor string, limit int) ([]TTLOperation, string, error) {
	return m.ttlHistory().ListTTLHistory(ctx, contractID, cursor, limit)
}

func (m *MockStore) ttlHistory() *MockTTLHistoryStore {
	if m.ttlHistoryStore == nil {
		m.ttlHistoryStore = newMockTTLHistoryStore()
	}
	return m.ttlHistoryStore
}
