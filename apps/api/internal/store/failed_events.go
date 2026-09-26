package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// FailedEvent is one dead-lettered indexer event (issue #202).
// EventPayload carries a JSON-serialized Event so requeue can
// re-insert into the events table without reconstructing fields.
type FailedEvent struct {
	ID           int64
	EventID      string
	ContractID   string
	Network      string
	EventPayload json.RawMessage
	ErrorMessage string
	Attempts     int
	CreatedAt    time.Time
}

// FailedEventStore is the read/write surface for the event DLQ.
// Kept as its own interface so the API and indexer adapter can be
// wired independently, mirroring HealthScoreStore.
type FailedEventStore interface {
	// InsertFailedEvent parks an event that exhausted retries.
	// On conflict of event_id the existing row is updated with the
	// latest error and attempt count.
	InsertFailedEvent(ctx context.Context, fe FailedEvent) error
	// ListFailedEvents returns DLQ rows newest-first. cursor is the
	// opaque id string of the last item from the previous page
	// (empty for the first page). limit defaults to 50.
	ListFailedEvents(ctx context.Context, cursor string, limit int) ([]FailedEvent, string, error)
	// GetFailedEvent returns one DLQ row by its surrogate id.
	GetFailedEvent(ctx context.Context, id int64) (FailedEvent, error)
	// DeleteFailedEvent removes a DLQ row (used after successful requeue).
	DeleteFailedEvent(ctx context.Context, id int64) error
}

// ---- postgres implementation ----------------------------------------------

func (s *postgresStore) InsertFailedEvent(ctx context.Context, fe FailedEvent) error {
	if fe.Attempts <= 0 {
		fe.Attempts = 3
	}
	if fe.CreatedAt.IsZero() {
		fe.CreatedAt = time.Now().UTC()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO failed_events
			(event_id, contract_id, network, event_payload, error_message, attempts, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (event_id) DO UPDATE SET
			contract_id    = EXCLUDED.contract_id,
			network        = EXCLUDED.network,
			event_payload  = EXCLUDED.event_payload,
			error_message  = EXCLUDED.error_message,
			attempts       = EXCLUDED.attempts,
			created_at     = EXCLUDED.created_at`,
		fe.EventID, fe.ContractID, networkOrDefault(fe.Network), fe.EventPayload,
		fe.ErrorMessage, fe.Attempts, fe.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert failed event: %w", err)
	}
	return nil
}

func (s *postgresStore) ListFailedEvents(ctx context.Context, cursor string, limit int) ([]FailedEvent, string, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows pgx.Rows
	var err error
	if cursor == "" {
		rows, err = s.pool.Query(ctx, `
			SELECT id, event_id, contract_id, network, event_payload,
			       error_message, attempts, created_at
			FROM failed_events
			ORDER BY id DESC
			LIMIT $1`, limit+1)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT id, event_id, contract_id, network, event_payload,
			       error_message, attempts, created_at
			FROM failed_events
			WHERE id < $1::bigint
			ORDER BY id DESC
			LIMIT $2`, cursor, limit+1)
	}
	if err != nil {
		return nil, "", fmt.Errorf("list failed events: %w", err)
	}
	defer rows.Close()

	var out []FailedEvent
	for rows.Next() {
		var fe FailedEvent
		if err := rows.Scan(
			&fe.ID, &fe.EventID, &fe.ContractID, &fe.Network, &fe.EventPayload,
			&fe.ErrorMessage, &fe.Attempts, &fe.CreatedAt,
		); err != nil {
			return nil, "", fmt.Errorf("scan failed event: %w", err)
		}
		out = append(out, fe)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var next string
	if len(out) > limit {
		out = out[:limit]
		next = fmt.Sprintf("%d", out[len(out)-1].ID)
	}
	return out, next, nil
}

func (s *postgresStore) GetFailedEvent(ctx context.Context, id int64) (FailedEvent, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, event_id, contract_id, network, event_payload,
		       error_message, attempts, created_at
		FROM failed_events
		WHERE id = $1`, id)
	var fe FailedEvent
	err := row.Scan(
		&fe.ID, &fe.EventID, &fe.ContractID, &fe.Network, &fe.EventPayload,
		&fe.ErrorMessage, &fe.Attempts, &fe.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return FailedEvent{}, ErrNotFound
	}
	if err != nil {
		return FailedEvent{}, fmt.Errorf("get failed event: %w", err)
	}
	return fe, nil
}

func (s *postgresStore) DeleteFailedEvent(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM failed_events WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete failed event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
