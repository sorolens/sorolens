-- Reverse of 000002_contract_versions.up.sql
DROP INDEX IF EXISTS idx_contract_versions_unique_hash;
DROP INDEX IF EXISTS idx_contract_versions_wasm_hash;
DROP INDEX IF EXISTS idx_contract_versions_contract_ledger;
DROP TABLE IF EXISTS contract_versions;
