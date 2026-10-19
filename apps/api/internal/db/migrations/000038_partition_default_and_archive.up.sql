-- ============================================================
-- Partition maintenance for issue #514:
--   1. a DEFAULT partition so inserts never fail for lack of a
--      monthly partition (they land in events_default instead),
--   2. an archive_old_partitions() job that detaches months older
--      than a configurable retention window and moves them to the
--      events_archive schema instead of dropping them,
--   3. schema-scoped partition helpers, so a name that already
--      exists in events_archive never blocks re-creating the
--      live partition.
-- ============================================================

-- 1. Default partition, only if events is actually partitioned
-- (000005's conversion is guarded and may not have run).
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'public' AND c.relname = 'events' AND c.relkind = 'p'
  ) AND NOT EXISTS (
    SELECT 1 FROM pg_inherits i
    JOIN pg_class c ON c.oid = i.inhrelid
    WHERE c.relname = 'events_default'
  ) THEN
    EXECUTE 'CREATE TABLE events_default PARTITION OF events DEFAULT';
  END IF;
END $$;

CREATE SCHEMA IF NOT EXISTS events_archive;

-- 2. create_monthly_partition, scoped to public and to the events
-- parent's attachment list. The old version matched by bare relname,
-- which an archived table would block.
CREATE OR REPLACE FUNCTION create_monthly_partition(p_year int, p_month int)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  _start      timestamptz;
  _end        timestamptz;
  _partition_name text;
BEGIN
  _start := make_timestamptz(p_year, p_month, 1, 0, 0, 0, 'UTC');
  _end   := _start + INTERVAL '1 month';
  _partition_name := format('events_%s_%s', p_year, LPAD(p_month::text, 2, '0'));

  IF NOT EXISTS (
    SELECT 1 FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'public' AND c.relname = _partition_name
  ) AND NOT EXISTS (
    -- already attached to another parent? refuse rather than collide.
    SELECT 1 FROM pg_inherits i
    JOIN pg_class c ON c.oid = i.inhrelid
    WHERE c.relname = _partition_name
  ) THEN
    IF NOT EXISTS (SELECT 1 FROM events_partitions WHERE year = p_year AND month = p_month) THEN
      EXECUTE format(
        'CREATE TABLE public.%I PARTITION OF events FOR VALUES FROM (%L) TO (%L)',
        _partition_name, _start, _end
      );
      INSERT INTO events_partitions (year, month) VALUES (p_year, p_month);
    END IF;
  END IF;
END;
$$;

-- Same next-month wrapper, now delegating so both share the fixed checks.
CREATE OR REPLACE FUNCTION create_next_month_partition()
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  _next timestamptz := date_trunc('month', NOW() + INTERVAL '1 month');
BEGIN
  PERFORM create_monthly_partition(
    EXTRACT(YEAR FROM _next)::int,
    EXTRACT(MONTH FROM _next)::int
  );
END;
$$;

-- 3. Retention: detach monthly partitions whose whole range is older
-- than p_retain_months back, and move them out of the live table
-- instead of deleting them. Rows stay queryable as
-- events_archive.events_YYYY_MM. events_default is never touched —
-- it has no month boundary to age out.
CREATE OR REPLACE FUNCTION archive_old_partitions(p_retain_months int)
RETURNS SETOF TEXT
LANGUAGE plpgsql
AS $$
DECLARE
  rec      RECORD;
  _cutoff  timestamptz;
  _y       int;
  _m       int;
BEGIN
  IF p_retain_months <= 0 THEN
    RETURN;
  END IF;
  _cutoff := date_trunc('month', NOW()) - (p_retain_months * INTERVAL '1 month');

  FOR rec IN
    SELECT c.relname AS part_name
    FROM pg_inherits i
    JOIN pg_class c ON c.oid = i.inhrelid
    JOIN pg_class p ON p.oid = i.inhparent
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE p.relname = 'events'
      AND n.nspname = 'public'
      AND c.relname ~ '^events_[0-9]{4}_[0-9]{2}$'
    ORDER BY c.relname
  LOOP
    _y := substring(rec.part_name FROM 8 FOR 4)::int;
    _m := substring(rec.part_name FROM 13 FOR 2)::int;

    -- Archive only when the partition's upper bound is at or before the
    -- cutoff; the current and future months always stay live.
    IF make_timestamptz(_y, _m, 1, 0, 0, 0, 'UTC') + INTERVAL '1 month' <= _cutoff THEN
      EXECUTE format('ALTER TABLE public.events DETACH PARTITION %I', rec.part_name);
      EXECUTE format('ALTER TABLE public.%I SET SCHEMA events_archive', rec.part_name);
      DELETE FROM events_partitions WHERE year = _y AND month = _m;
      RETURN NEXT rec.part_name;
    END IF;
  END LOOP;
END;
$$;
