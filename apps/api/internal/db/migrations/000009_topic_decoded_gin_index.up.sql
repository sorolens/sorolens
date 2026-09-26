-- ============================================================
-- 000009_topic_decoded_gin_index
--
-- Adds a GIN index on events.topic_decoded so filtering by a
-- decoded topic value (the API's ?topic= query parameter) uses an
-- index instead of scanning every event of a contract.
--
-- The API matches decoded topics with the JSONB containment
-- operator:
--
--     topic_decoded @> '["transfer"]'::jsonb
--
-- jsonb_path_ops is the correct operator class for that operator:
-- @> is the only operator the topic filter needs, and
-- jsonb_path_ops produces a smaller and faster index for
-- containment than the default jsonb_ops class (which is only
-- required for key-existence `?`, `?|` and `?&` queries).
--
-- events is partitioned by month (see 000003_partition_events), so
-- a plain CREATE INDEX on the parent builds a partitioned index
-- that is applied to every existing partition and to partitions
-- created later. CREATE INDEX CONCURRENTLY is deliberately not
-- used: PostgreSQL does not support it on partitioned tables, and
-- CI applies each migration file outside an explicit transaction
-- anyway.
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_events_topic_decoded
    ON events USING GIN (topic_decoded jsonb_path_ops);
