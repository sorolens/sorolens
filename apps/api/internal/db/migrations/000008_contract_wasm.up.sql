-- ============================================================
-- 000008_contract_wasm
--
-- Content-addressed cache of Soroban contract Wasm binaries
-- (issue #162). Keyed by wasm hash so identical code shared
-- across contracts is stored once. Served by
-- GET /api/v1/contracts/:id/wasm with Cache-Control: immutable.
-- ============================================================

CREATE TABLE contract_wasm (
    wasm_hash  TEXT        PRIMARY KEY,
    code       BYTEA       NOT NULL,
    size_bytes INTEGER     NOT NULL,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Lookup by fetch time for cache maintenance / eviction tooling.
CREATE INDEX idx_contract_wasm_fetched_at ON contract_wasm (fetched_at DESC);
