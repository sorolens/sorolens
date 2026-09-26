-- ============================================================
-- 000011_alert_groups
--
-- Alert deduplication and grouping engine (issue #269).
--
-- alert_groups collapses repeated ContractAlerts that share the
-- same (contract_id, severity, rule) key within a configurable
-- dedupe window. The API's GET /api/v1/alerts returns this
-- grouped view by default; ?flat=true falls back to the raw
-- contract_alerts feed.
--
-- dedupe_window_secs  operator-supplied window in seconds.
-- count               number of raw alerts collapsed into this group.
-- first_seen / last_seen  time range of the collapsed alerts.
-- last_message        the most recent alert message in the group.
-- backfill_eligible   true means this group may be re-computed when
--                     the operator triggers a historical backfill.
-- ============================================================

CREATE TABLE alert_groups (
    id               BIGSERIAL PRIMARY KEY,
    group_key        TEXT        NOT NULL,  -- contract_id|severity|rule
    contract_id      TEXT        NOT NULL REFERENCES monitored_contracts(contract_id) ON DELETE CASCADE,
    severity         TEXT        NOT NULL CHECK (severity IN ('Info', 'Warning', 'Critical')),
    rule             TEXT        NOT NULL DEFAULT '',
    count            BIGINT      NOT NULL DEFAULT 1,
    dedupe_window_secs BIGINT    NOT NULL DEFAULT 300,
    first_seen       TIMESTAMPTZ NOT NULL,
    last_seen        TIMESTAMPTZ NOT NULL,
    last_message     TEXT        NOT NULL DEFAULT '',
    backfill_eligible BOOLEAN    NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fast lookup by contract for the grouped API surface.
CREATE INDEX idx_alert_groups_contract_last_seen
    ON alert_groups (contract_id, last_seen DESC);

-- Unique constraint so upsert logic can use ON CONFLICT.
CREATE UNIQUE INDEX idx_alert_groups_group_key
    ON alert_groups (group_key);
