-- ============================================================
-- 000008_alert_rules
--
-- User-defined alert rules written in the Sorolens alert-rule DSL
-- (see packages/rules). Each row is an expression the indexer's
-- notifier loop evaluates every poll cycle; a firing rule becomes a
-- contract_alerts row (deduplicated on a synthetic tx key) and, at
-- Critical severity, is dispatched to matching webhook
-- subscriptions.
--
-- contract_id is denormalized from the expression's "on" clause so
-- the indexer can filter rules per contract cheaply. "" means the
-- rule applies to every tracked contract.
-- ============================================================
CREATE TABLE alert_rules (
    id          TEXT        PRIMARY KEY,
    name        TEXT        NOT NULL,
    expression  TEXT        NOT NULL,
    contract_id TEXT        NOT NULL DEFAULT '',
    network     TEXT        NOT NULL DEFAULT 'testnet',
    severity    TEXT        NOT NULL DEFAULT 'Warning'
                  CHECK (severity IN ('Info', 'Warning', 'Critical')),
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The indexer reads enabled rules per contract on every poll.
CREATE INDEX idx_alert_rules_enabled_contract
    ON alert_rules (enabled, contract_id);