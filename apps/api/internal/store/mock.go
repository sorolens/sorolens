package store

import (
	"context"
	"encoding/json"
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
	contractUpgrades   []ContractUpgrade
	watchlist          map[string]map[string]bool
	alertSubscriptions []AlertSubscription
	users              map[string]User
	healthScores       map[string]ContractHealthScore
	indexerCursors     map[string]uint32
	contractVersions   map[string][]ContractVersion

	// Error injection
	UpsertContractErr           error
	GetContractErr              error
	ListContractsErr            error
	GetGlobalStatsErr           error
	ListEventsErr               error
	ListInvocationsErr          error
	ListStorageErr              error
	GetContractStatsErr         error
	RecentEventsErr             error
	RecentInvocationsErr        error
	CreateAPIKeyErr             error
	GetAPIKeyErr                error
	UpsertUserErr               error
	GetUserErr                  error
	ListUpgradesErr             error
	GetHealthScoreErr           error
	RecordContractVersionErr    error
	ListContractVersionsErr     error
	GetLatestContractVersionErr error
}

// NewMockStore returns an initialized MockStore.
func NewMockStore() *MockStore {
	return &MockStore{
		contracts:          make(map[string]Contract),
		syncStates:         make(map[string]SyncState),
		monitored:          make(map[string]MonitoredContract),
		watchlist:          make(map[string]map[string]bool),
		alerts:             make([]ContractAlert, 0),
		alertSubscriptions: make([]AlertSubscription, 0),
		users:              make(map[string]User),
		indexerCursors:     make(map[string]uint32),
		contractVersions:   make(map[string][]ContractVersion),
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

func (m *MockStore) CreateMonthlyPartitionIfNotExists(_ context.Context, _ int, _ int) error {
	return nil
}

func (m *MockStore) GetIndexerCursor(_ context.Context, network string) (uint32, error) {
	if m.indexerCursors == nil {
		return 0, nil
	}
	return m.indexerCursors[networkOrDefault(network)], nil
}

func (m *MockStore) SetIndexerCursor(_ context.Context, network string, ledger uint32) error {
	if m.indexerCursors == nil {
		m.indexerCursors = make(map[string]uint32)
	}
	m.indexerCursors[networkOrDefault(network)] = ledger
	return nil
}

func (m *MockStore) BatchInsertWithCursor(ctx context.Context, network string, ledger uint32, events []Event, invocations []Invocation, syncState SyncState) error {
	if err := m.BatchInsertEvents(ctx, events); err != nil {
		return err
	}
	if err := m.BatchInsertInvocations(ctx, invocations); err != nil {
		return err
	}
	if syncState.ContractID != "" {
		if err := m.UpsertSyncState(ctx, syncState); err != nil {
			return err
		}
	}
	return m.SetIndexerCursor(ctx, network, ledger)
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
		if f.Network != "" && e.Network != f.Network {
			continue
		}
		if f.Type != "" && e.Type != f.Type {
			continue
		}
		if f.Topic != "" && !topicDecodedContains(e.TopicDecoded, f.Topic) {
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

// topicDecodedContains reports whether a decoded topic list contains the value
// encoded by a ?topic= filter. Comparison goes through JSON so an int in test
// data matches the float64 produced by decoding a numeric filter, mirroring the
// containment semantics of postgresStore.ListEvents.
func topicDecodedContains(topics []any, topic string) bool {
	want := topicFilterValue(topic)
	for _, t := range topics {
		if jsonValueEqual(t, want) {
			return true
		}
	}
	return false
}

func jsonValueEqual(a, b any) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ab) == string(bb)
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

func (m *MockStore) ListAllInvocations(_ context.Context, cursorLedger uint32, cursorTxHash string, limit int, f InvocationFilters) ([]Invocation, uint32, string, error) {
	if m.ListInvocationsErr != nil {
		return nil, 0, "", m.ListInvocationsErr
	}
	if limit <= 0 {
		limit = 50
	}
	out := make([]Invocation, 0, len(m.invocations))
	for _, inv := range m.invocations {
		if f.ContractID != "" && inv.ContractID != f.ContractID {
			continue
		}
		if f.Network != "" && inv.Network != f.Network {
			continue
		}
		if f.Status != "" && inv.Status != f.Status {
			continue
		}
		if f.FunctionName != "" && inv.FunctionName != f.FunctionName {
			continue
		}
		if f.From != 0 && inv.Ledger < f.From {
			continue
		}
		if f.To != 0 && inv.Ledger > f.To {
			continue
		}
		if f.Since != nil && inv.LedgerClosedAt.Before(*f.Since) {
			continue
		}
		if f.Until != nil && inv.LedgerClosedAt.After(*f.Until) {
			continue
		}
		out = append(out, inv)
	}

	// Mirror the postgres ordering: newest first, tx_hash as the tie-breaker.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Ledger != out[j].Ledger {
			return out[i].Ledger > out[j].Ledger
		}
		return out[i].TxHash > out[j].TxHash
	})

	if cursorLedger != 0 || cursorTxHash != "" {
		kept := out[:0]
		for _, inv := range out {
			if inv.Ledger < cursorLedger || (inv.Ledger == cursorLedger && inv.TxHash < cursorTxHash) {
				kept = append(kept, inv)
			}
		}
		out = kept
	}

	var nextLedger uint32
	var nextTxHash string
	if len(out) > limit {
		last := out[limit-1]
		nextLedger = last.Ledger
		nextTxHash = last.TxHash
		out = out[:limit]
	}
	return out, nextLedger, nextTxHash, nil
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

// DailyAggregates aggregates the in-memory events/invocations into per-day
// buckets (midnight UTC). Mirrors the postgres generate_series behaviour:
// every day in the window appears, empty days as zeroes.
func (m *MockStore) DailyAggregates(_ context.Context, contractID string, days int) ([]DailyAggregate, error) {
	if days <= 0 {
		days = 90
	}
	now := time.Now().UTC()
	start := now.AddDate(0, 0, -days)
	dayOf := func(t time.Time) time.Time {
		y, mo, d := t.UTC().Date()
		return time.Date(y, mo, d, 0, 0, 0, 0, time.UTC)
	}

	feeByDay := map[time.Time]float64{}
	invByDay := map[time.Time]float64{}
	for _, inv := range m.invocations {
		if inv.ContractID != contractID || inv.LedgerClosedAt.Before(start) {
			continue
		}
		feeByDay[dayOf(inv.LedgerClosedAt)] += float64(inv.ResourceFeeCharged)
		invByDay[dayOf(inv.LedgerClosedAt)]++
	}

	evByDay := map[time.Time]float64{}
	for _, e := range m.events {
		if e.ContractID != contractID || e.LedgerClosedAt.Before(start) {
			continue
		}
		evByDay[dayOf(e.LedgerClosedAt)]++
	}

	var out []DailyAggregate
	for i := days - 1; i >= 0; i-- {
		d := dayOf(now.AddDate(0, 0, -i))
		out = append(out, DailyAggregate{
			Day:         d,
			Fee:         feeByDay[d],
			Invocations: invByDay[d],
			Events:      evByDay[d],
		})
	}
	return out, nil
}

// RecentHourlyActivity returns hourly buckets (oldest first) for the most
// recent `hours` hours, mirroring the postgres query with zero-fill.
func (m *MockStore) RecentHourlyActivity(_ context.Context, contractID string, hours int) ([]HourlyActivity, error) {
	if hours <= 0 {
		hours = 24
	}
	now := time.Now().UTC()
	start := now.Add(-time.Duration(hours) * time.Hour)
	hourOf := func(t time.Time) time.Time {
		y, mo, d := t.UTC().Date()
		return time.Date(y, mo, d, t.Hour(), 0, 0, 0, time.UTC)
	}

	evByHour := map[time.Time]int64{}
	for _, e := range m.events {
		if e.ContractID != contractID || e.LedgerClosedAt.Before(start) {
			continue
		}
		evByHour[hourOf(e.LedgerClosedAt)]++
	}

	invByHour := map[time.Time]int64{}
	cpuByHour := map[time.Time]int64{}
	feeByHour := map[time.Time]int64{}
	for _, inv := range m.invocations {
		if inv.ContractID != contractID || inv.LedgerClosedAt.Before(start) {
			continue
		}
		h := hourOf(inv.LedgerClosedAt)
		invByHour[h]++
		cpuByHour[h] += inv.CPUInsn
		feeByHour[h] += inv.ResourceFeeCharged
	}

	var out []HourlyActivity
	for i := hours - 1; i >= 0; i-- {
		h := hourOf(now.Add(-time.Duration(i) * time.Hour))
		out = append(out, HourlyActivity{
			Hour:        h,
			EventCount:  evByHour[h],
			InvokeCount: invByHour[h],
			CPU:         cpuByHour[h],
			Fees:        feeByHour[h],
		})
	}
	return out, nil
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

// RecentInvocations returns the newest invocations for a contract, mirroring
// the postgres query's ledger/tx-hash descending order.
func (m *MockStore) RecentInvocations(_ context.Context, contractID string, limit int) ([]Invocation, error) {
	if m.RecentInvocationsErr != nil {
		return nil, m.RecentInvocationsErr
	}
	if limit <= 0 {
		limit = 20
	}
	var out []Invocation
	for i := len(m.invocations) - 1; i >= 0 && len(out) < limit; i-- {
		if m.invocations[i].ContractID == contractID {
			out = append(out, m.invocations[i])
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

// ---- store.AlertSubscriptionStore -------------------------------------------

func (m *MockStore) Create(_ context.Context, s AlertSubscription) error {
	s.ChannelType = channelOrDefault(s.ChannelType)
	m.alertSubscriptions = append(m.alertSubscriptions, s)
	return nil
}

func (m *MockStore) ListByContract(_ context.Context, contractID string) ([]AlertSubscription, error) {
	out := make([]AlertSubscription, 0)
	for _, s := range m.alertSubscriptions {
		if s.ContractID == contractID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *MockStore) Delete(_ context.Context, id string) error {
	filtered := make([]AlertSubscription, 0, len(m.alertSubscriptions))
	for _, s := range m.alertSubscriptions {
		if s.ID != id {
			filtered = append(filtered, s)
		}
	}
	if len(filtered) == len(m.alertSubscriptions) {
		return ErrNotFound
	}
	m.alertSubscriptions = filtered
	return nil
}

func (m *MockStore) ListAll(_ context.Context) ([]AlertSubscription, error) {
	out := make([]AlertSubscription, len(m.alertSubscriptions))
	copy(out, m.alertSubscriptions)
	return out, nil
}

// ---- store.WatchlistStore ---------------------------------------------------

func (m *MockStore) AddToWatchlist(_ context.Context, userID, contractID string) error {
	if m.watchlist[userID] == nil {
		m.watchlist[userID] = make(map[string]bool)
	}
	m.watchlist[userID][contractID] = true
	return nil
}

func (m *MockStore) RemoveFromWatchlist(_ context.Context, userID, contractID string) error {
	if m.watchlist[userID] != nil {
		delete(m.watchlist[userID], contractID)
	}
	return nil
}

func (m *MockStore) ListWatchlist(_ context.Context, userID string) ([]string, error) {
	items := m.watchlist[userID]
	out := make([]string, 0, len(items))
	for contractID := range items {
		out = append(out, contractID)
	}
	sort.Strings(out)
	return out, nil
}

func (m *MockStore) IsInWatchlist(_ context.Context, userID, contractID string) (bool, error) {
	return m.watchlist[userID][contractID], nil
}

// ---- store.UserStore --------------------------------------------------------

// AddUser is a test helper that seeds a user directly.
func (m *MockStore) AddUser(u User) {
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	if u.Role == "" {
		u.Role = RoleViewer
	}
	m.users[u.ID] = u
}

func (m *MockStore) UpsertUser(_ context.Context, u User) error {
	if m.UpsertUserErr != nil {
		return m.UpsertUserErr
	}
	existing, ok := m.users[u.ID]
	if !ok {
		if u.Role == "" {
			u.Role = RoleViewer
		}
		if u.CreatedAt.IsZero() {
			u.CreatedAt = time.Now()
		}
		m.users[u.ID] = u
		return nil
	}
	// Preserve existing fields when the call does not supply a replacement.
	if u.GitHubID != nil {
		existing.GitHubID = u.GitHubID
	}
	if u.Role != "" {
		existing.Role = u.Role
	}
	m.users[u.ID] = existing
	return nil
}

func (m *MockStore) GetUserByID(_ context.Context, id string) (User, error) {
	if m.GetUserErr != nil {
		return User{}, m.GetUserErr
	}
	u, ok := m.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (m *MockStore) GetUserByGitHubID(_ context.Context, githubID string) (User, error) {
	if m.GetUserErr != nil {
		return User{}, m.GetUserErr
	}
	for _, u := range m.users {
		if u.GitHubID != nil && *u.GitHubID == githubID {
			return u, nil
		}
	}
	return User{}, ErrNotFound
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

func (m *MockStore) ComputeAndStoreBaselines(ctx context.Context, snapshotDate time.Time) error {
	return nil
}

func (m *MockStore) CheckAndEmitRegressions(ctx context.Context, snapshotDate time.Time) (int, error) {
	return 0, nil
}

// ---- contract versions ---------------------------------------------------

func (m *MockStore) RecordContractVersion(_ context.Context, v ContractVersion) error {
	if m.RecordContractVersionErr != nil {
		return m.RecordContractVersionErr
	}
	if v.RecordedAt.IsZero() {
		v.RecordedAt = time.Now()
	}
	existing := m.contractVersions[v.ContractID]
	for _, prev := range existing {
		if prev.WasmHash == v.WasmHash {
			return nil // idempotent ON CONFLICT DO NOTHING
		}
	}
	m.contractVersions[v.ContractID] = append(existing, v)
	return nil
}

func (m *MockStore) ListContractVersions(_ context.Context, contractID string) ([]ContractVersion, error) {
	if m.ListContractVersionsErr != nil {
		return nil, m.ListContractVersionsErr
	}
	versions, ok := m.contractVersions[contractID]
	if !ok {
		return nil, nil
	}
	out := make([]ContractVersion, len(versions))
	copy(out, versions)
	sort.Slice(out, func(i, j int) bool {
		return out[i].FirstSeenLedger < out[j].FirstSeenLedger
	})
	return out, nil
}

func (m *MockStore) GetLatestContractVersion(_ context.Context, contractID string) (ContractVersion, error) {
	if m.GetLatestContractVersionErr != nil {
		return ContractVersion{}, m.GetLatestContractVersionErr
	}
	versions, ok := m.contractVersions[contractID]
	if !ok || len(versions) == 0 {
		return ContractVersion{}, ErrNotFound
	}
	latest := versions[0]
	for _, v := range versions[1:] {
		if v.FirstSeenLedger > latest.FirstSeenLedger {
			latest = v
		}
	}
	return latest, nil
}

