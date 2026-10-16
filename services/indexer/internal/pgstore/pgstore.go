// Package pgstore is the indexer's Postgres-backed poller.Store (issue #500).
//
// It writes to the schema in apps/api/internal/db/migrations, the same tables
// the API reads, and its SQL mirrors apps/api/internal/store so the two
// cannot drift on column names or types. That package cannot be imported from
// here: it sits under apps/api/internal and belongs to a different Go module,
// so Go's internal-package rule forbids the import.
//
// Every batch insert is idempotent. Rows are keyed by their natural primary
// key and conflicts are ignored, so re-indexing a ledger window after a crash
// neither duplicates rows nor fails.
package pgstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

// storageHealthyHorizonLedgers mirrors the constant in
// apps/api/internal/store/health_score.go: a live storage entry expiring
// within this many ledgers (~1 day) counts against the TTL health component.
const storageHealthyHorizonLedgers int64 = 17_280

// connectTimeout bounds the startup connectivity check.
const connectTimeout = 10 * time.Second

// Compile-time check that Store satisfies the poller's interface.
var _ poller.Store = (*Store)(nil)

// Store implements poller.Store on a pgx connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// New wraps an existing pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Connect opens a pool for dsn and pings the database, so a bad DSN or an
// unreachable server fails at startup with a clear error rather than on the
// first indexing pass.
func Connect(ctx context.Context, dsn string) (*Store, error) {
	if dsn == "" {
		return nil, errors.New("pgstore: DATABASE_URL is not set")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("pgstore: invalid DATABASE_URL: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pgstore: create pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pgstore: database unreachable at %s:%d: %w", cfg.ConnConfig.Host, cfg.ConnConfig.Port, err)
	}
	return New(pool), nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Pool exposes the underlying pool (used by tests and future store surfaces).
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

func networkOrDefault(network string) string {
	if network == "" {
		return "testnet"
	}
	return network
}

// ---- contracts ------------------------------------------------------------

// ListContracts returns up to limit contracts ordered by id. cursor is the
// last id of the previous page ("" for the first); the returned cursor is ""
// once the final page has been read.
func (s *Store) ListContracts(ctx context.Context, cursor string, limit int) ([]poller.Contract, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, status, network, COALESCE(wasm_hash, '')
		FROM contracts
		WHERE ($1 = '' OR id > $1)
		ORDER BY id ASC
		LIMIT $2`, cursor, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list contracts: %w", err)
	}
	defer rows.Close()

	var out []poller.Contract
	for rows.Next() {
		var c poller.Contract
		if err := rows.Scan(&c.ID, &c.Status, &c.Network, &c.WasmHash); err != nil {
			return nil, "", fmt.Errorf("list contracts scan: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("list contracts: %w", err)
	}

	var next string
	if len(out) > limit {
		out = out[:limit]
		next = out[limit-1].ID
	}
	return out, next, nil
}

// UpdateContractWasmHash records the contract's current on-chain Wasm hash.
func (s *Store) UpdateContractWasmHash(ctx context.Context, contractID, wasmHash string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE contracts SET wasm_hash = $2 WHERE id = $1`, contractID, wasmHash)
	if err != nil {
		return fmt.Errorf("update contract wasm hash: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update contract wasm hash: contract %s not found", contractID)
	}
	return nil
}

// ---- events and invocations -----------------------------------------------

// BatchInsertEvents inserts events, ignoring rows that already exist.
//
// events is partitioned by ledger_closed_at with primary key
// (id, ledger_closed_at), so the conflict clause has no target: it matches
// that key on the partitioned table and plain id on an unpartitioned one.
func (s *Store) BatchInsertEvents(ctx context.Context, events []poller.Event) error {
	if len(events) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	now := time.Now()
	for _, e := range events {
		topics := e.TopicXDR
		if topics == nil {
			topics = []string{}
		}
		topicJSON, err := json.Marshal(topics)
		if err != nil {
			return fmt.Errorf("marshal topic_xdr for event %s: %w", e.ID, err)
		}
		batch.Queue(`
			INSERT INTO events
				(id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
				 topic_xdr, value_xdr, in_successful_call, inserted_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT DO NOTHING`,
			e.ID, e.ContractID, networkOrDefault(e.Network), int64(e.Ledger), e.LedgerClosedAt, e.TxHash, e.Type,
			topicJSON, e.ValueXDR, e.InSuccessfulCall, now,
		)
	}
	return execBatch(ctx, s.pool, batch, "batch insert events")
}

