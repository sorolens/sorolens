-- ============================================================
-- 000009_events_tx_hash_index (down)
--
-- Deliberate no-op. idx_events_tx_hash belongs to the baseline
-- schema: 000001_init.up.sql creates it and
-- 000003_partition_events.up.sql recreates it for the partitioned
-- events table. This migration only guarantees the index exists,
-- so dropping it on rollback would leave the baseline schema
-- incomplete.
--
-- To remove the index manually, run:
--   DROP INDEX IF EXISTS idx_events_tx_hash;
-- ============================================================

SELECT 1; -- no-op
