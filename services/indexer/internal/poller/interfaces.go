package poller

import (
	"context"
	"time"
)

// RPCClient is the subset of the Soroban RPC client the poller needs.
// The concrete implementation is soroban.Client from apps/api.
type RPCClient interface {
	GetLatestLedger(ctx context.Context) (*LatestLedger, error)
	GetEvents(ctx context.Context, startLedger, endLedger uint32, filters []EventFilter) (*GetEventsResult, error)
	GetTransaction(ctx context.Context, hash string) (*TransactionResult, error)
	GetLedgerEntries(ctx context.Context, keys []string) (*GetLedgerEntriesResult, error)
}

// Store is the subset of the data store the poller needs.
// The concrete implementation is store.postgresStore from apps/api.
type Store interface {
	ListContracts(ctx context.Context, cursor string, limit int) ([]Contract, string, error)
	BatchInsertEvents(ctx context.Context, events []Event) error
	BatchInsertInvocations(ctx context.Context, invocations []Invocation) error
	GetSyncState(ctx context.Context, contractID string) (SyncState, error)
	UpsertSyncState(ctx context.Context, s SyncState) error
	CreateNextMonthPartition(ctx context.Context) error
	CreateMonthlyPartitionIfNotExists(ctx context.Context, year int, month int) error

	// RecentHourlyActivity returns per-hour activity buckets for the most
	// recent `hours` hours (oldest first), aggregated across events and
	// invocations. Used by the anomaly detector to build a rolling baseline.
	RecentHourlyActivity(ctx context.Context, contractID string, hours int) ([]HourlyActivity, error)
	// InsertAlert persists an anomaly/health alert row (severity Warning for
	// anomaly spikes). Implementations may de-duplicate on (tx_hash,
	// contract_id).
	InsertAlert(ctx context.Context, a Alert) error
	// InsertContractUpgrade records a Wasm-hash change for a contract. The
	// store is responsible for ignoring duplicate (contract, tx) rows.
	InsertContractUpgrade(ctx context.Context, u ContractUpgrade) error
	// UpdateContractWasmHash records the now-current on-chain Wasm hash for a
	// contract so subsequent polls can diff against it.
	UpdateContractWasmHash(ctx context.Context, contractID, wasmHash string) error

	// ContractHealthInputs aggregates the raw signals that feed the composite
	// health score (issue #137). It never errors on empty data.
	ContractHealthInputs(ctx context.Context, contractID string) (HealthInputs, error)
	// UpsertContractHealthScore caches a computed 0-100 health score.
	UpsertContractHealthScore(ctx context.Context, h ContractHealthScore) error

	// InsertFailedEvent parks an event that exhausted processing retries
	// in the dead-letter queue (issue #202).
	InsertFailedEvent(ctx context.Context, fe FailedEvent) error
}

// RedisClient is the subset of Redis operations the poller needs for advisory locks.
type RedisClient interface {
	// SetNX sets key to value with ttl if key does not already exist.
	// Returns true if the key was set (lock acquired).
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	// Del removes the key (releases the lock).
	Del(ctx context.Context, key string) error
}

// ---- local mirror types (avoid importing apps/api from this package) ------
// The poller defines its own minimal types. The main.go adapter converts
// between apps/api types and these types when wiring up the real implementations.

// LatestLedger mirrors soroban.LatestLedger.
type LatestLedger struct {
	Sequence        uint32
	ProtocolVersion int
}

// EventFilter mirrors soroban.EventFilter.
type EventFilter struct {
	Type        string
	ContractIDs []string
}

// RPCEvent mirrors soroban.RPCEvent.
type RPCEvent struct {
	ID                       string
	ContractID               string
	Ledger                   uint32
	LedgerClosedAt           string
	TxHash                   string
	Type                     string
	Topic                    []string
	Value                    string
	InSuccessfulContractCall bool
	TransactionIndex         int
}

// GetEventsResult mirrors soroban.GetEventsResult.
type GetEventsResult struct {
	Events       []RPCEvent
	LatestLedger uint32
	Cursor       string
}

