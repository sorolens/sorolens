package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FullStore combines Store and QueryStore, both implemented by the postgres backend.
type FullStore interface {
	Store
	QueryStore
}

// NewFullStore returns a FullStore backed by the given pool.
func NewFullStore(pool *pgxpool.Pool) FullStore {
	return &postgresStore{pool: pool}
}

// EventFilters holds optional query filters for listing events.
type EventFilters struct {
	Type string
	From uint32
	To   uint32
}

// InvocationFilters holds optional query filters for listing invocations.
type InvocationFilters struct {
	Status       string
	FunctionName string
	From         uint32
	To           uint32
}

// StorageFilters holds optional query filters for listing storage entries.
type StorageFilters struct {
	Durability string
	Status     string
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

// QueryStore provides read-only querying methods needed by the HTTP API.
type QueryStore interface {
	ListEvents(ctx context.Context, contractID, cursor string, limit int, f EventFilters) ([]Event, string, error)
	ListInvocations(ctx context.Context, contractID, cursor string, limit int, f InvocationFilters) ([]Invocation, string, error)
	ListStorageEntries(ctx context.Context, contractID, cursor string, limit int, f StorageFilters) ([]StorageEntry, string, error)
	GetContractStats(ctx context.Context, contractID, window string) (ContractStats, error)
	RecentEvents(ctx context.Context, contractID string, limit int) ([]Event, error)
}

// ---- ListEvents --------------------------------------------------------------

func (s *postgresStore) ListEvents(ctx context.Context, contractID, cursor string, limit int, f EventFilters) ([]Event, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, contract_id, ledger, ledger_closed_at, tx_hash, type,
		       topic_xdr, value_xdr, topic_decoded, value_decoded,
		       in_successful_call, inserted_at
		FROM events
		WHERE contract_id = $1
		  AND ($2 = '' OR id > $2)
		  AND ($3 = '' OR type = $3)
		  AND ($4 = 0   OR ledger >= $4)
		  AND ($5 = 0   OR ledger <= $5)
		ORDER BY ledger ASC, id ASC
		LIMIT $6`,
		contractID, cursor, f.Type, f.From, f.To, limit+1,
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
			&e.ID, &e.ContractID, &e.Ledger, &e.LedgerClosedAt, &e.TxHash, &e.Type,
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

// ---- RecentEvents -----------------------------------------------------------

func (s *postgresStore) RecentEvents(ctx context.Context, contractID string, limit int) ([]Event, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, contract_id, ledger, ledger_closed_at, tx_hash, type,
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
			&e.ID, &e.ContractID, &e.Ledger, &e.LedgerClosedAt, &e.TxHash, &e.Type,
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

// ---- ListInvocations --------------------------------------------------------

func (s *postgresStore) ListInvocations(ctx context.Context, contractID, cursor string, limit int, f InvocationFilters) ([]Invocation, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT tx_hash, contract_id, ledger, ledger_closed_at, status,
		       function_name, args_decoded, result_decoded, result_xdr,
		       resource_fee_charged, cpu_insn, mem_byte,
		       ledger_read_byte, ledger_write_byte, application_order, inserted_at
		FROM invocations
		WHERE contract_id = $1
		  AND ($2 = '' OR tx_hash > $2)
		  AND ($3 = '' OR status = $3)
		  AND ($4 = '' OR function_name = $4)
		  AND ($5 = 0   OR ledger >= $5)
		  AND ($6 = 0   OR ledger <= $6)
		ORDER BY ledger ASC, tx_hash ASC
		LIMIT $7`,
		contractID, cursor, f.Status, f.FunctionName, f.From, f.To, limit+1,
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
			&inv.TxHash, &inv.ContractID, &inv.Ledger, &inv.LedgerClosedAt, &inv.Status,
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
		SELECT contract_id, key_xdr, key_decoded, value_xdr, value_decoded,
		       durability, live_until_ledger, last_modified_ledger, status, last_seen_at
		FROM storage_entries
		WHERE contract_id = $1
		  AND ($2 = '' OR key_xdr > $2)
		  AND ($3 = '' OR durability = $3)
		  AND ($4 = '' OR status = $4)
		ORDER BY key_xdr ASC
		LIMIT $5`,
		contractID, cursor, f.Durability, f.Status, limit+1,
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
			&se.ContractID, &se.KeyXDR, &keyDec, &se.ValueXDR, &valDec,
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
