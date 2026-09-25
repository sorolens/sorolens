package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FullStore combines every store interface: all of them are implemented by
// the postgres backend and the in-memory MockStore.
type FullStore interface {
	Store
	QueryStore
	WatchdogStore
	ContractUpgradeStore
	HealthScoreStore
	APIKeyStore
	AlertSubscriptionStore
	WatchlistStore
	UserStore
	PerformanceStore
	GlobalEventStore
}

// NewFullStore returns a FullStore backed by the given pool.
func NewFullStore(pool *pgxpool.Pool) FullStore {
	return &postgresStore{pool: pool}
}

// EventFilters holds optional query filters for listing events.
type EventFilters struct {
	Type    string
	Network string
	// Topic matches events whose decoded topic list contains the given value
	// (JSONB containment). See topicFilterJSON for the accepted encodings.
	Topic            string
	From             uint32
	To               uint32
	InSuccessfulCall *bool
}

// InvocationFilters holds optional query filters for listing invocations.
type InvocationFilters struct {
	Status       string
	FunctionName string
	Network      string
	From         uint32
	To           uint32
}

// StorageFilters holds optional query filters for listing storage entries.
type StorageFilters struct {
	Durability string
	Status     string
	Network    string
}

// ContractStats holds per-contract aggregated statistics.
type ContractStats struct {
	EventCount            int64
	InvocationCount       int64
	StorageCount          int64
	LastSyncedLedger      uint32
	WindowEventCount      int64
	WindowInvocationCount int64
	WindowDuration        string
}

// HourlyActivity holds one hour bucket of contract activity (hour start, UTC).
// It backs the indexer's anomaly detector window (issue #136).
type HourlyActivity struct {
	Hour        time.Time // hour start, UTC
	EventCount  int64
	InvokeCount int64
	CPU         int64 // sum of cpu_insn
	Fees        int64 // sum of resource fees, stroops
}

// DailyAggregate holds one calendar day of contract activity. Fees are the
// sum of resource fees charged across invocations (stroops). The forecast
// feature fits trend + weekly seasonality over these cheap aggregate scans
// rather than over raw rows.
type DailyAggregate struct {
	Day         time.Time // midnight UTC
	Fee         float64
	Invocations float64
	Events      float64
}

// QueryStore provides read-only querying methods needed by the HTTP API.
type QueryStore interface {
	ListEvents(ctx context.Context, contractID, cursor string, limit int, f EventFilters) ([]Event, string, error)
	ListInvocations(ctx context.Context, contractID, cursor string, limit int, f InvocationFilters) ([]Invocation, string, error)
	ListStorageEntries(ctx context.Context, contractID, cursor string, limit int, f StorageFilters) ([]StorageEntry, string, error)
	GetContractStats(ctx context.Context, contractID, window string) (ContractStats, error)
	RecentEvents(ctx context.Context, contractID string, limit int) ([]Event, error)
	// RecentInvocations returns the most recent invocations for a contract,
	// newest first, capped at limit. Like RecentEvents it backs single-call
	// "latest invocation" lookups (e.g. the dashboard summary) without walking
	// the ascending paginated list.
	RecentInvocations(ctx context.Context, contractID string, limit int) ([]Invocation, error)

	// ContractFirstLedger returns the earliest ledger for which the contract
	// has indexed data (events or invocations). It returns 0 when nothing has
	// been indexed yet, letting callers fall back to the contract's
	// created_at_ledger.
	ContractFirstLedger(ctx context.Context, contractID string) (uint32, error)
	// GetStorageSnapshot returns the storage entry version that was live at
	// the given ledger: for each key, the most recent version written at or
	// before ledger whose TTL had not yet expired.
	GetStorageSnapshot(ctx context.Context, contractID string, ledger uint32) ([]StorageEntry, error)
	// LastEventAtOrBefore returns the most recent event with ledger <= ledger,
	// or ErrNotFound when the contract has no such event.
	LastEventAtOrBefore(ctx context.Context, contractID string, ledger uint32) (Event, error)
	// DailyAggregates returns one row per calendar day for the most recent
	// `days` days (midnight UTC buckets), oldest first. Days with no activity
	// yield a zero aggregate rather than a gap, so the forecasting model can
	// fit a contiguous series.
	DailyAggregates(ctx context.Context, contractID string, days int) ([]DailyAggregate, error)
	// RecentHourlyActivity returns one row per hour bucket for the most recent
	// `hours` hours (hour-start UTC, oldest first). Hours with no activity
	// yield a zero bucket, providing a contiguous series to the indexer's
	// anomaly detector.
	RecentHourlyActivity(ctx context.Context, contractID string, hours int) ([]HourlyActivity, error)
}

