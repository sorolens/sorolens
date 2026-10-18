-- Reverts the sharded indexer topology (issue #272).
DROP INDEX IF EXISTS indexer_workers_heartbeat_at_idx;
DROP INDEX IF EXISTS indexer_shards_worker_id_idx;
DROP TABLE IF EXISTS indexer_shards;
DROP TABLE IF EXISTS indexer_workers;
