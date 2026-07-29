// Package store defines the data-access layer for indexed Soroban contract
// data. It provides a Store interface backed by PostgreSQL (pgxpool) for
// persisting and querying contracts, events, invocations, storage entries,
// sync state, and global statistics.
//
// # Data model
//
// The Store manages five tables — contracts, events, invocations,
// storage_entries, and sync_state — that together form the indexed
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

	// ListContracts returns a cursor-paginated list of all tracked contracts.
	// cursor is opaque; pass "" for the first page. Returns the next cursor
	// (empty string when there are no more pages) as the second return value.
	ListContracts(ctx context.Context, cursor string, limit int) ([]Contract, string, error)

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
}

// NewStore returns a Store backed by the given pgxpool.Pool.
func NewStore(pool *pgxpool.Pool) Store {
	return &postgresStore{pool: pool}
}
