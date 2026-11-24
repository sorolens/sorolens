-- Issue #801: per-holder token balances aggregated from token_transfers.
-- The balance is updated incrementally by the indexer as new transfer/mint/burn
-- events arrive; it is never negative (clamped to 0 on underflow to stay
-- consistent with the on-chain constraint).

CREATE TABLE IF NOT EXISTS token_holder_balances (
    contract_id TEXT           NOT NULL,
    network     TEXT           NOT NULL,
    holder      TEXT           NOT NULL,
    balance     NUMERIC(39, 0) NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    PRIMARY KEY (contract_id, holder)
);

-- Descending balance index for the top-holders ranking query.
CREATE INDEX IF NOT EXISTS idx_token_holder_balances_rank
    ON token_holder_balances (contract_id, balance DESC);
