package pgstore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sorolens/sorolens/services/indexer/internal/coordinator"
)

// CoordinatorStore is the shared coordinator.Store used by the coordinator and
// worker roles. Shard ownership lives in indexer_shards and worker liveness in
// indexer_workers (migration 000011), so every replica sees the same table and
// exactly one elected leader can change it.
//
// The leader lock is a Postgres session-scoped advisory lock. A lock belongs to
// a connection, not a pooled virtual session, so this store pins one physical
// connection for as long as it leads and destroys that connection if the lock
// cannot be released cleanly.
type CoordinatorStore struct {
	pool *pgxpool.Pool

	mu       sync.Mutex
	lockConn *pgxpool.Conn
	lockKey  int64
}

var _ coordinator.Store = (*CoordinatorStore)(nil)

// NewCoordinatorStore wraps an existing pool. The caller keeps ownership of
// the pool; Close only releases the pinned leader connection.
func NewCoordinatorStore(pool *pgxpool.Pool) *CoordinatorStore {
	return &CoordinatorStore{pool: pool}
}

// Close releases the leader connection, if any. The session is destroyed
// rather than returned to the pool: returning it would leave the advisory
// lock held by an idle pooled session and block every future leader.
func (s *CoordinatorStore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lockConn == nil {
		return
	}
	conn := s.lockConn
	s.lockConn, s.lockKey = nil, 0
	_ = conn.Hijack().Close(context.Background())
}

// ListContracts returns every tracked contract id, ordered for deterministic
// shard assignment.
func (s *CoordinatorStore) ListContracts(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM contracts ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("coordinator: list contracts: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("coordinator: list contracts scan: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("coordinator: list contracts: %w", err)
	}
	return out, nil
}

// TryAdvisoryLock pins a connection and tries pg_try_advisory_lock on it. It
// never blocks. A successful call keeps the connection pinned until
// ReleaseAdvisoryLock or Close, because releasing it to the pool would drop the
// session lock.
func (s *CoordinatorStore) TryAdvisoryLock(ctx context.Context, key int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lockConn != nil {
		if s.lockKey != key {
			return false, fmt.Errorf("coordinator: already holding advisory lock %d, cannot try %d", s.lockKey, key)
		}
		// A dropped connection releases the lock server-side. Detect that on
		// the next attempt and compete for it again rather than reporting a
		// leadership we no longer have.
		if err := s.lockConn.Ping(ctx); err == nil {
			return true, nil
		}
		s.lockConn.Release()
		s.lockConn, s.lockKey = nil, 0
	}

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return false, fmt.Errorf("coordinator: acquire lock connection: %w", err)
	}
	var acquired bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&acquired); err != nil {
		conn.Release()
		return false, fmt.Errorf("coordinator: try advisory lock: %w", err)
	}
	if !acquired {
		conn.Release()
		return false, nil
	}
	s.lockConn, s.lockKey = conn, key
	return true, nil
}

// ReleaseAdvisoryLock unlocks and returns the pinned connection to the pool.
func (s *CoordinatorStore) ReleaseAdvisoryLock(ctx context.Context, key int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lockConn == nil {
		return nil
	}
	if s.lockKey != key {
		return fmt.Errorf("coordinator: releasing advisory lock %d while holding %d", key, s.lockKey)
	}

	conn := s.lockConn
	s.lockConn, s.lockKey = nil, 0

	var released bool
	if err := conn.QueryRow(ctx, `SELECT pg_advisory_unlock($1)`, key).Scan(&released); err != nil {
		// The unlock did not run; destroy the session so Postgres releases the
		// lock instead of leaking it in an idle pooled connection.
		_ = conn.Hijack().Close(context.Background())
		return fmt.Errorf("coordinator: release advisory lock: %w", err)
	}
	conn.Release()
	if !released {
		return fmt.Errorf("coordinator: advisory lock %d was not held", key)
	}
	return nil
}

// EnsureShard creates an unowned shard row if it does not exist.
func (s *CoordinatorStore) EnsureShard(ctx context.Context, shardID int) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO indexer_shards (shard_id) VALUES ($1) ON CONFLICT (shard_id) DO NOTHING`, shardID)
	if err != nil {
		return fmt.Errorf("coordinator: ensure shard %d: %w", shardID, err)
	}
	return nil
}

// ListShards returns every shard ordered by id.
func (s *CoordinatorStore) ListShards(ctx context.Context) ([]coordinator.Shard, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT shard_id, COALESCE(worker_id, ''), assigned_at
		FROM indexer_shards
		ORDER BY shard_id`)
	if err != nil {
		return nil, fmt.Errorf("coordinator: list shards: %w", err)
	}
	defer rows.Close()

	var out []coordinator.Shard
	for rows.Next() {
		var sh coordinator.Shard
		if err := rows.Scan(&sh.ShardID, &sh.WorkerID, &sh.AssignedAt); err != nil {
			return nil, fmt.Errorf("coordinator: list shards scan: %w", err)
		}
		out = append(out, sh)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("coordinator: list shards: %w", err)
	}
	return out, nil
}

