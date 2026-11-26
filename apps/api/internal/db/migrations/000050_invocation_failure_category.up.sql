-- Issue #806: classify failed invocations by host error category.
--
-- failure_category is one of the values defined in apps/api/failurecategory
-- (budget_exceeded, contract_error, auth_failure, storage, wasm_trap, other).
-- failure_code is the raw error behind it, e.g. "Error(Contract, #4)",
-- "Error(Budget, ExceededLimit)" or "INVOKE_HOST_FUNCTION_TRAPPED"; see
-- docs/failure-categories.md.
--
-- The indexer sets both for FAILED invocations only. They stay NULL for every
-- other status and for rows indexed before this migration. Both columns are
-- added nullable without a default, a metadata-only change that takes no
-- table rewrite.
ALTER TABLE invocations ADD COLUMN IF NOT EXISTS failure_category TEXT;
ALTER TABLE invocations ADD COLUMN IF NOT EXISTS failure_code TEXT;
