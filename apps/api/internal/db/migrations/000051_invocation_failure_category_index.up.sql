-- Issue #806: index for the failure_category filter on GET /api/v1/invocations
-- (apps/api/internal/store/query.go, ListAllInvocations):
--     WHERE failure_category = $n ... ORDER BY ledger DESC, tx_hash DESC
-- Partial on failure_category IS NOT NULL, so only failed invocations are
-- indexed. It is alone in its own migration because CREATE INDEX
-- CONCURRENTLY cannot run inside a transaction (docs/migrations.md).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_invocations_failure_category
    ON invocations (failure_category, ledger DESC, tx_hash DESC)
    WHERE failure_category IS NOT NULL;