// ---- ListEvents --------------------------------------------------------------

func (s *postgresStore) ListEvents(ctx context.Context, contractID, cursor string, limit int, f EventFilters) ([]Event, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	// The topic filter is appended only when set so the planner always sees a
	// plain `topic_decoded @> $n::jsonb` predicate and can use the GIN index
	// added in migration 000009. Wrapping it in a `($n = '' OR ...)` guard
	// like the other filters would hide the containment operator behind a
	// disjunction and force a sequential scan.
	args := []any{contractID, cursor, f.Network, f.Type, f.From, f.To, limit + 1}
	dynamicClauses := ""
	if f.Topic != "" {
		args = append(args, topicFilterJSON(f.Topic))
		dynamicClauses += fmt.Sprintf("  AND topic_decoded @> $%d::jsonb\n", len(args))
	}
	if f.InSuccessfulCall != nil {
		args = append(args, *f.InSuccessfulCall)
		dynamicClauses += fmt.Sprintf("  AND in_successful_call = $%d\n", len(args))
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
		       topic_xdr, value_xdr, topic_decoded, value_decoded,
		       in_successful_call, inserted_at
		FROM events
		WHERE contract_id = $1
		  AND ($2 = '' OR id > $2)
		  AND ($3 = '' OR network = $3)
		  AND ($4 = '' OR type = $4)
		  AND ($5 = 0   OR ledger >= $5)
		  AND ($6 = 0   OR ledger <= $6)
`+dynamicClauses+`		ORDER BY ledger ASC, id ASC
		LIMIT $7`,
		args...,
	)
	if err != nil {
		return nil, "", fmt.Errorf("list events: %w", err)
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

	var nextCursor string
	if len(out) > limit {
		nextCursor = out[limit-1].ID
		out = out[:limit]
	}
	return out, nextCursor, nil
}

// topicFilterJSON encodes a ?topic= value as a JSONB array for the containment
// operator used by ListEvents (`topic_decoded @> $n::jsonb`).
//
// The value is treated as JSON when it parses, so `123` matches the number 123
// and `"transfer"` matches the string "transfer". Anything that is not valid
// JSON is treated as a bare string, so `transfer` also matches "transfer".
// Decoded topics are stored as a JSON array, hence the array wrapper.
func topicFilterValue(topic string) any {
	var v any
	if err := json.Unmarshal([]byte(topic), &v); err != nil {
		return topic
	}
	return v
}

func topicFilterJSON(topic string) string {
	b, err := json.Marshal([]any{topicFilterValue(topic)})
	if err != nil {
		return "[]"
	}
	return string(b)
}

// ---- RecentEvents -----------------------------------------------------------

func (s *postgresStore) RecentEvents(ctx context.Context, contractID string, limit int) ([]Event, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
		       topic_xdr, value_xdr, topic_decoded, value_decoded,
		       in_successful_call, inserted_at
		FROM events
		WHERE contract_id = $1
		ORDER BY ledger DESC, id DESC
		LIMIT $2`,
		contractID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("recent events: %w", err)
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
			return nil, err
		}
		_ = json.Unmarshal(topicXDR, &e.TopicXDR)
		_ = json.Unmarshal(topicDec, &e.TopicDecoded)
		_ = json.Unmarshal(valDec, &e.ValueDecoded)
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---- RecentInvocations ------------------------------------------------------

// RecentInvocations returns the newest invocations for one contract, ordered
// by ledger and tx hash descending. It mirrors RecentEvents and is the
// single-row lookup the dashboard summary uses for "latest invocation".
func (s *postgresStore) RecentInvocations(ctx context.Context, contractID string, limit int) ([]Invocation, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		SELECT tx_hash, contract_id, network, ledger, ledger_closed_at, status,
		       function_name, args_decoded, result_decoded, result_xdr,
		       resource_fee_charged, cpu_insn, mem_byte,
		       ledger_read_byte, ledger_write_byte, application_order, inserted_at
		FROM invocations
		WHERE contract_id = $1
		ORDER BY ledger DESC, tx_hash DESC
		LIMIT $2`,
		contractID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("recent invocations: %w", err)
	}
	defer rows.Close()

	var out []Invocation
	for rows.Next() {
		var inv Invocation
		var argsDec, resultDec []byte
		if err := rows.Scan(
			&inv.TxHash, &inv.ContractID, &inv.Network, &inv.Ledger, &inv.LedgerClosedAt, &inv.Status,
			&inv.FunctionName, &argsDec, &resultDec, &inv.ResultXDR,
			&inv.ResourceFeeCharged, &inv.CPUInsn, &inv.MemByte,
			&inv.LedgerReadByte, &inv.LedgerWriteByte, &inv.ApplicationOrder, &inv.InsertedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(argsDec, &inv.ArgsDecoded)
		_ = json.Unmarshal(resultDec, &inv.ResultDecoded)
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (s *postgresStore) ListInvocations(ctx context.Context, contractID, cursor string, limit int, f InvocationFilters) ([]Invocation, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT tx_hash, contract_id, network, ledger, ledger_closed_at, status,
		       function_name, args_decoded, result_decoded, result_xdr,
		       resource_fee_charged, cpu_insn, mem_byte,
		       ledger_read_byte, ledger_write_byte, application_order, inserted_at
		FROM invocations
		WHERE contract_id = $1
		  AND ($2 = '' OR tx_hash > $2)
		  AND ($3 = '' OR network = $3)
		  AND ($4 = '' OR status = $4)
		  AND ($5 = '' OR function_name = $5)
		  AND ($6 = 0   OR ledger >= $6)
		  AND ($7 = 0   OR ledger <= $7)
		ORDER BY ledger ASC, tx_hash ASC
		LIMIT $8`,
		contractID, cursor, f.Network, f.Status, f.FunctionName, f.From, f.To, limit+1,
	)
	if err != nil {
		return nil, "", fmt.Errorf("list invocations: %w", err)
	}
	defer rows.Close()

	var out []Invocation
	for rows.Next() {
		var inv Invocation
		var argsDec, resultDec []byte
		if err := rows.Scan(
			&inv.TxHash, &inv.ContractID, &inv.Network, &inv.Ledger, &inv.LedgerClosedAt, &inv.Status,
			&inv.FunctionName, &argsDec, &resultDec, &inv.ResultXDR,
			&inv.ResourceFeeCharged, &inv.CPUInsn, &inv.MemByte,
			&inv.LedgerReadByte, &inv.LedgerWriteByte, &inv.ApplicationOrder, &inv.InsertedAt,
		); err != nil {
			return nil, "", err
		}
		_ = json.Unmarshal(argsDec, &inv.ArgsDecoded)
		_ = json.Unmarshal(resultDec, &inv.ResultDecoded)
		out = append(out, inv)
	}
	if rows.Err() != nil {
		return nil, "", rows.Err()
	}

	var nextCursor string
	if len(out) > limit {
		nextCursor = out[limit-1].TxHash
		out = out[:limit]
	}
	return out, nextCursor, nil
}

