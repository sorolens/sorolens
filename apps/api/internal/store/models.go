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
	LastActivityAt     *time.Time
	// Tags are free-form labels held in the contract_tags table, hydrated
	// alongside the contract so the list and detail responses carry them.
	Tags []string
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

// ContractVersion records a single Wasm hash transition observed by the indexer.
// It corresponds to one row in the contract_versions table.
type ContractVersion struct {
	ID                int64
	ContractID        string
	WasmHash          string
	FirstSeenLedger   int64
	TxHash            string // empty string when not yet linked to a tx
	VerifiedSourceRef string // empty string when the Wasm is unverified
	RecordedAt        time.Time
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
	// ChannelType is webhook | slack | discord | pagerduty (issue #127).
	ChannelType string
	// RoutingKey is the PagerDuty integration key (pagerduty only). Secret.
	RoutingKey string
	CreatedAt  time.Time
	UpdatedAt  time.Time
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

// Label maps a human-readable name to a Stellar account or contract ID.
type Label struct {
	Label       string
	Value       string
	WorkspaceID string
	Public      bool
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
	HealthyChecks     int64
	TotalChecks       int64
	WatchdogStatus    string
	TotalInvocations  int64
	FailedInvocations int64
	Activity          []HourlyActivity
	TotalStorage      int64
	ExpiringStorage   int64
}

// VerificationDiagnostic is one actionable item produced by a contract source
// verification attempt (issue #263). It is persisted as JSONB and returned
// verbatim to API clients, so the JSON field names are part of the API shape.
type VerificationDiagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"` // error | warning | info
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
}

// ContractVerification is the persisted outcome of a source-verification
// attempt for one contract (issue #263). There is exactly one row per
// contract: the most recent submission replaces the previous verdict.
type ContractVerification struct {
	ContractID     string
	Status         string // pending | verified | failed
	OnChainHash    string
	CompiledHash   string
	Matched        bool
	SourceKind     string // archive | git | ""
	SourceRef      string // archive filename, or "<git url>@<commit>"
	SourceDigest   string // sha256 of the archive, or of the git ref
	StellarVersion string
	RustcVersion   string
	CargoVersion   string
	Diagnostics    []VerificationDiagnostic
	BuildLog       string
	SubmittedAt    time.Time
	VerifiedAt     *time.Time
	UpdatedAt      time.Time
}
