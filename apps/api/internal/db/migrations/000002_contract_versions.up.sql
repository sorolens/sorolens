-- ============================================================
-- contract_versions
-- Records every Wasm hash transition detected for a tracked
-- contract. One row is appended each time the indexer observes
-- a different wasm_hash compared to the previously known value.
-- ============================================================
CREATE TABLE contract_versions (
    id                   BIGSERIAL   PRIMARY KEY,
    contract_id          TEXT        NOT NULL REFERENCES contracts (id),
    wasm_hash            TEXT        NOT NULL,
    -- The first ledger in which this wasm_hash was observed.
    first_seen_ledger    BIGINT      NOT NULL,
    -- The transaction that introduced this Wasm hash, if known.
    tx_hash              TEXT,
    -- Cross-link to the source verification registry (issue #4).
    -- NULL when the Wasm has not been verified yet.
    verified_source_ref  TEXT,
    -- Wall-clock time when the indexer wrote this row.
    recorded_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Primary access pattern: all versions for a contract, newest first.
CREATE INDEX idx_contract_versions_contract_ledger
    ON contract_versions (contract_id, first_seen_ledger DESC);

-- Lookup a specific hash across all contracts (e.g. badge endpoint).
CREATE INDEX idx_contract_versions_wasm_hash
    ON contract_versions (wasm_hash);

-- Enforce exactly one row per (contract_id, wasm_hash) combination so
-- duplicate detections on overlapping indexer windows are idempotent.
CREATE UNIQUE INDEX idx_contract_versions_unique_hash
    ON contract_versions (contract_id, wasm_hash);
