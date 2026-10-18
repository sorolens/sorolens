-- 000038_failed_transactions.up.sql
-- Dead-letter store for transactions that could not be fetched from the
-- Stellar RPC after exhausting retries. Rows are requeued by the indexer
-- and removed once successfully fetched and ingested.

CREATE TABLE IF NOT EXISTS failed_transactions (
    id                  BIGSERIALL PRIMARY KEY,
    tx_hash             TEXT NOT NULL UNIQUE,
    ledger              BIGINT NOT NULL,
    attempts            INTEGER NOT NULL DEFAULT 0,
    last_error          TEXT,
    status              TEXT NOT NULL DEFAULT 'pending',
    next_retry_at       TIMESTAMP TIZ NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMP TIZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP TIZNOT NULL DEFAULT NOW(),
    CONSTRAINT failed_tx_status_check CHECK (status IN ('pending', 'requeued', 'resolved', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_failed_transactions_status_next_retry
    ON failed_transactions (status, next_retry_at);

CREATE INDEX IF NOT EXISTS idx_failed_transactions_ledger
    ON failed_transactions (ledger);