// AssignShard records ownership. The worker row is created if a heartbeat has
// not reached the database yet, because indexer_shards.worker_id is a foreign
// key; an existing heartbeat is never overwritten.
func (s *CoordinatorStore) AssignShard(ctx context.Context, shardID int, workerID string, at time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("coordinator: assign shard %d: begin: %w", shardID, err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `
		INSERT INTO indexer_workers (worker_id, started_at, heartbeat_at)
		VALUES ($1, $2, $2)
		ON CONFLICT (worker_id) DO NOTHING`, workerID, at); err != nil {
		return fmt.Errorf("coordinator: ensure worker %s: %w", workerID, err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE indexer_shards
		SET worker_id = $2, assigned_at = $3
		WHERE shard_id = $1`, shardID, workerID, at)
	if err != nil {
		return fmt.Errorf("coordinator: assign shard %d: %w", shardID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("coordinator: shard %d does not exist", shardID)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("coordinator: assign shard %d: commit: %w", shardID, err)
	}
	return nil
}

// UnassignShard clears ownership so the shard can be handed out again.
func (s *CoordinatorStore) UnassignShard(ctx context.Context, shardID int) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE indexer_shards
		SET worker_id = NULL, assigned_at = NULL
		WHERE shard_id = $1`, shardID)
	if err != nil {
		return fmt.Errorf("coordinator: unassign shard %d: %w", shardID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("coordinator: shard %d does not exist", shardID)
	}
	return nil
}

// ShardsForWorker lists the shard ids owned by one worker, ordered by id.
func (s *CoordinatorStore) ShardsForWorker(ctx context.Context, workerID string) ([]int, error) {
	if workerID == "" {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT shard_id FROM indexer_shards
		WHERE worker_id = $1
		ORDER BY shard_id`, workerID)
	if err != nil {
		return nil, fmt.Errorf("coordinator: shards for worker: %w", err)
	}
	defer rows.Close()

	var out []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("coordinator: shards for worker scan: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("coordinator: shards for worker: %w", err)
	}
	return out, nil
}

// Heartbeat creates the worker on first contact and advances its liveness.
// started_at keeps the first contact time, and heartbeat_at never moves
// backwards when a delayed heartbeat arrives after a newer one.
func (s *CoordinatorStore) Heartbeat(ctx context.Context, workerID string, at time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO indexer_workers (worker_id, started_at, heartbeat_at)
		VALUES ($1, $2, $2)
		ON CONFLICT (worker_id) DO UPDATE SET
			heartbeat_at = GREATEST(indexer_workers.heartbeat_at, EXCLUDED.heartbeat_at)`,
		workerID, at)
	if err != nil {
		return fmt.Errorf("coordinator: heartbeat %s: %w", workerID, err)
	}
	return nil
}

// ListWorkers returns every known worker ordered by id.
func (s *CoordinatorStore) ListWorkers(ctx context.Context) ([]coordinator.Worker, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT worker_id, started_at, heartbeat_at
		FROM indexer_workers
		ORDER BY worker_id`)
	if err != nil {
		return nil, fmt.Errorf("coordinator: list workers: %w", err)
	}
	defer rows.Close()

	var out []coordinator.Worker
	for rows.Next() {
		var w coordinator.Worker
		if err := rows.Scan(&w.WorkerID, &w.StartedAt, &w.HeartbeatAt); err != nil {
			return nil, fmt.Errorf("coordinator: list workers scan: %w", err)
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("coordinator: list workers: %w", err)
	}
	return out, nil
}

// DeleteWorker removes a dead worker and returns its shards to the pool.
// indexer_shards.worker_id is ON DELETE SET NULL, but assigned_at is cleared
// explicitly so an unowned shard never carries a stale assignment time.
func (s *CoordinatorStore) DeleteWorker(ctx context.Context, workerID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("coordinator: delete worker %s: begin: %w", workerID, err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `
		UPDATE indexer_shards
		SET worker_id = NULL, assigned_at = NULL
		WHERE worker_id = $1`, workerID); err != nil {
		return fmt.Errorf("coordinator: unassign shards of %s: %w", workerID, err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM indexer_workers WHERE worker_id = $1`, workerID); err != nil {
		return fmt.Errorf("coordinator: delete worker %s: %w", workerID, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("coordinator: delete worker %s: commit: %w", workerID, err)
	}
	return nil
}
