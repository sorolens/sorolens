-- ============================================================
-- 000009_events_tx_hash_index
--
-- Guarantees idx_events_tx_hash exists on the events table.
--
-- The index is part of the baseline schema: 000001_init.up.sql
-- creates it and 000003_partition_events.up.sql recreates it
-- after the events table is partitioned. However, environments
-- restored from older backups, or that applied only part of the
-- migration chain, can be missing it, forcing a sequential scan
-- on every transaction-hash lookup of the events table.
--
-- CREATE INDEX IF NOT EXISTS makes this migration a no-op
-- whenever the index already exists, so it is safe to run
-- against any deployment state.
--
-- See issue #170.
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_events_tx_hash ON events(tx_hash);
