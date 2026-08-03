package store

import (
	"context"
	"errors"
	"time"
)

// MockStore is an in-memory Store + QueryStore implementation for unit tests.
type MockStore struct {
	contracts      map[string]Contract
	events         []Event
	invocations    []Invocation
	storageEntries []StorageEntry
	syncStates     map[string]SyncState
	globalStats    GlobalStats

	// Error injection
	UpsertContractErr   error
	GetContractErr      error
	ListContractsErr    error
	GetGlobalStatsErr   error
	ListEventsErr       error
	ListInvocationsErr  error
	ListStorageErr      error
	GetContractStatsErr error
	RecentEventsErr     error
}

// NewMockStore returns an initialized MockStore.
func NewMockStore() *MockStore {
	return &MockStore{
		contracts:  make(map[string]Contract),
		syncStates: make(map[string]SyncState),
	}
}

// ---- store.Store ------------------------------------------------------------

func (m *MockStore) UpsertContract(_ context.Context, c Contract) error {
	if m.UpsertContractErr != nil {
		return m.UpsertContractErr
	}
	if c.AddedAt.IsZero() {
		c.AddedAt = time.Now()
	}
	m.contracts[c.ID] = c
	return nil
}

func (m *MockStore) GetContract(_ context.Context, contractID string) (Contract, error) {
	if m.GetContractErr != nil {
		return Contract{}, m.GetContractErr
	}
	c, ok := m.contracts[contractID]
	if !ok {
		return Contract{}, ErrNotFound
	}
	return c, nil
}

func (m *MockStore) ListContracts(_ context.Context, cursor string, limit int) ([]Contract, string, error) {
	if m.ListContractsErr != nil {
		return nil, "", m.ListContractsErr
	}
	if limit <= 0 {
		limit = 50
	}
	var out []Contract
	for _, c := range m.contracts {
		if cursor == "" || c.ID > cursor {
			out = append(out, c)
		}
	}
	var nextCursor string
	if len(out) > limit {
		nextCursor = out[limit-1].ID
		out = out[:limit]
	}
	return out, nextCursor, nil
}

func (m *MockStore) BatchInsertEvents(_ context.Context, events []Event) error {
	m.events = append(m.events, events...)
	return nil
}

func (m *MockStore) BatchInsertInvocations(_ context.Context, invocations []Invocation) error {
	m.invocations = append(m.invocations, invocations...)
	return nil
}

func (m *MockStore) UpsertStorageEntries(_ context.Context, entries []StorageEntry) error {
	m.storageEntries = append(m.storageEntries, entries...)
	return nil
}

func (m *MockStore) GetSyncState(_ context.Context, contractID string) (SyncState, error) {
	ss, ok := m.syncStates[contractID]
	if !ok {
		return SyncState{ContractID: contractID}, nil
	}
	return ss, nil
}

func (m *MockStore) UpsertSyncState(_ context.Context, ss SyncState) error {
	m.syncStates[ss.ContractID] = ss
	return nil
}

func (m *MockStore) GetGlobalStats(_ context.Context) (GlobalStats, error) {
	if m.GetGlobalStatsErr != nil {
		return GlobalStats{}, m.GetGlobalStatsErr
	}
	return m.globalStats, nil
}

// SetGlobalStats lets tests control what GetGlobalStats returns.
func (m *MockStore) SetGlobalStats(gs GlobalStats) {
	m.globalStats = gs
}

// ---- store.QueryStore -------------------------------------------------------

func (m *MockStore) ListEvents(_ context.Context, contractID, cursor string, limit int, f EventFilters) ([]Event, string, error) {
	if m.ListEventsErr != nil {
		return nil, "", m.ListEventsErr
	}
	if limit <= 0 {
		limit = 50
	}
	var out []Event
	for _, e := range m.events {
		if e.ContractID != contractID {
			continue
		}
		if cursor != "" && e.ID <= cursor {
			continue
		}
		if f.Type != "" && e.Type != f.Type {
			continue
		}
		out = append(out, e)
		if len(out) > limit {
			break
		}
	}
	var nextCursor string
	if len(out) > limit {
		nextCursor = out[limit-1].ID
		out = out[:limit]
	}
	return out, nextCursor, nil
}

func (m *MockStore) ListInvocations(_ context.Context, contractID, cursor string, limit int, _ InvocationFilters) ([]Invocation, string, error) {
	if m.ListInvocationsErr != nil {
		return nil, "", m.ListInvocationsErr
	}
	if limit <= 0 {
		limit = 50
	}
	var out []Invocation
	for _, inv := range m.invocations {
		if inv.ContractID != contractID {
			continue
		}
		if cursor != "" && inv.TxHash <= cursor {
			continue
		}
		out = append(out, inv)
		if len(out) > limit {
			break
		}
	}
	var nextCursor string
	if len(out) > limit {
		nextCursor = out[limit-1].TxHash
		out = out[:limit]
	}
	return out, nextCursor, nil
}

func (m *MockStore) ListStorageEntries(_ context.Context, contractID, cursor string, limit int, _ StorageFilters) ([]StorageEntry, string, error) {
	if m.ListStorageErr != nil {
		return nil, "", m.ListStorageErr
	}
	if limit <= 0 {
		limit = 50
	}
	var out []StorageEntry
	for _, se := range m.storageEntries {
		if se.ContractID != contractID {
			continue
		}
		if cursor != "" && se.KeyXDR <= cursor {
			continue
		}
		out = append(out, se)
		if len(out) > limit {
			break
		}
	}
	var nextCursor string
	if len(out) > limit {
		nextCursor = out[limit-1].KeyXDR
		out = out[:limit]
	}
	return out, nextCursor, nil
}

func (m *MockStore) GetContractStats(_ context.Context, contractID, window string) (ContractStats, error) {
	if m.GetContractStatsErr != nil {
		return ContractStats{}, m.GetContractStatsErr
	}
	var cs ContractStats
	cs.WindowDuration = window
	if cs.WindowDuration == "" {
		cs.WindowDuration = "24h"
	}
	for _, e := range m.events {
		if e.ContractID == contractID {
			cs.EventCount++
			cs.WindowEventCount++
		}
	}
	for _, inv := range m.invocations {
		if inv.ContractID == contractID {
			cs.InvocationCount++
			cs.WindowInvocationCount++
		}
	}
	for _, se := range m.storageEntries {
		if se.ContractID == contractID {
			cs.StorageCount++
		}
	}
	return cs, nil
}

func (m *MockStore) RecentEvents(_ context.Context, contractID string, limit int) ([]Event, error) {
	if m.RecentEventsErr != nil {
		return nil, m.RecentEventsErr
	}
	if limit <= 0 {
		limit = 20
	}
	var out []Event
	for i := len(m.events) - 1; i >= 0 && len(out) < limit; i-- {
		if m.events[i].ContractID == contractID {
			out = append(out, m.events[i])
		}
	}
	return out, nil
}

// ErrPing is returned by MockPinger when Healthy is false.
var ErrPing = errors.New("mock: ping failed")

// MockPinger implements handler.Pinger for tests.
type MockPinger struct {
	Healthy bool
}

func (p *MockPinger) Ping(_ context.Context) error {
	if !p.Healthy {
		return ErrPing
	}
	return nil
}
