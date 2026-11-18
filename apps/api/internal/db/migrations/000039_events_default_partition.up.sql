-- ============================================================
-- 000039_events_default_partition
--
-- Add a DEFAULT partition to the events table so that historical backfills,
-- future events, or gaps outside provisioned monthly partitions can always
-- be inserted without failing.
-- ============================================================

CREATE TABLE IF NOT EXISTS events_default PARTITION OF events DEFAULT;
