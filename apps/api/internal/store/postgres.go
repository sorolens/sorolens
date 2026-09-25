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

// ListContracts returns a list of contracts matching the optional filters,
// ordered by ID. The cursor is the last-seen contract ID (lexicographic order).
func (s *postgresStore) ListContracts(ctx context.Context, cursor string, limit int, f ContractFilters) ([]Contract, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	// cursor is the last-seen contract ID (lexicographic order).
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.network, c.label, c.wasm_hash, c.created_at_ledger,
		       c.backfill_complete_at, c.status, c.added_at, activity.last_activity_at
		FROM contracts c
		LEFT JOIN (
			SELECT contract_id, MAX(ledger_closed_at) AS last_activity_at
			FROM (
				SELECT contract_id, ledger_closed_at FROM events
				UNION ALL
				SELECT contract_id, ledger_closed_at FROM invocations
			) activity_rows
			GROUP BY contract_id
		) activity ON activity.contract_id = c.id
		WHERE ($1 = '' OR c.id > $1)
		  AND ($2 = '' OR c.network = $2)
		  AND ($3 = '' OR c.status = $3)
		ORDER BY c.id ASC
		LIMIT $4`, cursor, f.Network, f.Status, limit+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out []Contract
	for rows.Next() {
		var c Contract
		if err := rows.Scan(
			&c.ID, &c.Network, &c.Label, &c.WasmHash, &c.CreatedAtLedger,
			&c.BackfillCompleteAt, &c.Status, &c.AddedAt, &c.LastActivityAt,
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

// networkOrDefault normalizes an empty network to the testnet default so
// callers that predate multi-network support keep writing valid rows.
func networkOrDefault(network string) string {
	if network == "" {
		return "testnet"
	}
	return network
}

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
				(id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
				 topic_xdr, value_xdr, topic_decoded, value_decoded,
				 in_successful_call, inserted_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			ON CONFLICT (id) DO NOTHING`,
			e.ID, e.ContractID, networkOrDefault(e.Network), e.Ledger, e.LedgerClosedAt, e.TxHash, e.Type,
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
				(tx_hash, contract_id, network, ledger, ledger_closed_at, status,
				 function_name, args_decoded, result_decoded, result_xdr,
				 resource_fee_charged, cpu_insn, mem_byte,
				 ledger_read_byte, ledger_write_byte, application_order, inserted_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
			ON CONFLICT (tx_hash) DO NOTHING`,
			inv.TxHash, inv.ContractID, networkOrDefault(inv.Network), inv.Ledger, inv.LedgerClosedAt, inv.Status,
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
				(contract_id, network, key_xdr, key_decoded, value_xdr, value_decoded,
				 durability, live_until_ledger, last_modified_ledger, status, last_seen_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (contract_id, key_xdr) DO UPDATE SET
				network             = EXCLUDED.network,
				key_decoded         = EXCLUDED.key_decoded,
				value_xdr           = EXCLUDED.value_xdr,
				value_decoded       = EXCLUDED.value_decoded,
				live_until_ledger   = EXCLUDED.live_until_ledger,
				last_modified_ledger = EXCLUDED.last_modified_ledger,
				status              = EXCLUDED.status,
				last_seen_at        = EXCLUDED.last_seen_at`,
			e.ContractID, networkOrDefault(e.Network), e.KeyXDR, keyDecJSON, e.ValueXDR, valDecJSON,
			e.Durability, e.LiveUntilLedger, e.LastModifiedLedger, e.Status, time.Now(),
		)

		// Append a versioned row so the snapshot/replay endpoint can recover
		// the value that was live at any historical ledger.
		batch.Queue(`
			INSERT INTO storage_entry_history
				(contract_id, key_xdr, key_decoded, value_xdr, value_decoded,
				 durability, live_until_ledger, last_modified_ledger, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (contract_id, key_xdr, last_modified_ledger) DO UPDATE SET
				key_decoded         = EXCLUDED.key_decoded,
				value_xdr           = EXCLUDED.value_xdr,
				value_decoded       = EXCLUDED.value_decoded,
				live_until_ledger   = EXCLUDED.live_until_ledger,
				status              = EXCLUDED.status`,
			e.ContractID, e.KeyXDR, keyDecJSON, e.ValueXDR, valDecJSON,
			e.Durability, e.LiveUntilLedger, e.LastModifiedLedger, e.Status,
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

// ---- indexer cursors ------------------------------------------------------

// GetIndexerCursor retrieves the last committed ledger for a network.
func (s *postgresStore) GetIndexerCursor(ctx context.Context, network string) (uint32, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT ledger
		FROM indexer_cursors
		WHERE network = $1`, networkOrDefault(network))
	var ledger int64
	err := row.Scan(&ledger)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get indexer cursor: %w", err)
	}
	return uint32(ledger), nil
}

// SetIndexerCursor updates the last committed ledger for a network.
func (s *postgresStore) SetIndexerCursor(ctx context.Context, network string, ledger uint32) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO indexer_cursors (network, ledger, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (network) DO UPDATE SET
			ledger = EXCLUDED.ledger,
			updated_at = NOW()`, networkOrDefault(network), ledger)
	if err != nil {
		return fmt.Errorf("set indexer cursor: %w", err)
	}
	return nil
}

// BatchInsertWithCursor atomically writes events, invocations, contract sync state,
// and advances the network indexer cursor within a single database transaction.
func (s *postgresStore) BatchInsertWithCursor(ctx context.Context, network string, ledger uint32, events []Event, invocations []Invocation, syncState SyncState) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // safe if committed

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
				(id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
				 topic_xdr, value_xdr, topic_decoded, value_decoded,
				 in_successful_call, inserted_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			ON CONFLICT (id) DO NOTHING`,
			e.ID, e.ContractID, networkOrDefault(e.Network), e.Ledger, e.LedgerClosedAt, e.TxHash, e.Type,
			topicJSON, e.ValueXDR, topicDecJSON, valDecJSON,
			e.InSuccessfulCall, time.Now(),
		)
	}

	for _, inv := range invocations {
		argsJSON, _ := json.Marshal(inv.ArgsDecoded)
		resultJSON, _ := json.Marshal(inv.ResultDecoded)

		batch.Queue(`
			INSERT INTO invocations
				(tx_hash, contract_id, network, ledger, ledger_closed_at, status,
				 function_name, args_decoded, result_decoded, result_xdr,
				 resource_fee_charged, cpu_insn, mem_byte,
				 ledger_read_byte, ledger_write_byte, application_order, inserted_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
			ON CONFLICT (tx_hash) DO NOTHING`,
			inv.TxHash, inv.ContractID, networkOrDefault(inv.Network), inv.Ledger, inv.LedgerClosedAt, inv.Status,
			inv.FunctionName, argsJSON, resultJSON, inv.ResultXDR,
			inv.ResourceFeeCharged, inv.CPUInsn, inv.MemByte,
			inv.LedgerReadByte, inv.LedgerWriteByte, inv.ApplicationOrder, time.Now(),
		)
	}

	if syncState.ContractID != "" {
		batch.Queue(`
			INSERT INTO sync_state (contract_id, last_ledger, last_run_at, error_message, updated_at)
			VALUES ($1, $2, NOW(), NULL, NOW())
			ON CONFLICT (contract_id) DO UPDATE SET
				last_ledger   = EXCLUDED.last_ledger,
				last_run_at   = NOW(),
				error_message = NULL,
				updated_at    = NOW()`,
			syncState.ContractID, syncState.LastLedger,
		)
	}

	batch.Queue(`
		INSERT INTO indexer_cursors (network, ledger, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (network) DO UPDATE SET
			ledger     = EXCLUDED.ledger,
			updated_at = NOW()`,
		networkOrDefault(network), ledger,
	)

	br := tx.SendBatch(ctx, batch)
	totalQueued := len(events) + len(invocations)
	if syncState.ContractID != "" {
		totalQueued++
	}
	totalQueued++ // indexer_cursors

	for i := 0; i < totalQueued; i++ {
		if _, err := br.Exec(); err != nil {
			br.Close()
			return fmt.Errorf("exec batch item %d: %w", i, err)
		}
	}
	if err := br.Close(); err != nil {
		return fmt.Errorf("close batch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit batch: %w", err)
	}
	return nil
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

// ---- partition management -------------------------------------------------

// CreateNextMonthPartition creates the partition for next month if it does not exist.
func (s *postgresStore) CreateNextMonthPartition(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `SELECT create_next_month_partition()`)
	return err
}

// CreateMonthlyPartitionIfNotExists creates a partition for the given year/month if it does not exist.
func (s *postgresStore) CreateMonthlyPartitionIfNotExists(ctx context.Context, year int, month int) error {
	_, err := s.pool.Exec(ctx, `SELECT create_monthly_partition($1, $2)`, year, month)
	return err
}

// GetPartitionStats returns information about all existing partitions of the events table.
func (s *postgresStore) GetPartitionStats(ctx context.Context) ([]PartitionStats, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			c.relname AS partition_name,
			pg_total_relation_size(c.oid) AS table_size,
			c.reltuples AS row_count
		FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid
		JOIN pg_class p ON p.oid = i.inhparent
		WHERE p.relname = 'events'
		ORDER BY c.relname`)
	if err != nil {
		return nil, fmt.Errorf("get partition stats: %w", err)
	}
	defer rows.Close()

	var out []PartitionStats
	for rows.Next() {
		var ps PartitionStats
		if err := rows.Scan(&ps.PartitionName, &ps.TableSize, &ps.RowCount); err != nil {
			return nil, err
		}
		_, err := fmt.Sscanf(ps.PartitionName, "events_%d_%d", &ps.Year, &ps.Month)
		if err != nil {
			ps.Year = 0
			ps.Month = 0
		}
		out = append(out, ps)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}

// ---- watchlist ------------------------------------------------------------

func (s *postgresStore) AddToWatchlist(ctx context.Context, userID, contractID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO watchlist_items (user_id, contract_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, contract_id) DO NOTHING`,
		userID, contractID,
	)
	return err
}

func (s *postgresStore) RemoveFromWatchlist(ctx context.Context, userID, contractID string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM watchlist_items WHERE user_id = $1 AND contract_id = $2`,
		userID, contractID,
	)
	return err
}

func (s *postgresStore) ListWatchlist(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT contract_id FROM watchlist_items
		WHERE user_id = $1 ORDER BY added_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var cid string
		if err := rows.Scan(&cid); err != nil {
			return nil, err
		}
		out = append(out, cid)
	}
	return out, rows.Err()
}

func (s *postgresStore) IsInWatchlist(ctx context.Context, userID, contractID string) (bool, error) {
	var count int64
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM watchlist_items
		WHERE user_id = $1 AND contract_id = $2`,
		userID, contractID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
