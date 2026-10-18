package store

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---- model -----------------------------------------------------------------

// AlertGroup is one deduplicated, grouped summary of related ContractAlerts.
// It corresponds to one row in the alert_groups table (migration 000011).
type AlertGroup struct {
	ID               int64
	GroupKey         string // "contractID|severity|rule"
	ContractID       string
	Severity         string // Info | Warning | Critical
	Rule             string
	Count            int64
	DedupeWindowSecs int64
	FirstSeen        time.Time
	LastSeen         time.Time
	LastMessage      string
	BackfillEligible bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// AlertGroupFilters holds optional query predicates for ListAlertGroups.
type AlertGroupFilters struct {
	ContractID string // empty means all contracts
	Severity   string // empty means all severities
	Network    string // empty means all networks (joined via monitored_contracts)
}

// ---- interface -------------------------------------------------------------

// AlertGroupStore is the read/write surface for grouped alert data (issue #269).
type AlertGroupStore interface {
	// UpsertAlertGroup inserts a new group or merges into an existing one that
	// shares the same group_key, updating count/last_seen/last_message.
	UpsertAlertGroup(ctx context.Context, g AlertGroup) (AlertGroup, error)

	// ListAlertGroups returns groups ordered by last_seen DESC, optionally
	// filtered. cursor is an opaque keyset token; pass "" for the first page.
	// Returns the next cursor (empty when exhausted) as the second return value.
	ListAlertGroups(ctx context.Context, cursor string, limit int, f AlertGroupFilters) ([]AlertGroup, string, error)
}

// ---- cursor helpers --------------------------------------------------------

// encodeGroupCursor builds an opaque cursor for the alert-groups feed using
// the same base64.StdEncoding used by encodeAlertsCursor in watchdog.go.
func encodeGroupCursor(lastSeen time.Time, id int64) string {
	raw := lastSeen.UTC().Format(time.RFC3339Nano) + "|" + fmt.Sprintf("%d", id)
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// decodeGroupCursor parses a cursor produced by encodeGroupCursor.
func decodeGroupCursor(cursor string) (time.Time, int64, error) {
	if cursor == "" {
		return time.Time{}, 0, ErrInvalidCursor
	}
	raw, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, 0, ErrInvalidCursor
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return time.Time{}, 0, ErrInvalidCursor
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, 0, ErrInvalidCursor
	}
	var id int64
	if _, err := fmt.Sscanf(parts[1], "%d", &id); err != nil {
		return time.Time{}, 0, ErrInvalidCursor
	}
	return ts, id, nil
}

// ---- postgres implementation -----------------------------------------------

func (s *postgresStore) UpsertAlertGroup(ctx context.Context, g AlertGroup) (AlertGroup, error) {
	now := time.Now().UTC()
	row := s.pool.QueryRow(ctx, `
		INSERT INTO alert_groups
		    (group_key, contract_id, severity, rule, count, dedupe_window_secs,
		     first_seen, last_seen, last_message, backfill_eligible, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (group_key) DO UPDATE SET
		    count        = alert_groups.count + EXCLUDED.count,
		    last_seen    = EXCLUDED.last_seen,
		    last_message = EXCLUDED.last_message,
		    updated_at   = EXCLUDED.updated_at
		RETURNING id, group_key, contract_id, severity, rule, count,
		          dedupe_window_secs, first_seen, last_seen, last_message,
		          backfill_eligible, created_at, updated_at`,
		g.GroupKey, g.ContractID, g.Severity, g.Rule, g.Count, g.DedupeWindowSecs,
		g.FirstSeen.UTC(), g.LastSeen.UTC(), g.LastMessage, g.BackfillEligible, now, now,
	)
	return scanAlertGroupRow(row)
}

func (s *postgresStore) ListAlertGroups(ctx context.Context, cursor string, limit int, f AlertGroupFilters) ([]AlertGroup, string, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var cursorTS time.Time
	var cursorID int64
	hasCursor := cursor != ""
	if hasCursor {
		ts, id, err := decodeGroupCursor(cursor)
		if err != nil {
			return nil, "", ErrInvalidCursor
		}
		cursorTS, cursorID = ts, id
	}
	rows, err := s.pool.Query(ctx, `
		SELECT ag.id, ag.group_key, ag.contract_id, ag.severity, ag.rule,
		       ag.count, ag.dedupe_window_secs, ag.first_seen, ag.last_seen,
		       ag.last_message, ag.backfill_eligible, ag.created_at, ag.updated_at
		FROM alert_groups ag
		JOIN monitored_contracts mc ON mc.contract_id = ag.contract_id
		WHERE ($1 = '' OR ag.contract_id = $1)
		  AND ($2 = '' OR ag.severity    = $2)
		  AND ($3 = '' OR mc.network     = $3)
		  AND (NOT $4    OR (ag.last_seen, ag.id) < ($5::timestamptz, $6))
		ORDER BY ag.last_seen DESC, ag.id DESC
		LIMIT $7`,
		f.ContractID, f.Severity, f.Network,
		hasCursor, cursorTS, cursorID,
		limit+1,
	)
	if err != nil {
		return nil, "", fmt.Errorf("list alert groups: %w", err)
	}
	defer rows.Close()

	var out []AlertGroup
	for rows.Next() {
		ag, err := scanAlertGroupRows(rows)
		if err != nil {
			return nil, "", err
		}
		out = append(out, ag)
	}
	if rows.Err() != nil {
		return nil, "", rows.Err()
	}
	var next string
	if len(out) > limit {
		last := out[limit-1]
		next = encodeGroupCursor(last.LastSeen, last.ID)
		out = out[:limit]
	}
	return out, next, nil
}

func scanAlertGroupRow(row pgx.Row) (AlertGroup, error) {
	var g AlertGroup
	err := row.Scan(
		&g.ID, &g.GroupKey, &g.ContractID, &g.Severity, &g.Rule,
		&g.Count, &g.DedupeWindowSecs, &g.FirstSeen, &g.LastSeen,
		&g.LastMessage, &g.BackfillEligible, &g.CreatedAt, &g.UpdatedAt,
	)
	return g, err
}

func scanAlertGroupRows(rows pgx.Rows) (AlertGroup, error) {
	var g AlertGroup
	err := rows.Scan(
		&g.ID, &g.GroupKey, &g.ContractID, &g.Severity, &g.Rule,
		&g.Count, &g.DedupeWindowSecs, &g.FirstSeen, &g.LastSeen,
		&g.LastMessage, &g.BackfillEligible, &g.CreatedAt, &g.UpdatedAt,
	)
	return g, err
}

// ---- MockStore implementation ----------------------------------------------

// UpsertAlertGroup implements AlertGroupStore for MockStore.
func (m *MockStore) UpsertAlertGroup(_ context.Context, g AlertGroup) (AlertGroup, error) {
	for i, existing := range m.alertGroups {
		if existing.GroupKey == g.GroupKey {
			m.alertGroups[i].Count += g.Count
			m.alertGroups[i].LastSeen = g.LastSeen
			m.alertGroups[i].LastMessage = g.LastMessage
			m.alertGroups[i].UpdatedAt = time.Now().UTC()
			return m.alertGroups[i], nil
		}
	}
	g.ID = int64(len(m.alertGroups) + 1)
	g.CreatedAt = time.Now().UTC()
	g.UpdatedAt = g.CreatedAt
	m.alertGroups = append(m.alertGroups, g)
	return g, nil
}

// ListAlertGroups implements AlertGroupStore for MockStore.
func (m *MockStore) ListAlertGroups(_ context.Context, cursor string, limit int, f AlertGroupFilters) ([]AlertGroup, string, error) {
	if limit <= 0 {
		limit = 100
	}
	var cursorTS time.Time
	var cursorID int64
	if cursor != "" {
		ts, id, err := decodeGroupCursor(cursor)
		if err != nil {
			return nil, "", ErrInvalidCursor
		}
		cursorTS, cursorID = ts, id
	}

	// Copy and sort newest-first to match postgres ordering.
	all := make([]AlertGroup, len(m.alertGroups))
	copy(all, m.alertGroups)
	sort.Slice(all, func(i, j int) bool {
		if all[i].LastSeen.Equal(all[j].LastSeen) {
			return all[i].ID > all[j].ID
		}
		return all[i].LastSeen.After(all[j].LastSeen)
	})

	var out []AlertGroup
	for _, ag := range all {
		if f.ContractID != "" && ag.ContractID != f.ContractID {
			continue
		}
		if f.Severity != "" && ag.Severity != f.Severity {
			continue
		}
		if cursor != "" {
			if ag.LastSeen.After(cursorTS) {
				continue
			}
			if ag.LastSeen.Equal(cursorTS) && ag.ID >= cursorID {
				continue
			}
		}
		out = append(out, ag)
		if len(out) > limit {
			break
		}
	}
	var next string
	if len(out) > limit {
		last := out[limit-1]
		next = encodeGroupCursor(last.LastSeen, last.ID)
		out = out[:limit]
	}
	return out, next, nil
}
