-- ============================================================
-- 000038_contract_event_schemas
--
-- Inferred event schemas per contract and event name (issue #803).
-- Tracks topic layout, types, value shape, variants and ledgers.
-- ============================================================
CREATE TABLE IF NOT EXISTS contract_event_schemas (
    contract_id        TEXT        NOT NULL REFERENCES contracts (id) ON DELETE CASCADE,
    event_name         TEXT        NOT NULL,
    topic_arity        INT         NOT NULL DEFAULT 0,
    topic_types        JSONB       NOT NULL DEFAULT '[]'::jsonb,
    value_shape        TEXT        NOT NULL DEFAULT 'void',
    observed_shapes    JSONB       NOT NULL DEFAULT '[]'::jsonb,
    first_seen_ledger  BIGINT      NOT NULL DEFAULT 0,
    last_seen_ledger   BIGINT      NOT NULL DEFAULT 0,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (contract_id, event_name)
);

CREATE INDEX IF NOT EXISTS idx_contract_event_schemas_contract_id ON contract_event_schemas (contract_id);
