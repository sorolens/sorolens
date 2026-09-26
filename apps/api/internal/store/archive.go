package store

import (
	"context"
	"fmt"
	"time"
)

// ArchiveStore provides the Postgres side of the cold-storage tier (issue
// #146): reading the rows that are candidates for export, and deleting them
// once their Parquet copy is durable.
//
// The archive job reads a window of old events, writes them to object storage,
// and only then deletes them here. Deletion is therefore a separate, explicit
// step so a failed upload can never lose data.
type ArchiveStore interface {
	// ContractsWithEventsBefore returns the IDs of every contract that still
	// has at least one event older than `before`, ordered by ID.
	ContractsWithEventsBefore(ctx context.Context, before time.Time) ([]string, error)

	// EventsBefore returns up to `limit` events for one contract that are
	// older than `before`, oldest first. The ordering is stable
	// (ledger, id) so a caller looping until it gets a short page sees every
	// row exactly once.
	EventsBefore(ctx context.Context, contractID string, before time.Time, limit int) ([]Event, error)

	// DeleteEventsByID removes the given event IDs from Postgres and returns
	// the number of rows deleted. It is a no-op for an empty slice.
	DeleteEventsByID(ctx context.Context, ids []string) (int64, error)
}

// ---- ContractsWithEventsBefore ----------------------------------------------

func (s *postgresStore) ContractsWithEventsBefore(ctx context.Context, before time.Time) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT contract_id
		FROM events
		WHERE ledger_closed_at < $1
		ORDER BY contract_id`, before)
	if err != nil {
		return nil, fmt.Errorf("contracts with cold events: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ---- EventsBefore -----------------------------------------------------------

func (s *postgresStore) EventsBefore(ctx context.Context, contractID string, before time.Time, limit int) ([]Event, error) {
	if limit <= 0 || limit > 100_000 {
		limit = 10_000
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+eventColumns+`
		FROM events
		WHERE contract_id = $1
		  AND ledger_closed_at < $2
		ORDER BY ledger ASC, id ASC
		LIMIT $3`, contractID, before, limit)
	if err != nil {
		return nil, fmt.Errorf("events before cutoff: %w", err)
	}
	return scanEvents(rows)
}

// ---- DeleteEventsByID -------------------------------------------------------

func (s *postgresStore) DeleteEventsByID(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM events WHERE id = ANY($1)`, ids)
	if err != nil {
		return 0, fmt.Errorf("delete archived events: %w", err)
	}
	return tag.RowsAffected(), nil
}
