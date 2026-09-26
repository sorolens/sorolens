-- ============================================================
-- 000009_contract_verification
--
-- Persists the outcome of contract source verification (issue
-- #263): the submitted source reference, the toolchain versions
-- detected in the build sandbox, the hash produced by the
-- deterministic build, and the on-chain hash it was compared
-- against.
--
-- One row per contract; the latest verification wins. The API
-- serves the cached verdict so the dashboard can render a
-- `Verified` badge without re-running a build.
-- ============================================================

CREATE TABLE contract_verifications (
    contract_id     TEXT        PRIMARY KEY REFERENCES contracts(id) ON DELETE CASCADE,
    status          TEXT        NOT NULL CHECK (status IN ('pending', 'verified', 'failed')),
    on_chain_hash   TEXT        NOT NULL DEFAULT '',
    compiled_hash   TEXT        NOT NULL DEFAULT '',
    matched         BOOLEAN     NOT NULL DEFAULT FALSE,
    source_kind     TEXT        NOT NULL DEFAULT '' CHECK (source_kind IN ('', 'archive', 'git')),
    source_ref      TEXT        NOT NULL DEFAULT '',
    source_digest   TEXT        NOT NULL DEFAULT '',
    stellar_version TEXT        NOT NULL DEFAULT '',
    rustc_version   TEXT        NOT NULL DEFAULT '',
    cargo_version   TEXT        NOT NULL DEFAULT '',
    diagnostics     JSONB       NOT NULL DEFAULT '[]'::jsonb,
    build_log       TEXT        NOT NULL DEFAULT '',
    submitted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    verified_at     TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_contract_verifications_status
    ON contract_verifications (status, updated_at DESC);
