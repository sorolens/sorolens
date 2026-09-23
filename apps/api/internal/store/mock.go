package store

import (
	"context"
	"errors"
	"sort"
	"time"
)

// MockStore is an in-memory Store + QueryStore implementation for unit tests.
type MockStore struct {
	contracts          map[string]Contract
	events             []Event
	invocations        []Invocation
	storageEntries     []StorageEntry
	syncStates         map[string]SyncState
	globalStats        GlobalStats
	monitored          map[string]MonitoredContract
	healthChecks       []HealthCheck
	alerts             []ContractAlert
	apiKeys            []APIKey
	watchlist          map[string]map[string]bool
	alertSubscriptions []AlertSubscription

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
	CreateAPIKeyErr     error
	GetAPIKeyErr        error
}

// NewMockStore returns an initialized MockStore.
func NewMockStore() *MockStore {
	return &MockStore{
		contracts:  make(map[string]Contract),
		syncStates: make(map[string]SyncState),
		monitored:  make(map[string]MonitoredContract),
		watchlist:  make(map[string]map[string]bool),
		alerts:     make([]ContractAlert, 0),
		alertSubscriptions: make([]AlertSubscription, 0),
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

func (m *MockStore) ListContracts(_ context.Context, cursor string, limit int, f ContractFilters) ([]Contract, string, error) {
	if m.ListContractsErr != nil {
		return nil, "", m.ListContractsErr
	}
	if limit <= 0 {
		limit = 50
	}
	var out []Contract
	for _, c := range m.contracts {
		if cursor != "" && c.ID <= cursor {
			continue
		}
		if f.Network != "" && c.Network != f.Network {
			continue
		}
		if f.Status != "" && c.Status != f.Status {
			continue
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
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

func (m *MockStore) CreateNextMonthPartition(_ context.Context) error { return nil }

func (m *MockStore) CreateMonthlyPartitionIfNotExists(_ context.Context, _ int, _ int) error { return nil }

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
		if f.Network != "" && e.Network != f.Network {
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

func (m *MockStore) ListInvocations(_ context.Context, contractID, cursor string, limit int, f InvocationFilters) ([]Invocation, string, error) {
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
		if f.Network != "" && inv.Network != f.Network {
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

func (m *MockStore) ListStorageEntries(_ context.Context, contractID, cursor string, limit int, f StorageFilters) ([]StorageEntry, string, error) {
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
		if f.Network != "" && se.Network != f.Network {
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

// ---- store.QueryStore snapshot helpers --------------------------------------

// ContractFirstLedger returns the earliest ledger with indexed data for the
// contract, or 0 when there is none.
func (m *MockStore) ContractFirstLedger(_ context.Context, contractID string) (uint32, error) {
	var first uint32
	set := false
	for _, e := range m.events {
		if e.ContractID != contractID {
			continue
		}
		if !set || e.Ledger < first {
			first = e.Ledger
			set = true
		}
	}
	for _, inv := range m.invocations {
		if inv.ContractID != contractID {
			continue
		}
		if !set || inv.Ledger < first {
			first = inv.Ledger
			set = true
		}
	}
	return first, nil
}

// GetStorageSnapshot returns, per key, the version that was live at ledger.
// Because storageEntries is append-only in the mock, historical versions are
// preserved exactly as the indexer would have written them.
func (m *MockStore) GetStorageSnapshot(_ context.Context, contractID string, ledger uint32) ([]StorageEntry, error) {
	best := make(map[string]StorageEntry)
	for _, se := range m.storageEntries {
		if se.ContractID != contractID {
			continue
		}
		// Not written yet at this ledger.
		if se.LastModifiedLedger > int64(ledger) {
			continue
		}
		// TTL had already expired at this ledger.
		if se.LiveUntilLedger > 0 && se.LiveUntilLedger < int64(ledger) {
			continue
		}
		prev, ok := best[se.KeyXDR]
		if !ok || se.LastModifiedLedger >= prev.LastModifiedLedger {
			best[se.KeyXDR] = se
		}
	}
	out := make([]StorageEntry, 0, len(best))
	for _, se := range best {
		out = append(out, se)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].KeyXDR < out[j].KeyXDR })
	return out, nil
}

// LastEventAtOrBefore returns the most recent event with ledger <= ledger.
func (m *MockStore) LastEventAtOrBefore(_ context.Context, contractID string, ledger uint32) (Event, error) {
	var latest Event
	found := false
	for _, e := range m.events {
		if e.ContractID != contractID || e.Ledger > ledger {
			continue
		}
		if !found || e.Ledger >= latest.Ledger {
			latest = e
			found = true
		}
	}
	if !found {
		return Event{}, ErrNotFound
	}
	return latest, nil
}

// ---- store.APIKeyStore ------------------------------------------------------

// AddAPIKey is a test helper that seeds an API key directly.
func (m *MockStore) AddAPIKey(k APIKey) {
	m.apiKeys = append(m.apiKeys, k)
}

func (m *MockStore) CreateAPIKey(_ context.Context, k APIKey) error {
	if m.CreateAPIKeyErr != nil {
		return m.CreateAPIKeyErr
	}
	for _, scope := range k.Scopes {
		if !ValidScopes[scope] {
			return ErrInvalidScope
		}
	}
	m.apiKeys = append(m.apiKeys, k)
	return nil
}

func (m *MockStore) GetAPIKeyByHash(_ context.Context, hash string) (APIKey, error) {
	if m.GetAPIKeyErr != nil {
		return APIKey{}, m.GetAPIKeyErr
	}
	for _, k := range m.apiKeys {
		if k.KeyHash == hash && !k.Revoked() {
			return k, nil
		}
	}
	return APIKey{}, ErrNotFound
}

func (m *MockStore) ListAPIKeys(_ context.Context, cursor string, limit int) ([]APIKey, string, error) {
	if limit <= 0 {
		limit = 50
	}
	var out []APIKey
	for _, k := range m.apiKeys {
		if cursor != "" && k.ID <= cursor {
			continue
		}
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	var next string
	if len(out) > limit {
		next = out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}

func (m *MockStore) RevokeAPIKey(_ context.Context, id string) error {
	for i := range m.apiKeys {
		if m.apiKeys[i].ID == id {
			now := time.Now().UTC()
			m.apiKeys[i].RevokedAt = &now
			return nil
		}
	}
	return ErrNotFound
}

func (m *MockStore) TouchAPIKey(_ context.Context, id string) error {
	for i := range m.apiKeys {
		if m.apiKeys[i].ID == id {
			now := time.Now().UTC()
			m.apiKeys[i].LastUsedAt = &now
			return nil
		}
	}
	return nil
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