// ---- ListStorageEntries -----------------------------------------------------

func (s *postgresStore) ListStorageEntries(ctx context.Context, contractID, cursor string, limit int, f StorageFilters) ([]StorageEntry, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT contract_id, network, key_xdr, key_decoded, value_xdr, value_decoded,
		       durability, live_until_ledger, last_modified_ledger, status, last_seen_at
		FROM storage_entries
		WHERE contract_id = $1
		  AND ($2 = '' OR key_xdr > $2)
		  AND ($3 = '' OR network = $3)
		  AND ($4 = '' OR durability = $4)
		  AND ($5 = '' OR status = $5)
		ORDER BY key_xdr ASC
		LIMIT $6`,
		contractID, cursor, f.Network, f.Durability, f.Status, limit+1,
	)
	if err != nil {
		return nil, "", fmt.Errorf("list storage entries: %w", err)
	}
	defer rows.Close()

	var out []StorageEntry
	for rows.Next() {
		var se StorageEntry
		var keyDec, valDec []byte
		if err := rows.Scan(
			&se.ContractID, &se.Network, &se.KeyXDR, &keyDec, &se.ValueXDR, &valDec,
			&se.Durability, &se.LiveUntilLedger, &se.LastModifiedLedger, &se.Status, &se.LastSeenAt,
		); err != nil {
			return nil, "", err
		}
		_ = json.Unmarshal(keyDec, &se.KeyDecoded)
		_ = json.Unmarshal(valDec, &se.ValueDecoded)
		out = append(out, se)
	}
	if rows.Err() != nil {
		return nil, "", rows.Err()
	}

	var nextCursor string
	if len(out) > limit {
		nextCursor = out[limit-1].KeyXDR
		out = out[:limit]
	}
	return out, nextCursor, nil
}

// ---- GetContractStats -------------------------------------------------------

func windowToInterval(window string) string {
	switch window {
	case "7d":
		return "7 days"
	case "30d":
		return "30 days"
	default:
		return "24 hours"
	}
}

func (s *postgresStore) GetContractStats(ctx context.Context, contractID, window string) (ContractStats, error) {
	interval := windowToInterval(window)
	row := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM events          WHERE contract_id = $1) AS event_count,
			(SELECT COUNT(*) FROM invocations     WHERE contract_id = $1) AS invocation_count,
			(SELECT COUNT(*) FROM storage_entries WHERE contract_id = $1) AS storage_count,
			COALESCE((SELECT last_ledger FROM sync_state WHERE contract_id = $1), 0) AS last_synced_ledger,
			(SELECT COUNT(*) FROM events      WHERE contract_id = $1 AND ledger_closed_at >= NOW() - $2::interval) AS window_events,
			(SELECT COUNT(*) FROM invocations WHERE contract_id = $1 AND ledger_closed_at >= NOW() - $2::interval) AS window_invocations`,
		contractID, interval,
	)
	var cs ContractStats
	err := row.Scan(
		&cs.EventCount, &cs.InvocationCount, &cs.StorageCount,
		&cs.LastSyncedLedger,
		&cs.WindowEventCount, &cs.WindowInvocationCount,
	)
	cs.WindowDuration = window
	if cs.WindowDuration == "" {
		cs.WindowDuration = "24h"
	}
	return cs, err
}

