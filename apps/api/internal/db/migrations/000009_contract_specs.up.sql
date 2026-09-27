-- ============================================================
-- 000009_contract_specs
--
-- Caches the parsed SEP-48 interface spec for each tracked contract. The
-- indexer extracts the "contractspecv0" custom section from the contract's
-- Wasm the first time it indexes it and stores the projected JSON tree here,
-- so the API can serve GET /api/v1/contracts/:id/spec without re-fetching or
-- re-parsing the Wasm on every request.
--
-- One row per contract: it is replaced wholesale when the contract's Wasm is
-- upgraded and re-parsed, so no history table is needed.
-- ============================================================
CREATE TABLE IF NOT EXISTS contract_specs (
    contract_id TEXT        PRIMARY KEY REFERENCES contracts (id) ON DELETE CASCADE,
    spec        JSONB       NOT NULL,   -- {"functions":[{name, doc, inputs, outputs}]}
    wasm_hash   TEXT,                   -- hex Wasm hash the spec was parsed from
    parsed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
