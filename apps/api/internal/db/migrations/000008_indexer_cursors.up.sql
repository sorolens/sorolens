-- ============================================================
-- 000008_indexer_cursors
--
-- Tracks the last successfully committed ledger sequence per network
-- for indexer crash recovery.
-- ============================================================
CREATE TABLE IF NOT EXISTS indexer_cursors (
    network    TEXT PRIMARY KEY,
    ledger     BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
