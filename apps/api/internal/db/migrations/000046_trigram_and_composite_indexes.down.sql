-- Rollback trigram and composite indexes (issue #762).
DROP INDEX IF EXISTS idx_events_contract_time_id;
DROP INDEX IF EXISTS idx_labels_workspace_value_trgm;
DROP INDEX IF EXISTS idx_labels_workspace_label_trgm;
DROP INDEX IF EXISTS idx_labels_public_value_trgm;
DROP INDEX IF EXISTS idx_labels_public_label_trgm;
DROP INDEX IF EXISTS idx_contracts_label_trgm;
DROP INDEX IF EXISTS idx_contracts_id_trgm;
