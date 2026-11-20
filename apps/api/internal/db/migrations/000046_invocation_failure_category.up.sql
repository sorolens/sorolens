-- Failure classification for invocations (issue #806).
--
-- failure_category is a small, stable enum of causes (budget, contract, auth,
-- storage, wasm_trap, value, crypto, events, object, context, transaction,
-- other) decoded by the indexer from the transaction result and host diagnostic
-- events. failure_code preserves the raw host or transaction error code
-- ("sce_storage:3", "tx_bad_seq", ...) so an unknown cause is never dropped.
--
-- Both columns are nullable: rows indexed before this migration, and
-- successful invocations, have no failure to classify.
ALTER TABLE invocations ADD COLUMN IF NOT EXISTS failure_category TEXT;
ALTER TABLE invocations ADD COLUMN IF NOT EXISTS failure_code TEXT;
