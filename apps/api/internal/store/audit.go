package store

import (
	"context"
	"fmt"
	"time"
)

// AuditEvent is one state-changing API request (issue #122). Only a hash of
// the request body is stored, never the body itself.
type AuditEvent struct {
	ID              int64
	Actor           string
	Action          string // "<METHOD> <route pattern>", e.g. "POST /api/v1/contracts"
	ResourceType    string
	ResourceID      string
	IP              string
	UserAgent       string
	RequestBodyHash string // sha256 hex of the request body
	Status          int    // HTTP response status
	At              time.Time
}

// AuditStore is the read/write surface for the audit trail.
type AuditStore interface {
	// InsertAuditEvent appends one audit row.
	InsertAuditEvent(ctx context.Context, e AuditEvent) error
	// ListAuditEvents returns audit rows at or after since (zero means no
	// lower bound), newest first. cursor is the ID of the last row of the
	// previous page (0 for the first page); the returned cursor is 0 when
	// there are no more pages.
	ListAuditEvents(ctx context.Context, since time.Time, cursor int64, limit int) ([]AuditEvent, int64, error)
}

func (s *postgresStore) InsertAuditEvent(ctx context.Context, e AuditEvent) error {
	at := e.At
	if at.IsZero() {
		at = time.Now().UTC()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_events
			(actor, action, resource_type, resource_id, ip, user_agent, request_body_hash, status, at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		e.Actor, e.Action, e.ResourceType, e.ResourceID, e.IP, e.UserAgent,
		e.RequestBodyHash, e.Status, at,
	)
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

func (s *postgresStore) ListAuditEvents(ctx context.Context, since time.Time, cursor int64, limit int) ([]AuditEvent, int64, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var sinceArg *time.Time
	if !since.IsZero() {
		sinceArg = &since
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, actor, action, resource_type, resource_id, ip, user_agent,
		       request_body_hash, status, at
		FROM audit_events
		WHERE ($1::timestamptz IS NULL OR at >= $1)
		  AND ($2 = 0 OR id < $2)
		ORDER BY id DESC
		LIMIT $3`, sinceArg, cursor, limit+1)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()

	var out []AuditEvent
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.ResourceType, &e.ResourceID,
			&e.IP, &e.UserAgent, &e.RequestBodyHash, &e.Status, &e.At); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	if rows.Err() != nil {
		return nil, 0, rows.Err()
	}
	var next int64
	if len(out) > limit {
		next = out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}
