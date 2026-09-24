-- ============================================================
-- 000006_contract_upgrades
--
-- Append-only history of contract upgrades. Every time an indexer
-- poll observes that a tracked contract's Wasm hash changed since the
-- last record, a row is appended here with the old + new hash, the
-- ledger at which the change was observed, and (when known) the
-- transaction that performed the upgrade.
--
-- Deduplication is keyed on (tx_hash) per the issue guidance so a
-- re-run of the same poll cycle cannot insert the same upgrade twice.
-- tx_hash may be temporarily NULL while the indexer only has ledger
-- evidence; a later pass can backfill the tx.
-- ============================================================

CREATE TABLE contract_upgrades (
    id          BIGSERIAL   PRIMARY KEY,
    contract_id TEXT        NOT NULL,
    from_hash   TEXT        NOT NULL DEFAULT '',
    to_hash     TEXT        NOT NULL,
    ledger      BIGINT      NOT NULL,
    tx_hash     TEXT,
    at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Idempotency: one upgrade record per (contract, tx). NULL tx_hash
    -- values are treated as distinct, so the same upgrade observed
    -- without a tx is still only recorded once per poll because the
    -- indexer compares against the last known hash before inserting.
    UNIQUE (contract_id, tx_hash)
);

-- Most recent upgrades per contract, newest first.
CREATE INDEX idx_contract_upgrades_lookup
    ON contract_upgrades (contract_id, ledger DESC);