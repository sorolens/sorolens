package store

import "time"

// Contract is a Soroban contract being tracked by the indexer.
type Contract struct {
	ID                  string
	Network             string
	Label               string
	WasmHash            string
	CreatedAtLedger     int64
	BackfillCompleteAt  *time.Time
	Status              string // pending | backfilling | active | paused | error
	AddedAt             time.Time
}

// Event is a single contract event indexed from the Soroban RPC.
type Event struct {
	ID                  string
	ContractID          string
	Ledger              uint32
	LedgerClosedAt      time.Time
	TxHash              string
	Type                string
	TopicXDR            []string // raw base64 XDR ScVal strings
	ValueXDR            string
	TopicDecoded        []any
	ValueDecoded        any
	InSuccessfulCall    bool
	InsertedAt          time.Time
}

// Invocation is a single transaction that invoked a tracked contract.
type Invocation struct {
	TxHash              string
	ContractID          string
	Ledger              uint32
	LedgerClosedAt      time.Time
	Status              string // SUCCESS | FAILED | NOT_FOUND
	FunctionName        string
	ArgsDecoded         map[string]any
	ResultDecoded       any
	ResultXDR           string
	ResourceFeeCharged  int64
	CPUInsn             int64
	MemByte             int64
	LedgerReadByte      int64
	LedgerWriteByte     int64
	ApplicationOrder    int
	InsertedAt          time.Time
}

// StorageEntry is a snapshot of one contract storage key.
type StorageEntry struct {
	ContractID          string
	KeyXDR              string
	KeyDecoded          any
	ValueXDR            string
	ValueDecoded        any
	Durability          string // temporary | persistent | instance
	LiveUntilLedger     int64
	LastModifiedLedger  int64
	Status              string // live | archived | deleted
	LastSeenAt          time.Time
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
