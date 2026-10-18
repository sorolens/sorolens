-- Reverse composite indexes added for the global events feed.
-- Kept in sync with 000017_events_global_composite_indexes.up.sql

DROP INDEX IF EXISTS idx_events_global_contract_ledger;
DROP INDEX IF EXISTS idx_events_global_ledger_desc;
DROP INDEX IF EXISTS idx_events_global_contract_ledger_desc;
