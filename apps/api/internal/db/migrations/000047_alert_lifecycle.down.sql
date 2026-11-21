-- Rollback alert lifecycle columns (issue #789).
DROP INDEX IF EXISTS idx_contract_alerts_state;

ALTER TABLE contract_alerts
    DROP COLUMN IF EXISTS resolved_by,
    DROP COLUMN IF EXISTS resolved_at,
    DROP COLUMN IF EXISTS snoozed_by,
    DROP COLUMN IF EXISTS snoozed_at,
    DROP COLUMN IF EXISTS snoozed_until,
    DROP COLUMN IF EXISTS acknowledged_by,
    DROP COLUMN IF EXISTS acknowledged_at,
    DROP COLUMN IF EXISTS state;
