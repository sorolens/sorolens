-- Issue #762: composite indexes, trigram search indexes and hot query performance.
-- Enable pg_trgm for accelerated substring and pattern matching (ILIKE '%term%').
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Trigram GIN indexes for fuzzy and wildcard search on contract labels and IDs.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_contracts_id_trgm
    ON contracts USING gin (id gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_contracts_label_trgm
    ON contracts USING gin (label gin_trgm_ops);

-- Trigram GIN index for account label and value search.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_account_labels_label_trgm
    ON account_labels USING gin (label gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_monitored_contracts_name_trgm
    ON monitored_contracts USING gin (name gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_monitored_contracts_id_trgm
    ON monitored_contracts USING gin (contract_id gin_trgm_ops);

-- Composite B-tree index for global events feed filtering by contract prefix and time range.
-- events is partitioned by closed_at (000005), so PostgreSQL prohibits CONCURRENTLY here.
CREATE INDEX IF NOT EXISTS idx_events_contract_time_id
    ON events (contract_id text_pattern_ops, ledger_closed_at DESC, id DESC);
