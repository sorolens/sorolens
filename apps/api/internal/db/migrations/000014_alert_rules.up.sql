-- ============================================================
-- 000014_alert_rules
--
-- User-defined alert rules expressed in the Sorolens rule
-- language (apps/api/rulelang). A rule is stored as its source
-- text alongside the parsed summary fields the indexer needs to
-- evaluate it without re-parsing on every pass.
--
-- source        the rule text, e.g. "fee_per_invocation > 0.5 XLM for 5m"
-- severity      the alert severity a firing rule raises
-- contract_id   optional scope; NULL means every contract the
--               rule's network covers
-- network       optional scope; '' means all networks
-- window_secs   parsed `for` window in seconds (0 = instantaneous)
-- enabled       paused rules are skipped by the evaluator
-- ============================================================

CREATE TABLE alert_rules (
    id          BIGSERIAL   PRIMARY KEY,
    name        TEXT        NOT NULL,
    source      TEXT        NOT NULL,
    severity    TEXT        NOT NULL DEFAULT 'Warning'
                            CHECK (severity IN ('Info', 'Warning', 'Critical')),
    contract_id TEXT,
    network     TEXT        NOT NULL DEFAULT '',
    window_secs BIGINT      NOT NULL DEFAULT 0,
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- A rule scoped to a tracked contract must reference a real one.
    CONSTRAINT alert_rules_contract_fk
        FOREIGN KEY (contract_id) REFERENCES contracts (id) ON DELETE CASCADE
);

-- The evaluator selects enabled rules on every indexer pass.
CREATE INDEX idx_alert_rules_enabled ON alert_rules (enabled) WHERE enabled;

-- Dashboard lists rules newest first.
CREATE INDEX idx_alert_rules_created_at ON alert_rules (created_at DESC);

-- One rule source should not be registered twice for the same scope.
CREATE UNIQUE INDEX idx_alert_rules_unique_source
    ON alert_rules (source, COALESCE(contract_id, ''), network);
