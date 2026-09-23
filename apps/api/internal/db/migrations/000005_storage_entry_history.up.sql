-- ============================================================
-- 000005_storage_entry_history
--
-- Append-only history of storage entries so the snapshot/replay
-- endpoint (#124) can answer "what did this contract's storage look
-- like at ledger N?".
--
-- storage_entries keeps exactly one row per (contract_id, key_xdr) and
-- is upserted on every indexer run, so it only ever reflects the
-- latest state. Every write to storage_entries also appends a row here
-- keyed by (contract_id, key_xdr, last_modified_ledger), which makes
-- the version that was live at any historical ledger recoverable.
-- ============================================================

CREATE TABLE storage_entry_history (
    id                   BIGSERIAL   PRIMARY KEY,
    contract_id          TEXT        NOT NULL,
    key_xdr              TEXT        NOT NULL,
    key_decoded          JSONB,
    value_xdr            TEXT,
    value_decoded        JSONB,
    durability           TEXT        NOT NULL,
    live_until_ledger    BIGINT,
    last_modified_ledger BIGINT,
    status               TEXT        NOT NULL DEFAULT 'live',
    recorded_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (contract_id, key_xdr, last_modified_ledger)
);

-- Snapshot query: latest version of a key at or before ledger N.
CREATE INDEX idx_storage_history_lookup
    ON storage_entry_history (contract_id, key_xdr, last_modified_ledger DESC);

-- Seed history from the current snapshot so time-travel works for data
-- indexed before this migration was applied.
INSERT INTO storage_entry_history
    (contract_id, key_xdr, key_decoded, value_xdr, value_decoded,
     durability, live_until_ledger, last_modified_ledger, status)
SELECT contract_id, key_xdr, key_decoded, value_xdr, value_decoded,
       durability, live_until_ledger, last_modified_ledger, status
  FROM storage_entries
ON CONFLICT (contract_id, key_xdr, last_modified_ledger) DO NOTHING;
