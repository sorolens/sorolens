-- Issue #789: alert acknowledge, snooze and resolve lifecycle.
ALTER TABLE contract_alerts
    ADD COLUMN IF NOT EXISTS state TEXT NOT NULL DEFAULT 'open',
    ADD COLUMN IF NOT EXISTS acknowledged_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS acknowledged_by TEXT,
    ADD COLUMN IF NOT EXISTS snoozed_until TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS snoozed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS snoozed_by TEXT,
    ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS resolved_by TEXT;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_contract_alerts_state
    ON contract_alerts (state, timestamp DESC);