// DailyAggregates queries the last `days` calendar days (oldest first),
// coalescing fee/invocation sums from invocations and event counts from
// events. generate_series guarantees a row for every day in the window.
func (s *postgresStore) DailyAggregates(ctx context.Context, contractID string, days int) ([]DailyAggregate, error) {
	if days <= 0 {
		days = 90
	}
	if days > 3650 {
		days = 3650
	}
	rows, err := s.pool.Query(ctx, `
		WITH buckets AS (
			SELECT date_trunc('day', d)::date AS day
			FROM generate_series(now() - ($2::int || ' days')::interval, now(), '1 day') AS d
		)
		SELECT b.day,
		       COALESCE(inv.fee, 0)          AS fee,
		       COALESCE(inv.invocations, 0)  AS invocations,
		       COALESCE(ev.events, 0)        AS events
		FROM buckets b
		LEFT JOIN (
			SELECT date_trunc('day', ledger_closed_at)::date AS day,
			       COALESCE(SUM(resource_fee_charged), 0)    AS fee,
			       COUNT(*)                                   AS invocations
			FROM invocations
			WHERE contract_id = $1
			  AND ledger_closed_at >= now() - ($2::int || ' days')::interval
			GROUP BY 1
		) inv ON inv.day = b.day
		LEFT JOIN (
			SELECT date_trunc('day', ledger_closed_at)::date AS day,
			       COUNT(*)                                   AS events
			FROM events
			WHERE contract_id = $1
			  AND ledger_closed_at >= now() - ($2::int || ' days')::interval
			GROUP BY 1
		) ev ON ev.day = b.day
		ORDER BY b.day ASC`,
		contractID, days,
	)
	if err != nil {
		return nil, fmt.Errorf("daily aggregates: %w", err)
	}
	defer rows.Close()

	var out []DailyAggregate
	for rows.Next() {
		var a DailyAggregate
		var day time.Time
		if err := rows.Scan(&day, &a.Fee, &a.Invocations, &a.Events); err != nil {
			return nil, fmt.Errorf("daily aggregates scan: %w", err)
		}
		a.Day = day
		out = append(out, a)
	}
	return out, rows.Err()
}

// ---- ContractFirstLedger ----------------------------------------------------

