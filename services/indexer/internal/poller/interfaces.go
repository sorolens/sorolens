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
}

// Store is the subset of the data store the poller needs.
// The concrete implementation is store.postgresStore from apps/api.
type Store interface {
	ListContracts(ctx context.Context, cursor string, limit int) ([]Contract, string, error)
	BatchInsertEvents(ctx context.Context, events []Event) error
	BatchInsertInvocations(ctx context.Context, invocations []Invocation) error
	GetSyncState(ctx context.Context, contractID string) (SyncState, error)
	UpsertSyncState(ctx context.Context, s SyncState) error
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

// Contract mirrors store.Contract (fields the poller needs).
type Contract struct {
	ID     string
	Status string
	Network string
}

// Event mirrors store.Event.
type Event struct {
	ID               string
	ContractID       string
	Ledger           uint32
	LedgerClosedAt   time.Time
	TxHash           string
	Type             string
	TopicXDR         []string
	ValueXDR         string
	InSuccessfulCall bool
}

// Invocation mirrors store.Invocation.
type Invocation struct {
	TxHash           string
	ContractID       string
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
