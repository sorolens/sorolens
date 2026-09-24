package store

import "time"

// Contract is a Soroban contract being tracked by the indexer.
type Contract struct {
	ID                 string
	Network            string
	Label              string
	WasmHash           string
	CreatedAtLedger    int64
	BackfillCompleteAt *time.Time
	Status             string // pending | backfilling | active | paused | error
	AddedAt            time.Time
}

// Event is a single contract event indexed from the Soroban RPC.
type Event struct {
	ID               string
	ContractID       string
	Network          string
	Ledger           uint32
	LedgerClosedAt   time.Time
	TxHash           string
	Type             string
	TopicXDR         []string // raw base64 XDR ScVal strings
	ValueXDR         string
	TopicDecoded     []any
	ValueDecoded     any
	InSuccessfulCall bool
	InsertedAt       time.Time
}

// Invocation is a single transaction that invoked a tracked contract.
type Invocation struct {
	TxHash             string
	ContractID         string
	Network            string
	Ledger             uint32
	LedgerClosedAt     time.Time
	Status             string // SUCCESS | FAILED | NOT_FOUND
	FunctionName       string
	ArgsDecoded        map[string]any
	ResultDecoded      any
	ResultXDR          string
	ResourceFeeCharged int64
	CPUInsn            int64
	MemByte            int64
	LedgerReadByte     int64
	LedgerWriteByte    int64
	ApplicationOrder   int
	InsertedAt         time.Time
}

// StorageEntry is a snapshot of one contract storage key.
type StorageEntry struct {
	ContractID         string
	Network            string
	KeyXDR             string
	KeyDecoded         any
	ValueXDR           string
	ValueDecoded       any
	Durability         string // temporary | persistent | instance
	LiveUntilLedger    int64
	LastModifiedLedger int64
	Status             string // live | archived | deleted
	LastSeenAt         time.Time
}

// SyncState tracks the indexer cursor for one contract.
type SyncState struct {
	ContractID   string
	LastLedger   uint32
	LastRunAt    *time.Time
	ErrorMessage string
	UpdatedAt    time.Time
}

// GlobalStats is a network-wide summary across all tracked contracts.
type GlobalStats struct {
	TrackedContracts    int64
	TotalEvents         int64
	TotalInvocations    int64
	TotalStorageEntries int64
}

// AlertSubscription represents a webhook subscription for watchdog alerts.
type AlertSubscription struct {
	ID             string
	ContractID     string
	WebhookURL     string
	SeverityFilter string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ContractHealthScore is one cached composite 0-100 health score row for a
// contract (issue #137). The indexer recomputes and upserts it every poll
// cycle; the component_* fields hold the normalized 0-100 value of each input
// so the dashboard can render a breakdown without recomputing.
type ContractHealthScore struct {
	ContractID           string
	Score                int32
	ComponentUptime      int32
	ComponentErrorRate   int32
	ComponentPerformance int32
	ComponentStorageTTL  int32
	ComputedAt           time.Time
}

// HealthScoreInputs holds the raw aggregates that feed the composite health
// score (issue #137). The indexer fetches these every poll cycle and runs the
// pure scoring function over them.
type HealthScoreInputs struct {
	// Watchdog uptime: recent health-check history for the contract.
	HealthyChecks int64
	TotalChecks   int64
	// WatchdogStatus is monitored_contracts.status (Healthy/Degraded/
	// Unresponsive). Empty when the contract is not registered with the
	// watchdog; used as a fallback when there is no check history.
	WatchdogStatus string
	// Invocation error rate over the trailing window.
	TotalInvocations  int64
	FailedInvocations int64
	// Activity is the chronological per-hour activity (oldest first) used to
	// evaluate the CPU/fee trend.
	Activity []HourlyActivity
	// Storage TTL headroom: live entries vs entries expiring within the
	// headroom horizon of the current ledger.
	TotalStorage    int64
	ExpiringStorage int64
}

// ContractUpgrade records one observed Wasm-hash change for a tracked contract.
type ContractUpgrade struct {
	ID         int64
	ContractID string
	FromHash   string
	ToHash     string
	Ledger     int64
	TxHash     string
	At         time.Time
}

// User represents a Sorolens user.
type User struct {
	ID        string
	GitHubID  *string
	CreatedAt time.Time
}

// WatchlistItem represents a contract bookmarked by a user.
type WatchlistItem struct {
	UserID     string
	ContractID string
	AddedAt    time.Time
}
