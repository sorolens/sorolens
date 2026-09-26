-- Re-adds the watchdog FK. Safe to drop/re-add: monitored_contracts rows that
-- are deleted no longer cascade to contract_alerts while the constraint is
-- absent; when it returns, only rows referencing a monitored contract are
-- admitted again (new inserts). Historical rule alerts on now-unmonitored
-- contracts are expected to be cleaned up by retention tooling.
ALTER TABLE contract_alerts
    ADD CONSTRAINT contract_alerts_contract_id_fkey
    FOREIGN KEY (contract_id) REFERENCES monitored_contracts (contract_id) ON DELETE CASCADE;