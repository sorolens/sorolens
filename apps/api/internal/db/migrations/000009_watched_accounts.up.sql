-- ============================================================
-- 000009_watched_accounts
--
-- Contract discovery (issue #123). A watched account is a Stellar
-- G-address whose contract deployments are tracked automatically:
-- every ledger the indexer scans for create_contract operations
-- sourced from a watched account and upserts the deployed contract
-- into `contracts` with label 'discovered_by:<account_id>'.
--
-- discovered_count is bumped once per newly tracked contract so the
-- UI can show how many contracts each account has produced.
-- ============================================================

CREATE TABLE watched_accounts (
    account_id       TEXT        PRIMARY KEY,
    added_by         TEXT        NOT NULL DEFAULT '',
    discovered_count BIGINT      NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
