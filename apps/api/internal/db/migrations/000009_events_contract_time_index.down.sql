-- ============================================================
-- 000009_events_contract_time_index (down)
--
-- Drops the composite index added in the matching .up.sql. On a
-- partitioned events table this drops the partitioned parent index and
-- all of its partition indexes in one statement.
-- ============================================================

DROP INDEX IF EXISTS idx_events_contract_id_ledger_closed_at;
