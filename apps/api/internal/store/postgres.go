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

// ErrNotFound is returned by Get* methods when no row matches.
var ErrNotFound = errors.New("store: not found")

type postgresStore struct {
	pool *pgxpool.Pool
}

// ---- contracts ------------------------------------------------------------

// UpsertContract inserts or updates a contract in the database. It uses the contract ID as the unique constraint for upserting.
func (s *postgresStore) UpsertContract(ctx context.Context, c Contract) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contracts
			(id, network, label, wasm_hash, created_at_ledger, backfill_complete_at, status, added_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET
			network             = EXCLUDED.network,
			label               = EXCLUDED.label,
			wasm_hash           = EXCLUDED.wasm_hash,
			created_at_ledger   = EXCLUDED.created_at_ledger,
			backfill_complete_at = EXCLUDED.backfill_complete_at,
			status              = EXCLUDED.status`,
		c.ID, c.Network, c.Label, c.WasmHash,
		c.CreatedAtLedger, c.BackfillCompleteAt, c.Status, c.AddedAt,
	)
	return err
}

// GetContract retrieves a contract by its ID. Returns ErrNotFound if no contract exists.
func (s *postgresStore) GetContract(ctx context.Context, contractID string) (Contract, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, network, label, wasm_hash, created_at_ledger,
		       backfill_complete_at, status, added_at
		FROM contracts WHERE id = $1`, contractID)
	var c Contract
	err := row.Scan(
		&c.ID, &c.Network, &c.Label, &c.WasmHash, &c.CreatedAtLedger,
		&c.BackfillCompleteAt, &c.Status, &c.AddedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Contract{}, ErrNotFound
	}
	return c, err
}

// ListContracts returns a list of contracts, ordered by ID. The cursor is the last-seen contract ID (lexicographic order).
func (s *postgresStore) ListContracts(ctx context.Context, cursor string, limit int) ([]Contract, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	// cursor is the last-seen contract ID (lexicographic order).
	rows, err := s.pool.Query(ctx, `
		SELECT id, network, label, wasm_hash, created_at_ledger,
		       backfill_complete_at, status, added_at
		FROM contracts
		WHERE ($1 = '' OR id > $1)
		ORDER BY id ASC
		LIMIT $2`, cursor, limit+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out []Contract
	for rows.Next() {
		var c Contract
		if err := rows.Scan(
			&c.ID, &c.Network, &c.Label, &c.WasmHash, &c.CreatedAtLedger,
			&c.BackfillCompleteAt, &c.Status, &c.AddedAt,
		); err != nil {
			return nil, "", err
		}
		out = append(out, c)
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

// ---- events ---------------------------------------------------------------

// BatchInsertEvents inserts multiple events in a single batch operation. It ignores duplicate events based on the primary key (id).
func (s *postgresStore) BatchInsertEvents(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, e := range events {
		topicJSON, err := json.Marshal(e.TopicXDR)
		if err != nil {
			return fmt.Errorf("marshal topic_xdr for event %s: %w", e.ID, err)
		}
		topicDecJSON, _ := json.Marshal(e.TopicDecoded)
		valDecJSON, _ := json.Marshal(e.ValueDecoded)

		batch.Queue(`
			INSERT INTO events
				(id, contract_id, ledger, ledger_closed_at, tx_hash, type,
				 topic_xdr, value_xdr, topic_decoded, value_decoded,
				 in_successful_call, inserted_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			ON CONFLICT (id) DO NOTHING`,
			e.ID, e.ContractID, e.Ledger, e.LedgerClosedAt, e.TxHash, e.Type,
			topicJSON, e.ValueXDR, topicDecJSON, valDecJSON,
			e.InSuccessfulCall, time.Now(),
		)
	}

	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range events {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("batch insert events: %w", err)
		}
	}
	return nil
}

// ---- invocations ----------------------------------------------------------

// BatchInsertInvocations inserts multiple invocations in a single batch operation. It ignores duplicate invocations based on the primary key (tx_hash).
func (s *postgresStore) BatchInsertInvocations(ctx context.Context, invocations []Invocation) error {
	if len(invocations) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, inv := range invocations {
		argsJSON, _ := json.Marshal(inv.ArgsDecoded)
		resultJSON, _ := json.Marshal(inv.ResultDecoded)

		batch.Queue(`
			INSERT INTO invocations
				(tx_hash, contract_id, ledger, ledger_closed_at, status,
				 function_name, args_decoded, result_decoded, result_xdr,
				 resource_fee_charged, cpu_insn, mem_byte,
				 ledger_read_byte, ledger_write_byte, application_order, inserted_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
			ON CONFLICT (tx_hash) DO NOTHING`,
			inv.TxHash, inv.ContractID, inv.Ledger, inv.LedgerClosedAt, inv.Status,
			inv.FunctionName, argsJSON, resultJSON, inv.ResultXDR,
			inv.ResourceFeeCharged, inv.CPUInsn, inv.MemByte,
			inv.LedgerReadByte, inv.LedgerWriteByte, inv.ApplicationOrder, time.Now(),
		)
	}

	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range invocations {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("batch insert invocations: %w", err)
		}
	}
	return nil
}

// ---- storage entries ------------------------------------------------------

// UpsertStorageEntries inserts or updates multiple storage entries in a single batch operation. It uses the contract_id and key_xdr as the unique constraint for upserting.
func (s *postgresStore) UpsertStorageEntries(ctx context.Context, entries []StorageEntry) error {
	if len(entries) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, e := range entries {
		keyDecJSON, _ := json.Marshal(e.KeyDecoded)
		valDecJSON, _ := json.Marshal(e.ValueDecoded)

		batch.Queue(`
			INSERT INTO storage_entries
				(contract_id, key_xdr, key_decoded, value_xdr, value_decoded,
				 durability, live_until_ledger, last_modified_ledger, status, last_seen_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT (contract_id, key_xdr) DO UPDATE SET
				key_decoded         = EXCLUDED.key_decoded,
				value_xdr           = EXCLUDED.value_xdr,
				value_decoded       = EXCLUDED.value_decoded,
				live_until_ledger   = EXCLUDED.live_until_ledger,
				last_modified_ledger = EXCLUDED.last_modified_ledger,
				status              = EXCLUDED.status,
				last_seen_at        = EXCLUDED.last_seen_at`,
			e.ContractID, e.KeyXDR, keyDecJSON, e.ValueXDR, valDecJSON,
			e.Durability, e.LiveUntilLedger, e.LastModifiedLedger, e.Status, time.Now(),
		)
	}

	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range entries {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("upsert storage entries: %w", err)
		}
	}
	return nil
}

