-- Issue #790: maintenance windows that suppress notifications.
CREATE TABLE IF NOT EXISTS maintenance_windows (
    id                     BIGSERIAL PRIMARY KEY,
    name                   TEXT NOT NULL,
    scope                  TEXT NOT NULL CHECK (scope IN ('global', 'contract', 'group')),
    target_id              TEXT,
    start_time             TIMESTAMPTZ NOT NULL,
    end_time               TIMESTAMPTZ NOT NULL,
    timezone               TEXT NOT NULL DEFAULT 'UTC',
    recurrence             TEXT NOT NULL DEFAULT 'none',
    suppress_notifications BOOLEAN NOT NULL DEFAULT true,
    created_by             TEXT NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (end_time > start_time)
);

CREATE INDEX IF NOT EXISTS idx_maintenance_windows_scope
    ON maintenance_windows (scope, target_id);

CREATE INDEX IF NOT EXISTS idx_maintenance_windows_time
    ON maintenance_windows (start_time, end_time);

ALTER TABLE contract_alerts
    ADD COLUMN IF NOT EXISTS maintenance_window_id BIGINT;

ALTER TABLE contract_alerts
    ADD CONSTRAINT fk_contract_alerts_maintenance_window
    FOREIGN KEY (maintenance_window_id)
    REFERENCES maintenance_windows(id)
    ON DELETE SET NULL
    NOT VALID;