func (s *postgresStore) ContractFirstLedger(ctx context.Context, contractID string) (uint32, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT COALESCE(MIN(ledger), 0) FROM (
			SELECT ledger FROM events      WHERE contract_id = $1
			UNION ALL
			SELECT ledger FROM invocations WHERE contract_id = $1
		) AS l`, contractID)
	var first uint32
	if err := row.Scan(&first); err != nil {
		return 0, fmt.Errorf("contract first ledger: %w", err)
	}
	return first, nil
}

// RecentHourlyActivity queries the most recent `hours` hourly buckets
// (oldest first), coalescing event counts from events and invocation/CPU/fee
// totals from invocations. generate_series guarantees a zero bucket for every
// hour even when the contract was idle, giving the indexer's anomaly detector
// a contiguous baseline series.
func (s *postgresStore) RecentHourlyActivity(ctx context.Context, contractID string, hours int) ([]HourlyActivity, error) {
	if hours <= 0 {
		hours = 24
	}
	if hours > 24*31 {
		hours = 24 * 31
	}
	rows, err := s.pool.Query(ctx, `
		WITH buckets AS (
			SELECT date_trunc('hour', d) AS hour
			FROM generate_series(now() - ($2::int || ' hours')::interval, now(), '1 hour') AS d
		)
		SELECT b.hour,
		       COALESCE(ev.events, 0)      AS events,
		       COALESCE(inv.invocations, 0) AS invocations,
		       COALESCE(inv.cpu, 0)         AS cpu,
		       COALESCE(inv.fees, 0)        AS fees
		FROM buckets b
		LEFT JOIN (
			SELECT date_trunc('hour', ledger_closed_at) AS hour,
			       COUNT(*)                             AS events
			FROM events
			WHERE contract_id = $1
			  AND ledger_closed_at >= now() - ($2::int || ' hours')::interval
			GROUP BY 1
		) ev ON ev.hour = b.hour
		LEFT JOIN (
			SELECT date_trunc('hour', ledger_closed_at) AS hour,
			       COUNT(*)                             AS invocations,
			       COALESCE(SUM(cpu_insn), 0)           AS cpu,
			       COALESCE(SUM(resource_fee_charged), 0) AS fees
			FROM invocations
			WHERE contract_id = $1
			  AND ledger_closed_at >= now() - ($2::int || ' hours')::interval
			GROUP BY 1
		) inv ON inv.hour = b.hour
		ORDER BY b.hour ASC`,
		contractID, hours,
	)
	if err != nil {
		return nil, fmt.Errorf("hourly activity: %w", err)
	}
	defer rows.Close()

	var out []HourlyActivity
	for rows.Next() {
		var a HourlyActivity
		var cpu, fees float64
		if err := rows.Scan(&a.Hour, &a.EventCount, &a.InvokeCount, &cpu, &fees); err != nil {
			return nil, fmt.Errorf("hourly activity scan: %w", err)
		}
		a.CPU = int64(cpu)
		a.Fees = int64(fees)
		out = append(out, a)
	}
	return out, rows.Err()
}

// ---- GetStorageSnapshot -----------------------------------------------------

func (s *postgresStore) GetStorageSnapshot(ctx context.Context, contractID string, ledger uint32) ([]StorageEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (key_xdr)
		       contract_id, key_xdr, key_decoded, value_xdr, value_decoded,
		       durability, live_until_ledger, last_modified_ledger, status, recorded_at
		FROM storage_entry_history
		WHERE contract_id = $1
		  AND (last_modified_ledger IS NULL OR last_modified_ledger <= $2)
		  AND (live_until_ledger IS NULL OR live_until_ledger >= $2)
		ORDER BY key_xdr ASC, last_modified_ledger DESC NULLS LAST`,
		contractID, ledger,
	)
	if err != nil {
		return nil, fmt.Errorf("get storage snapshot: %w", err)
	}
	defer rows.Close()

	var out []StorageEntry
	for rows.Next() {
		var se StorageEntry
		var keyDec, valDec []byte
		if err := rows.Scan(
			&se.ContractID, &se.KeyXDR, &keyDec, &se.ValueXDR, &valDec,
			&se.Durability, &se.LiveUntilLedger, &se.LastModifiedLedger, &se.Status, &se.LastSeenAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(keyDec, &se.KeyDecoded)
		_ = json.Unmarshal(valDec, &se.ValueDecoded)
		out = append(out, se)
	}
	return out, rows.Err()
}

// ---- LastEventAtOrBefore ----------------------------------------------------

func (s *postgresStore) LastEventAtOrBefore(ctx context.Context, contractID string, ledger uint32) (Event, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
		       topic_xdr, value_xdr, topic_decoded, value_decoded,
		       in_successful_call, inserted_at
		FROM events
		WHERE contract_id = $1 AND ledger <= $2
		ORDER BY ledger DESC, id DESC
		LIMIT 1`, contractID, ledger)

	var e Event
	var topicXDR, topicDec, valDec []byte
	err := row.Scan(
		&e.ID, &e.ContractID, &e.Network, &e.Ledger, &e.LedgerClosedAt, &e.TxHash, &e.Type,
		&topicXDR, &e.ValueXDR, &topicDec, &valDec,
		&e.InSuccessfulCall, &e.InsertedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	if err != nil {
		return Event{}, err
	}
	_ = json.Unmarshal(topicXDR, &e.TopicXDR)
	_ = json.Unmarshal(topicDec, &e.TopicDecoded)
	_ = json.Unmarshal(valDec, &e.ValueDecoded)
	return e, nil
}
