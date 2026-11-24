-- Issue #801: SEP-41 token transfer tracking.
-- Stores individual transfer, mint, burn and approve events extracted from
-- SEP-41 contract events so the API can serve a per-contract transfer history
-- and aggregated holder balances.

CREATE TABLE IF NOT EXISTS token_transfers (
    id               BIGSERIAL        PRIMARY KEY,
    contract_id      TEXT             NOT NULL,
    network          TEXT             NOT NULL,
    tx_hash          TEXT             NOT NULL,
    ledger           INTEGER          NOT NULL,
    ledger_closed_at TIMESTAMPTZ      NOT NULL,
    event_id         TEXT             NOT NULL,
    transfer_type    TEXT             NOT NULL
        CHECK (transfer_type IN ('transfer', 'mint', 'burn', 'clawback', 'approve')),
    from_address     TEXT,
    to_address       TEXT,
    amount           NUMERIC(39, 0)   NOT NULL DEFAULT 0,
    inserted_at      TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_token_transfers_event UNIQUE (event_id)
);

CREATE INDEX IF NOT EXISTS idx_token_transfers_contract
    ON token_transfers (contract_id, ledger DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_token_transfers_from
    ON token_transfers (from_address)
    WHERE from_address IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_token_transfers_to
    ON token_transfers (to_address)
    WHERE to_address IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_token_transfers_tx
    ON token_transfers (tx_hash);
