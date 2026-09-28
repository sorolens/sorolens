package store

import (
	"context"
	"sort"
	"time"
)

// ---- WatchedAccountStore ----------------------------------------------------

func (m *MockStore) AddWatchedAccount(_ context.Context, a WatchedAccount) (WatchedAccount, bool, error) {
	if existing, ok := m.watchedAccounts[a.AccountID]; ok {
		return existing, false, nil
	}
	a.DiscoveredCount = 0
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	m.watchedAccounts[a.AccountID] = a
	return a, true, nil
}

func (m *MockStore) ListWatchedAccounts(_ context.Context) ([]WatchedAccount, error) {
	out := make([]WatchedAccount, 0, len(m.watchedAccounts))
	for _, a := range m.watchedAccounts {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].AccountID < out[j].AccountID
	})
	return out, nil
}

func (m *MockStore) DeleteWatchedAccount(_ context.Context, accountID string) error {
	if _, ok := m.watchedAccounts[accountID]; !ok {
		return ErrNotFound
	}
	delete(m.watchedAccounts, accountID)
	return nil
}

func (m *MockStore) RecordDiscoveredContract(_ context.Context, d DiscoveredContract) (bool, error) {
	if _, ok := m.contracts[d.ContractID]; ok {
		return false, nil
	}
	m.contracts[d.ContractID] = Contract{
		ID:              d.ContractID,
		Network:         networkOrDefault(d.Network),
		Label:           DiscoveredByLabel(d.AccountID),
		CreatedAtLedger: d.Ledger,
		Status:          "active",
		AddedAt:         time.Now().UTC(),
	}
	if d.Ledger > 1 {
		m.syncStates[d.ContractID] = SyncState{ContractID: d.ContractID, LastLedger: uint32(d.Ledger - 1)}
	}
	if a, ok := m.watchedAccounts[d.AccountID]; ok {
		a.DiscoveredCount++
		m.watchedAccounts[d.AccountID] = a
	}
	return true, nil
}

// ---- AuditStore -------------------------------------------------------------

func (m *MockStore) InsertAuditEvent(_ context.Context, e AuditEvent) error {
	if m.InsertAuditErr != nil {
		return m.InsertAuditErr
	}
	m.auditMu.Lock()
	defer m.auditMu.Unlock()
	e.ID = int64(len(m.auditEvents) + 1)
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	m.auditEvents = append(m.auditEvents, e)
	return nil
}

func (m *MockStore) ListAuditEvents(_ context.Context, since time.Time, cursor int64, limit int) ([]AuditEvent, int64, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	m.auditMu.Lock()
	defer m.auditMu.Unlock()
	var out []AuditEvent
	for i := len(m.auditEvents) - 1; i >= 0; i-- {
		e := m.auditEvents[i]
		if cursor != 0 && e.ID >= cursor {
			continue
		}
		if !since.IsZero() && e.At.Before(since) {
			continue
		}
		out = append(out, e)
	}
	var next int64
	if len(out) > limit {
		next = out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}

// AuditEvents returns a snapshot of every recorded audit row, oldest first.
func (m *MockStore) AuditEvents() []AuditEvent {
	m.auditMu.Lock()
	defer m.auditMu.Unlock()
	return append([]AuditEvent(nil), m.auditEvents...)
}
