-- TTL history: records ExtendFootprintTTL and RestoreFootprint operations
-- so the API can expose a contract's TTL change log (issue #804).

CREATE TABLE IF NOT EXISTS ttl_history (
    id               TEXT        NOT NULL DEFAULT gen_random_uuid()::text,
    contract_id      TEXT        NOT NULL,
    network          TEXT        NOT NULL,
    tx_hash          TEXT        NOT NULL,
    ledger           BIGINT      NOT NULL,
    ledger_closed_at TIMESTAMPTZ NOT NULL,
    op_type          TEXT        NOT NULL CHECK (op_type IN ('extend_ttl', 'restore_footprint')),
    key_xdr          TEXT        NOT NULL DEFAULT '',
    old_live_ledger  BIGINT,
    new_live_ledger  BIGINT,
    inserted_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id),
    UNIQUE (contract_id, tx_hash, key_xdr)
);

CREATE INDEX IF NOT EXISTS ttl_history_contract_id_idx
    ON ttl_history (contract_id, id DESC);
CREATE INDEX IF NOT EXISTS ttl_history_ledger_idx
    ON ttl_history (contract_id, ledger DESC);