// BatchInsertInvocations inserts invocations, ignoring tx hashes already
// stored.
func (s *Store) BatchInsertInvocations(ctx context.Context, invocations []poller.Invocation) error {
	if len(invocations) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	now := time.Now()
	for _, inv := range invocations {
		batch.Queue(`
			INSERT INTO invocations
				(tx_hash, contract_id, network, ledger, ledger_closed_at, status,
				 result_xdr, application_order, inserted_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (tx_hash) DO NOTHING`,
			inv.TxHash, inv.ContractID, networkOrDefault(inv.Network), int64(inv.Ledger), inv.LedgerClosedAt, inv.Status,
			inv.ResultXDR, inv.ApplicationOrder, now,
		)
	}
	return execBatch(ctx, s.pool, batch, "batch insert invocations")
}

func execBatch(ctx context.Context, pool *pgxpool.Pool, batch *pgx.Batch, what string) error {
	br := pool.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			br.Close() //nolint:errcheck
			return fmt.Errorf("%s: %w", what, err)
		}
	}
	if err := br.Close(); err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	return nil
}

// InsertFailedEvent parks an event in the dead-letter queue, replacing an
// earlier failure of the same event.
func (s *Store) InsertFailedEvent(ctx context.Context, fe poller.FailedEvent) error {
	attempts := fe.Attempts
	if attempts <= 0 {
		attempts = 3
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
		fe.ErrorMessage, attempts, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert failed event: %w", err)
	}
	return nil
}

// ---- sync state and cursors -----------------------------------------------

// GetSyncState returns the contract's indexing cursor, or a zero LastLedger
// when the contract has never been indexed.
func (s *Store) GetSyncState(ctx context.Context, contractID string) (poller.SyncState, error) {
	var last int64
	err := s.pool.QueryRow(ctx, `SELECT last_ledger FROM sync_state WHERE contract_id = $1`, contractID).Scan(&last)
	if errors.Is(err, pgx.ErrNoRows) {
		return poller.SyncState{ContractID: contractID}, nil
	}
	if err != nil {
		return poller.SyncState{}, fmt.Errorf("get sync state: %w", err)
	}
	return poller.SyncState{ContractID: contractID, LastLedger: uint32(last)}, nil
}

// UpsertSyncState records the last fully indexed ledger for a contract and
// clears any previous error.
func (s *Store) UpsertSyncState(ctx context.Context, st poller.SyncState) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sync_state (contract_id, last_ledger, last_run_at, error_message, updated_at)
		VALUES ($1, $2, $3, NULL, $3)
		ON CONFLICT (contract_id) DO UPDATE SET
			last_ledger   = EXCLUDED.last_ledger,
			last_run_at   = EXCLUDED.last_run_at,
			error_message = NULL,
			updated_at    = EXCLUDED.updated_at`,
		st.ContractID, int64(st.LastLedger), now,
	)
	if err != nil {
		return fmt.Errorf("upsert sync state: %w", err)
	}
	return nil
}

// GetIndexerCursor returns the last committed ledger for a network, or 0.
func (s *Store) GetIndexerCursor(ctx context.Context, network string) (uint32, error) {
	var ledger int64
	err := s.pool.QueryRow(ctx, `SELECT ledger FROM indexer_cursors WHERE network = $1`, networkOrDefault(network)).Scan(&ledger)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get indexer cursor: %w", err)
	}
	return uint32(ledger), nil
}

// SetIndexerCursor advances the network cursor; it never moves backwards.
func (s *Store) SetIndexerCursor(ctx context.Context, network string, ledger uint32) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO indexer_cursors (network, ledger, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (network) DO UPDATE SET
			ledger = GREATEST(indexer_cursors.ledger, EXCLUDED.ledger),
			updated_at = NOW()`, networkOrDefault(network), int64(ledger))
	if err != nil {
		return fmt.Errorf("set indexer cursor: %w", err)
	}
	return nil
}

// ---- partitions -----------------------------------------------------------

// CreateNextMonthPartition creates next month's events partition if missing.
func (s *Store) CreateNextMonthPartition(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `SELECT create_next_month_partition()`); err != nil {
		return fmt.Errorf("create next month partition: %w", err)
	}
	return nil
}

// CreateMonthlyPartitionIfNotExists creates the events partition for a month.
func (s *Store) CreateMonthlyPartitionIfNotExists(ctx context.Context, year int, month int) error {
	if _, err := s.pool.Exec(ctx, `SELECT create_monthly_partition($1, $2)`, year, month); err != nil {
		return fmt.Errorf("create monthly partition %04d-%02d: %w", year, month, err)
	}
	return nil
}

// ---- anomaly and health -----------------------------------------------------

