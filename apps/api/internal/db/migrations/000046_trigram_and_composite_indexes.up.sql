-- Issue #762: composite indexes, trigram search indexes and hot query performance.
-- Enable pg_trgm for accelerated substring and pattern matching (ILIKE '%term%').
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Trigram GIN indexes for the contract search in
-- apps/api/internal/store/postgres.go (SearchContracts):
--     WHERE c.id ILIKE $1 OR c.label ILIKE $1
-- Both sides of the OR need an index or the planner falls back to a seq scan.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_contracts_id_trgm
    ON contracts USING gin (id gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_contracts_label_trgm
    ON contracts USING gin (label gin_trgm_ops);

-- Trigram GIN indexes for account label search in
-- apps/api/internal/store/postgres.go (SearchLabels), which unions the public
-- and the per-workspace tables and filters both columns with a single OR:
--     WHERE label ILIKE '%' || $2 || '%' OR value ILIKE '%' || $2 || '%'
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_labels_public_label_trgm
    ON labels_public USING gin (label gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_labels_public_value_trgm
    ON labels_public USING gin (value gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_labels_workspace_label_trgm
    ON labels_workspace USING gin (label gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_labels_workspace_value_trgm
    ON labels_workspace USING gin (value gin_trgm_ops);

-- Composite B-tree index for the global events feed in
-- apps/api/internal/store/events_global.go, which filters with
--     contract_id LIKE $2 || '%'
-- The btree indexes events already carries use the column collation, so none
-- of them can serve a LIKE prefix search; text_pattern_ops can.
-- events is partitioned by ledger_closed_at (000005), so PostgreSQL prohibits
-- CONCURRENTLY here: a plain CREATE INDEX on the parent builds a partitioned
-- index, which is what docs/migrations.md prescribes for this table.
CREATE INDEX IF NOT EXISTS idx_events_contract_time_id
    ON events (contract_id text_pattern_ops, ledger_closed_at DESC, id DESC);
