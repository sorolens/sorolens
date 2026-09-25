-- ============================================================
-- 000009_events_contract_time_index
--
-- Composite index backing the common "events for contract X inside
-- time range Y" dashboard/stat query, e.g. the windowed contract stat
-- and the daily activity aggregate in apps/api/internal/store/query.go:
--
--   SELECT COUNT(*) FROM events
--    WHERE contract_id = $1
--      AND ledger_closed_at >= NOW() - $2::interval;
--
-- The existing idx_events_contract_ledger (contract_id, ledger DESC) can
-- serve the contract_id prefix, but it still reads every event for that
-- contract and filters on ledger_closed_at afterwards. Leading on both
-- (contract_id, ledger_closed_at) lets the planner satisfy the predicate
-- with a single bounded index range scan per contract.
--
-- The events table is partitioned by RANGE (ledger_closed_at) once
-- 000003_partition_events applies; creating the index on the parent
-- creates a partitioned index that propagates to every partition. The
-- same statement is valid while the table is unpartitioned, so it is
-- safe regardless of which of the two states a given database is in.
--
-- Idempotent: safe to re-run.
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_events_contract_id_ledger_closed_at
    ON events (contract_id, ledger_closed_at);