// RecentHourlyActivity returns one bucket per hour for the last `hours`
// hours, oldest first, with zero buckets for idle hours.
func (s *Store) RecentHourlyActivity(ctx context.Context, contractID string, hours int) ([]poller.HourlyActivity, error) {
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
		       COALESCE(ev.events, 0)       AS events,
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
			SELECT date_trunc('hour', ledger_closed_at)   AS hour,
			       COUNT(*)                               AS invocations,
			       COALESCE(SUM(cpu_insn), 0)             AS cpu,
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

	var out []poller.HourlyActivity
	for rows.Next() {
		var a poller.HourlyActivity
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

// InsertAlert persists an anomaly alert, de-duplicated on (tx_hash,
// contract_id).
func (s *Store) InsertAlert(ctx context.Context, a poller.Alert) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contract_alerts (contract_id, severity, message, ledger, tx_hash, timestamp)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (tx_hash, contract_id) DO NOTHING`,
		a.ContractID, a.Severity, a.Message, a.Ledger, a.TxHash, a.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert alert: %w", err)
	}
	return nil
}

// ContractHealthInputs gathers the raw signals behind the health score.
func (s *Store) ContractHealthInputs(ctx context.Context, contractID string) (poller.HealthInputs, error) {
	var in poller.HealthInputs

	if err := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM((status = 'Healthy')::int), 0),
			COUNT(*),
			COALESCE((SELECT status FROM monitored_contracts WHERE contract_id = $1), '')
		FROM (
			SELECT status FROM health_checks WHERE contract_id = $1 ORDER BY timestamp DESC LIMIT 100
		) recent`, contractID,
	).Scan(&in.HealthyChecks, &in.TotalChecks, &in.WatchdogStatus); err != nil {
		return in, fmt.Errorf("watchdog health inputs: %w", err)
	}

	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE status <> 'SUCCESS')
		FROM invocations
		WHERE contract_id = $1
		  AND ledger_closed_at >= NOW() - INTERVAL '24 hours'`, contractID,
	).Scan(&in.TotalInvocations, &in.FailedInvocations); err != nil {
		return in, fmt.Errorf("invocation health inputs: %w", err)
	}

	activity, err := s.RecentHourlyActivity(ctx, contractID, 24)
	if err != nil {
		return in, fmt.Errorf("activity health inputs: %w", err)
	}
	in.Activity = activity

	if err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'live'),
			COUNT(*) FILTER (
				WHERE status = 'live'
				  AND live_until_ledger IS NOT NULL
				  AND live_until_ledger <= COALESCE((SELECT MAX(last_ledger) FROM sync_state WHERE contract_id = $1), 0) + $2
			)
		FROM storage_entries
		WHERE contract_id = $1`, contractID, storageHealthyHorizonLedgers,
	).Scan(&in.TotalStorage, &in.ExpiringStorage); err != nil {
		return in, fmt.Errorf("storage health inputs: %w", err)
	}
	return in, nil
}

// UpsertContractHealthScore caches a computed health score.
func (s *Store) UpsertContractHealthScore(ctx context.Context, h poller.ContractHealthScore) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contract_health_scores
			(contract_id, score, component_uptime, component_error_rate,
			 component_performance, component_storage_ttl, computed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (contract_id) DO UPDATE SET
			score                 = EXCLUDED.score,
			component_uptime      = EXCLUDED.component_uptime,
			component_error_rate  = EXCLUDED.component_error_rate,
			component_performance = EXCLUDED.component_performance,
			component_storage_ttl = EXCLUDED.component_storage_ttl,
			computed_at           = EXCLUDED.computed_at`,
		h.ContractID, h.Score, h.ComponentUptime, h.ComponentErrorRate,
		h.ComponentPerformance, h.ComponentStorageTTL, h.ComputedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert contract health score: %w", err)
	}
	return nil
}

// ---- upgrades and Wasm cache ------------------------------------------------

// InsertContractUpgrade records a Wasm hash change, ignoring duplicates of the
// same (contract, tx).
func (s *Store) InsertContractUpgrade(ctx context.Context, u poller.ContractUpgrade) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contract_upgrades (contract_id, from_hash, to_hash, ledger, tx_hash, at)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6)
		ON CONFLICT (contract_id, tx_hash) DO NOTHING`,
		u.ContractID, u.FromHash, u.ToHash, int64(u.Ledger), u.TxHash, u.At,
	)
	if err != nil {
		return fmt.Errorf("insert contract upgrade: %w", err)
	}
	return nil
}

// HasContractWasm reports whether the Wasm for wasmHash is cached.
func (s *Store) HasContractWasm(ctx context.Context, wasmHash string) (bool, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM contract_wasm WHERE wasm_hash = $1)`, wasmHash,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("has contract wasm: %w", err)
	}
	return exists, nil
}

// UpsertContractWasm caches Wasm bytes under their content hash.
func (s *Store) UpsertContractWasm(ctx context.Context, wasmHash string, code []byte) error {
	if wasmHash == "" {
		return errors.New("upsert contract wasm: empty hash")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contract_wasm (wasm_hash, code, size_bytes, fetched_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (wasm_hash) DO NOTHING`,
		wasmHash, code, len(code),
	)
	if err != nil {
		return fmt.Errorf("upsert contract wasm: %w", err)
	}
	return nil
}
