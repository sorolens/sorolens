-- 000017_events_global_composite_indexes.up.sql
--
-- Composite indexes for the global events feed.
--
-- The global feed queries in `store/events_global.go` filter by a
-- contract id and a cross-contract time range, then order by time.
-- The existing single-column indexes force a sequential scan or an
-- expensive sort once the table grows beyond ~1m rows. These
-- composite indexes allow the planner to satisfy the filter and the
-- ordering in a single index scan.
--
-- Write cost is kept in mind: only the columns actually used by
-- the feed queries are indexed, and the indexes are created
-- concurrently so the migration does not lock writes on the hot
-- events table.
--
-- Rollback is handled by the companion
-- `000017_events_global_composite_indexes.down.sql` migration.

-- Global feed for a single contract:
-*   SELECT ... FROM events
-*   WHERE contract_id = $1 AND created_at BETWEEN $2 AND $3
-*   ORDER BY created_at DESC;
-*   SELECT ... FROM events
-*   WHERE contract_id LIKE $1 || '%' AND created_at BETWEEN $2 AND $3
-*   ORDER BY created_at DESC;
--
-- The `(contract_id, created_at DESC)` index serves the equality
-- form directly and the `LIKE 'prefix%'` form via an index range
-- scan on `contract_id` when the collation is C (default), and it
-- also satisfies the DESC order without an extra sort.
--
-- The `(created_at DESC, contract_id)` index serves the
-- cross-contract time-range query where no contract filter is
-- applied (or where the contract filter is not selective enough).
--
-- Both indexes are created CONCURRENTLY so the migration does not
-- block writes on the events table. CONCURRENTLY
-- index creation cannot run inside a transaction block, so the
-- migration is executed as a single statement per index without
-- an explicit BEGIN/COMMIT wrapper.

CREATE INDEX CONCURRENTLY IF NOT EXISTS
    events_contract_id_created_at_desc_idx
    ON events (contract_id, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS
    events_created_at_desc_contract_id_idx
    ON events (created_at DESC, contract_id);
