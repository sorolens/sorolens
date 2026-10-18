-- Reverse the failed transaction parking table and its indexes.
DROP INDEX IF EXISTS idx failed_transactions_status_next_retry_at;
DROP INDEX IF EXISTS idx failed_transactions_tx_hash;
DROP TABLE IF EXISTS failed_transactions;
