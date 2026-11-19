-- ============================================================
-- 000038_contract_backfills
--
-- Persists the position of each contract's new-contract backfill
-- (issue #509). The indexer walks the range from newest_ledger down to
-- oldest_ledger in bounded windows and commits next_ledger in the same
-- transaction as each window's rows, so a restart resumes exactly where
-- the previous process stopped.
--
-- next_ledger is the newest ledger not scanned yet. The backfill is
-- finished once next_ledger < oldest_ledger.
-- ============================================================

CREATE TABLE contract_backfills (
    contract_id   TEXT        PRIMARY KEY REFERENCES contracts (id) ON DELETE CASCADE,
    oldest_ledger BIGINT      NOT NULL,
    newest_ledger BIGINT      NOT NULL,
    next_ledger   BIGINT      NOT NULL,
    started_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMPTZ
);
