package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// GlobalEventFilters holds optional filters for the cross-contract events
// explorer (issue #97).
type GlobalEventFilters struct {
	// ContractID matches contracts whose ID starts with this value, so a
	// partially typed contract ID still narrows the feed.
	ContractID string
	Type       string
	Network    string
	// Since and Until bound ledger_closed_at (inclusive). Zero means open.
	Since time.Time
	Until time.Time
}

// GlobalEventStore lists events across every tracked contract.
type GlobalEventStore interface {
	// ListAllEvents returns events newest first. cursor is the ID of the
	// last event on the previous page ("" for the first page); the returned
	// cursor is "" when there are no more pages.
	ListAllEvents(ctx context.Context, cursor string, limit int, f GlobalEventFilters) ([]Event, string, error)
}

func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func (s *postgresStore) ListAllEvents(ctx context.Context, cursor string, limit int, f GlobalEventFilters) ([]Event, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	// Event IDs are RPC paging tokens (TOID based), so ordering by ID is
	// ledger order and a keyset cursor on ID is stable under inserts.
	rows, err := s.pool.Query(ctx, `
		SELECT id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
		       topic_xdr, value_xdr, topic_decoded, value_decoded,
		       in_successful_call, inserted_at
		FROM events
		WHERE ($1 = '' OR id < $1)
		  AND ($2 = '' OR contract_id LIKE $2 || '%')
		  AND ($3 = '' OR type = $3)
		  AND ($4 = '' OR network = $4)
		  AND ($5::timestamptz IS NULL OR ledger_closed_at >= $5)
		  AND ($6::timestamptz IS NULL OR ledger_closed_at <= $6)
		ORDER BY id DESC
		LIMIT $7`,
		cursor, likePrefix(f.ContractID), f.Type, f.Network,
		optionalTime(f.Since), optionalTime(f.Until), limit+1,
	)
	if err != nil {
		return nil, "", fmt.Errorf("list all events: %w", err)
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var e Event
		var topicXDR, topicDec, valDec []byte
		if err := rows.Scan(
			&e.ID, &e.ContractID, &e.Network, &e.Ledger, &e.LedgerClosedAt, &e.TxHash, &e.Type,
			&topicXDR, &e.ValueXDR, &topicDec, &valDec,
			&e.InSuccessfulCall, &e.InsertedAt,
		); err != nil {
			return nil, "", err
		}
		_ = json.Unmarshal(topicXDR, &e.TopicXDR)
		_ = json.Unmarshal(topicDec, &e.TopicDecoded)
		_ = json.Unmarshal(valDec, &e.ValueDecoded)
		out = append(out, e)
	}
	if rows.Err() != nil {
		return nil, "", rows.Err()
	}

	var next string
	if len(out) > limit {
		next = out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}

// likePrefix escapes LIKE wildcards so user input is matched literally.
func likePrefix(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// ListAllEvents implements GlobalEventStore for tests.
func (m *MockStore) ListAllEvents(_ context.Context, cursor string, limit int, f GlobalEventFilters) ([]Event, string, error) {
	if m.ListEventsErr != nil {
		return nil, "", m.ListEventsErr
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var out []Event
	for _, e := range m.events {
		if cursor != "" && e.ID >= cursor {
			continue
		}
		if f.ContractID != "" && !strings.HasPrefix(e.ContractID, f.ContractID) {
			continue
		}
		if f.Type != "" && e.Type != f.Type {
			continue
		}
		if f.Network != "" && e.Network != f.Network {
			continue
		}
		if !f.Since.IsZero() && e.LedgerClosedAt.Before(f.Since) {
			continue
		}
		if !f.Until.IsZero() && e.LedgerClosedAt.After(f.Until) {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	var next string
	if len(out) > limit {
		next = out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}
