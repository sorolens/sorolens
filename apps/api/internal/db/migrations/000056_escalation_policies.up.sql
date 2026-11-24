-- Escalation policies: ordered notification steps that fire progressively
-- when an alert remains open (issue #791).

CREATE TABLE IF NOT EXISTS escalation_policies (
    id         TEXT        NOT NULL DEFAULT gen_random_uuid()::text,
    name       TEXT        NOT NULL,
    steps      JSONB       NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS escalation_policies_name_idx
    ON escalation_policies (name);
