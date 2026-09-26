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

// CallEdge is one parent -> child invocation edge of a transaction's
// cross-contract call graph. The indexer materialises these from the Soroban
// host diagnostic event stream (fn_call / fn_return / core_metrics).
//
// The transaction's root invocation is deliberately not an edge: it already
// lives in the invocations table (tx_hash PRIMARY KEY). Span ids are
// deterministic call-path strings ("0", "0.0", "0.1", "0.0.0", ...).
type CallEdge struct {
	TxHash           string
	ParentSpanID     string
	ChildSpanID      string
	CalleeContractID string
	FunctionName     string
	CPU              int64
	Mem              int64
	FeeShare         int64
	Depth            int
	Network          string
	Ledger           uint32
	LedgerClosedAt   time.Time
	InsertedAt       time.Time
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

	// SigningSecret is the recoverable "whsec_..." key deliveries are signed
	// with. HMAC signing needs the raw key, so it is stored (not one-way
	// hashed); access is confined to the reveal/rotate endpoints and the
	// delivery worker.
	SigningSecret string
	// SigningSecretHash is the SHA-256 hex digest of SigningSecret.
	SigningSecretHash string
	// SigningSecretCreatedAt starts the window during which the secret may be
	// revealed; SigningSecretRotatedAt re-opens it on rotation.
	SigningSecretCreatedAt time.Time
	SigningSecretRotatedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Role names for role-based access control.
const (
	RoleViewer      = "viewer"
	RoleContributor = "contributor"
	RoleAdmin       = "admin"
)

// ValidRoles is the set of roles a user may hold.
var ValidRoles = map[string]bool{
	RoleViewer:      true,
	RoleContributor: true,
	RoleAdmin:       true,
}

// RoleRank returns a total ordering over roles so the Router can decide
// whether one role satisfies a minimum requirement. Higher is more
// privileged; any unknown role ranks below viewer (no privileges).
func RoleRank(role string) int {
	switch role {
	case RoleAdmin:
		return 3
	case RoleContributor:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}

// User represents a Sorolens user.
type User struct {
	ID        string
	GitHubID  *string
	Role      string
	CreatedAt time.Time
}

// WatchlistItem represents a contract bookmarked by a user.
type WatchlistItem struct {
	UserID     string
	ContractID string
	AddedAt    time.Time
}

// ContractUpgrade records a Wasm-hash change for a tracked contract.
type ContractUpgrade struct {
	ID         int64
	ContractID string
	FromHash   string
	ToHash     string
	Ledger     int64
	TxHash     string
	At         time.Time
}

// ContractHealthScore is a cached composite health score for a contract.
type ContractHealthScore struct {
	ContractID           string
	Score                int32
	ComponentUptime      int32
	ComponentErrorRate   int32
	ComponentPerformance int32
	ComponentStorageTTL  int32
	ComputedAt           time.Time
}

// HealthScoreInputs holds the raw signals aggregated to compute a health score.
type HealthScoreInputs struct {
	HealthyChecks    int64
	TotalChecks      int64
	WatchdogStatus   string
	TotalInvocations int64
	FailedInvocations int64
	Activity          []HourlyActivity
	TotalStorage      int64
	ExpiringStorage   int64
}
