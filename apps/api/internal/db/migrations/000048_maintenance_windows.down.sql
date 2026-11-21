-- Rollback maintenance windows (issue #790).
ALTER TABLE contract_alerts
    DROP CONSTRAINT IF EXISTS fk_contract_alerts_maintenance_window;

ALTER TABLE contract_alerts
    DROP COLUMN IF EXISTS maintenance_window_id;

DROP TABLE IF EXISTS maintenance_windows CASCADE;
