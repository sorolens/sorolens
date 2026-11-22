-- Rollback full-text event search (issue #798).
DROP INDEX CONCURRENTLY IF EXISTS idx_events_event_tsv;
ALTER TABLE events DROP COLUMN IF EXISTS event_tsv;