// TransactionResult mirrors soroban.TransactionResult.
type TransactionResult struct {
	Status           string
	Ledger           uint32
	LedgerClosedAt   time.Time
	ApplicationOrder int
	ResultXDR        string
	ResourceFee      int64
}

// LedgerEntry mirrors soroban.LedgerEntry.
type LedgerEntry struct {
	// Key is the base64-encoded LedgerKey that was requested.
	Key string
	// XDR is the base64-encoded LedgerEntry from getLedgerEntries.
	XDR string
	// LastModifiedLedgerSeq is the most recent ledger in which the entry
	// was modified.
	LastModifiedLedgerSeq uint32
}

// GetLedgerEntriesResult mirrors soroban.GetLedgerEntriesResult.
type GetLedgerEntriesResult struct {
	Entries        []LedgerEntry
	LatestLedger   uint32
	KeysNotFound   []string
	DuplicatedKeys []string
}

// WasmHashResult mirrors soroban.GetWasmHashResult.

// ContractUpgrade mirrors store.ContractUpgrade.
type ContractUpgrade struct {
	ContractID string
	FromHash   string
	ToHash     string
	Ledger     uint32
	TxHash     string
	At         time.Time
}

// Contract mirrors store.Contract (fields the poller needs).
type Contract struct {
	ID      string
	Status  string
	Network string
	// WasmHash is the current on-chain Wasm hash the poller last observed.
	WasmHash string
}

// Event mirrors store.Event.
type Event struct {
	ID               string
	ContractID       string
	Network          string
	Ledger           uint32
	LedgerClosedAt   time.Time
	TxHash           string
	Type             string
	TopicXDR         []string
	ValueXDR         string
	InSuccessfulCall bool
}

// FailedEvent mirrors store.FailedEvent for the indexer DLQ (issue #202).
type FailedEvent struct {
	EventID      string
	ContractID   string
	Network      string
	EventPayload []byte
	ErrorMessage string
	Attempts     int
}

// Invocation mirrors store.Invocation.
type Invocation struct {
	TxHash           string
	ContractID       string
	Network          string
	Ledger           uint32
	LedgerClosedAt   time.Time
	Status           string
	ResultXDR        string
	ApplicationOrder int
}

// SyncState mirrors store.SyncState.
type SyncState struct {
	ContractID string
	LastLedger uint32
}

// HourlyActivity is one per-hour aggregate bucket for a contract, used by the
// anomaly detector. CPU and fees are totals over the hour.
type HourlyActivity struct {
	Hour        time.Time // bucket start, UTC
	EventCount  int64
	InvokeCount int64
	CPU         int64 // sum of cpu_insn
	Fees        int64 // sum of resource fees, stroops
}

// Alert mirrors store.ContractAlert. TxHash carries a synthetic, deterministic
// key (see anomaly job) so implementations can de-duplicate re-runs.
type Alert struct {
	ContractID string
	Severity   string // Info | Warning | Critical
	Message    string
	Ledger     int64
	TxHash     string
	Timestamp  time.Time
}

// HealthInputs mirrors store.HealthScoreInputs (issue #137). It carries the
// raw aggregates an implementation gathers so the pure healthscore package can
// compute the composite score without importing apps/api.
type HealthInputs struct {
	HealthyChecks     int64
	TotalChecks       int64
	WatchdogStatus    string
	TotalInvocations  int64
	FailedInvocations int64
	Activity          []HourlyActivity
	TotalStorage      int64
	ExpiringStorage   int64
}

// ContractHealthScore mirrors store.ContractHealthScore.
type ContractHealthScore struct {
	ContractID           string
	Score                int32
	ComponentUptime      int32
	ComponentErrorRate   int32
	ComponentPerformance int32
	ComponentStorageTTL  int32
	ComputedAt           time.Time
}

// ContractVersion mirrors store.ContractVersion.
type ContractVersion struct {
	ContractID        string
	WasmHash          string
	FirstSeenLedger   int64
	TxHash            string
	VerifiedSourceRef string
}

// ErrVersionNotFound is returned by GetLatestContractVersion when no entry exists.
var ErrVersionNotFound = errorString("poller: contract version not found")

type errorString string

func (e errorString) Error() string { return string(e) }
