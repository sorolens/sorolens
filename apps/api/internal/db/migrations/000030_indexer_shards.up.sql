-- Sharded indexer topology (issue #272).
--
-- Two tables back horizontal scaling. Whether a shard is owned decides which
-- worker indexes its contracts, and worker liveness decides when a shard is
-- handed back out, so a crashed worker's contracts resume automatically.

-- indexer_workers tracks liveness. The coordinator treats a worker as dead
-- once heartbeat_at is older than INDEXER_WORKER_STALE_AFTER.
CREATE TABLE IF NOT EXISTS indexer_workers (
    worker_id    TEXT PRIMARY KEY,
    started_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- indexer_shards is the shard -> worker ownership table. A NULL worker_id
-- means the shard is unowned and available for the leader to hand out.
--
-- Which contracts a shard holds is NOT stored here: it is derived by hashing
-- the contract id (coordinator.ShardForContract), so adding or removing a
-- contract never requires a write here, and every worker computes the same
-- answer from the same inputs.
CREATE TABLE IF NOT EXISTS indexer_shards (
    shard_id    INTEGER PRIMARY KEY,
    worker_id   TEXT REFERENCES indexer_workers(worker_id) ON DELETE SET NULL,
    assigned_at TIMESTAMPTZ
);

-- The coordinator's hot path is "which shards does this worker own" on every
-- worker heartbeat, and "which shards are unowned" on every rebalance.
CREATE INDEX IF NOT EXISTS indexer_shards_worker_id_idx
    ON indexer_shards (worker_id);

-- Staleness scanning reads heartbeat_at across all workers.
CREATE INDEX IF NOT EXISTS indexer_workers_heartbeat_at_idx
    ON indexer_workers (heartbeat_at);

-- Leader election uses a Postgres advisory lock (pg_try_advisory_lock with
-- the key "Sorolens" = 0x536F726F6C656E73), not a table, so there is nothing
-- to create for it here. Advisory locks are session-scoped: the coordinator
-- must hold a dedicated connection for as long as it leads, or the lock is
-- released the moment the connection returns to the pool.
