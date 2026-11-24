-- Rollback the failure_category index (issue #806).
DROP INDEX CONCURRENTLY IF EXISTS idx_invocations_failure_category;
