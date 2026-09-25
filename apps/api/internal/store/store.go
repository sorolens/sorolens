// Package store defines the data-access layer for indexed Soroban contract
// data. It provides a Store interface backed by PostgreSQL (pgxpool) for
// persisting and querying contracts, events, invocations, storage entries,
// sync state, and global statistics.
//
// # Data model
//
// The Store manages five tables (contracts, events, invocations,
// storage_entries, and sync_state) that together form the indexed
// observability backend for Sorolens. Contracts are tracked by network and
// status; events carry decoded XDR values; invocations include per-call
// resource metrics (CPU, memory, ledger I/O, fees); storage entries track
// durability and TTL health; and sync state records the last-polled ledger
// per contract for incremental indexing.
//
// See internal/db/migrations/ for the full schema DDL.
package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the data-access interface for all indexer and API persistence.
// All methods accept a context so callers can enforce deadlines and
// propagate cancellation.
type Store interface {
	// UpsertContract inserts or updates a contract row.
	UpsertContract(ctx context.Context, c Contract) error

	// GetContract returns the contract with the given ID, or ErrNotFound.
	GetContract(ctx context.Context, contractID string) (Contract, error)

	// ListContracts returns a cursor-paginated list of tracked contracts
	// matching the optional filters. cursor is opaque; pass "" for the first
	// page. Returns the next cursor (empty string when there are no more
	// pages) as the second return value.
	ListContracts(ctx context.Context, cursor string, limit int, f ContractFilters) ([]Contract, string, error)

	// BatchInsertEvents inserts events, ignoring duplicates by primary key.
	// All rows are sent in a single network round-trip.
	BatchInsertEvents(ctx context.Context, events []Event) error

	// BatchInsertInvocations inserts invocations, ignoring duplicates.
	BatchInsertInvocations(ctx context.Context, invocations []Invocation) error

	// UpsertStorageEntries inserts or updates storage entries for a contract.
	UpsertStorageEntries(ctx context.Context, entries []StorageEntry) error

	// GetSyncState returns the sync cursor for a contract, or a zero-value
	// SyncState (LastLedger == 0) if no state has been recorded yet.
	GetSyncState(ctx context.Context, contractID string) (SyncState, error)

	// UpsertSyncState writes the sync cursor for a contract.
	UpsertSyncState(ctx context.Context, s SyncState) error

	// GetGlobalStats returns aggregate counts across all tracked contracts.
	// Computed with a single SQL query.
	GetGlobalStats(ctx context.Context) (GlobalStats, error)

	// CreateNextMonthPartition creates the partition for next month if it does not exist.
	CreateNextMonthPartition(ctx context.Context) error

	// CreateMonthlyPartitionIfNotExists creates a partition for the given year/month if it does not exist.
	CreateMonthlyPartitionIfNotExists(ctx context.Context, year int, month int) error

	// GetIndexerCursor returns the last successfully committed ledger sequence for a network,
	// or 0 if no cursor has been recorded yet.
	GetIndexerCursor(ctx context.Context, network string) (uint32, error)

	// SetIndexerCursor updates the cursor for a network.
	SetIndexerCursor(ctx context.Context, network string, ledger uint32) error

	// BatchInsertWithCursor inserts events, invocations, upserts contract sync state,
	// and advances the network indexer cursor within a single database transaction.
	// If any operation fails or the process crashes mid-poll before commit,
	// the entire batch is rolled back atomically.
	BatchInsertWithCursor(ctx context.Context, network string, ledger uint32, events []Event, invocations []Invocation, syncState SyncState) error
}

// AlertSubscriptionStore is the read/write surface for alert webhook subscriptions.
type AlertSubscriptionStore interface {
	Create(ctx context.Context, s AlertSubscription) error
	ListByContract(ctx context.Context, contractID string) ([]AlertSubscription, error)
	Delete(ctx context.Context, id string) error
	ListAll(ctx context.Context) ([]AlertSubscription, error)
}

// WatchlistStore is the interface for per-user watchlist (bookmark) operations.
type WatchlistStore interface {
	// AddToWatchlist adds a contract to a user's watchlist.
	AddToWatchlist(ctx context.Context, userID, contractID string) error

	// RemoveFromWatchlist removes a contract from a user's watchlist.
	RemoveFromWatchlist(ctx context.Context, userID, contractID string) error

	// ListWatchlist returns all contract IDs in a user's watchlist.
	ListWatchlist(ctx context.Context, userID string) ([]string, error)

	// IsInWatchlist checks if a contract is in a user's watchlist.
	IsInWatchlist(ctx context.Context, userID, contractID string) (bool, error)
}

// UserStore is the data-access surface for role-based access control and
// user lookups used by the RBAC middleware.
type UserStore interface {
	// UpsertUser inserts or updates a user, setting its role. Missing GitHub
	// IDs and roles are left untouched on update.
	UpsertUser(ctx context.Context, u User) error
	// GetUserByID returns the user with the given ID, or ErrNotFound.
	GetUserByID(ctx context.Context, id string) (User, error)
	// GetUserByGitHubID returns the user whose GitHub ID matches, or
	// ErrNotFound.
	GetUserByGitHubID(ctx context.Context, githubID string) (User, error)
}

// ContractFilters holds optional query filters for listing contracts.
type ContractFilters struct {
	// Network restricts results to one of testnet | mainnet | futurenet.
	// Empty means all networks.
	Network string
	// Status restricts results to one contract status (e.g. "active").
	// Empty means all statuses.
	Status string
}

// NewStore returns a Store backed by the given pgxpool.Pool.
func NewStore(pool *pgxpool.Pool) Store {
	return &postgresStore{pool: pool}
}