// ---- sync state -----------------------------------------------------------

// GetSyncState retrieves the sync state for a given contract ID. If no sync state exists, it returns a default SyncState with the contract ID set.
func (s *postgresStore) GetSyncState(ctx context.Context, contractID string) (SyncState, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT contract_id, last_ledger, last_run_at, error_message, updated_at
		FROM sync_state WHERE contract_id = $1`, contractID)
	var ss SyncState
	err := row.Scan(&ss.ContractID, &ss.LastLedger, &ss.LastRunAt, &ss.ErrorMessage, &ss.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SyncState{ContractID: contractID}, nil
	}
	return ss, err
}

func (s *postgresStore) UpsertSyncState(ctx context.Context, ss SyncState) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sync_state (contract_id, last_ledger, last_run_at, error_message, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (contract_id) DO UPDATE SET
			last_ledger   = EXCLUDED.last_ledger,
			last_run_at   = EXCLUDED.last_run_at,
			error_message = EXCLUDED.error_message,
			updated_at    = EXCLUDED.updated_at`,
		ss.ContractID, ss.LastLedger, ss.LastRunAt, ss.ErrorMessage, now,
	)
	return err
}

// ---- global stats ---------------------------------------------------------

// GetGlobalStats retrieves aggregated statistics about the tracked contracts, events, invocations, and storage entries.
func (s *postgresStore) GetGlobalStats(ctx context.Context) (GlobalStats, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM contracts)       AS tracked_contracts,
			(SELECT COUNT(*) FROM events)          AS total_events,
			(SELECT COUNT(*) FROM invocations)     AS total_invocations,
			(SELECT COUNT(*) FROM storage_entries) AS total_storage_entries`)
	var g GlobalStats
	err := row.Scan(&g.TrackedContracts, &g.TotalEvents, &g.TotalInvocations, &g.TotalStorageEntries)
	return g, err
}
