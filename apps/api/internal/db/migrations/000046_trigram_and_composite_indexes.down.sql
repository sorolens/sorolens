-- Rollback trigram and composite indexes (issue #762).
DROP INDEX IF EXISTS idx_events_contract_time_id;
DROP INDEX IF EXISTS idx_monitored_contracts_id_trgm;
DROP INDEX IF EXISTS idx_monitored_contracts_name_trgm;
DROP INDEX IF EXISTS idx_account_labels_label_trgm;
DROP INDEX IF EXISTS idx_contracts_label_trgm;
DROP INDEX IF EXISTS idx_contracts_id_trgm;
