-- 000046_contract_tokens
-- Stellar Asset Contract (SAC) token metadata enrichment (issue #802).

CREATE TABLE contract_tokens (
    contract_id TEXT PRIMARY KEY REFERENCES contracts(id) ON DELETE CASCADE,
    asset_code  TEXT NOT NULL DEFAULT '',
    issuer      TEXT NOT NULL DEFAULT '',
    decimals    INTEGER NOT NULL DEFAULT 7,
    symbol      TEXT NOT NULL DEFAULT '',
    name        TEXT NOT NULL DEFAULT '',
    source      TEXT NOT NULL DEFAULT 'sac_discovery',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_contract_tokens_asset_code ON contract_tokens (asset_code);
