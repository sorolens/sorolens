-- ============================================================
-- Undo 000038: drop the archive job and restore the 000005 versions
-- of the partition helpers.
--
-- Limitation: partitions already moved to events_archive are NOT
-- re-attached (their month may now overlap events_default rows, and
-- re-attaching would silently resurrect data the retention job was
-- meant to bound). Move them back by hand with, per partition:
--   ALTER TABLE events_archive.events_YYYY_MM SET SCHEMA public;
--   ALTER TABLE events DETACH PARTITION events_default;  -- if empty
--   ALTER TABLE events ATTACH PARTITION events_YYYY_MM FOR VALUES FROM (...) TO (...);
--   CREATE TABLE events_default PARTITION OF events DEFAULT;
-- ============================================================

DROP FUNCTION IF EXISTS archive_old_partitions(int);

DROP FUNCTION IF EXISTS create_next_month_partition();
DROP FUNCTION IF EXISTS create_monthly_partition(int, int);

-- Restore the originals from 000005_partition_events.
CREATE OR REPLACE FUNCTION create_next_month_partition()
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  _year       int;
  _month      int;
  _start      timestamptz;
  _end        timestamptz;
  _partition_name text;
BEGIN
  _start := date_trunc('month', NOW() + INTERVAL '1 month');
  _end   := date_trunc('month', NOW() + INTERVAL '2 month');
  _year  := EXTRACT(YEAR FROM _start)::int;
  _month := EXTRACT(MONTH FROM _start)::int;
  _partition_name := format('events_%s_%s', _year, LPAD(_month::text, 2, '0'));

  IF NOT EXISTS (
    SELECT 1 FROM pg_class WHERE relname = _partition_name AND relkind = 'r'
  ) THEN
    IF NOT EXISTS (SELECT 1 FROM events_partitions WHERE year = _year AND month = _month) THEN
      EXECUTE format(
        'CREATE TABLE %I PARTITION OF events FOR VALUES FROM (%L) TO (%L)',
        _partition_name, _start, _end
      );
      INSERT INTO events_partitions (year, month) VALUES (_year, _month);
    END IF;
  END IF;
END;
$$;

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
    SELECT 1 FROM pg_class WHERE relname = _partition_name AND relkind = 'r'
  ) THEN
    IF NOT EXISTS (SELECT 1 FROM events_partitions WHERE year = p_year AND month = p_month) THEN
      EXECUTE format(
        'CREATE TABLE %I PARTITION OF events FOR VALUES FROM (%L) TO (%L)',
        _partition_name, _start, _end
      );
      INSERT INTO events_partitions (year, month) VALUES (p_year, p_month);
    END IF;
  END IF;
END;
$$;

-- Drop the default partition only when it holds no rows; refusing
-- otherwise keeps archived-but-landed-in-default data from vanishing
-- with the schema.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_class WHERE relname = 'events_default' AND relkind = 'r') THEN
    IF NOT EXISTS (SELECT 1 FROM public.events_default LIMIT 1) THEN
      EXECUTE 'DROP TABLE public.events_default';
    ELSE
      RAISE NOTICE 'events_default contains rows; leaving it in place';
    END IF;
  END IF;
END $$;

-- The schema drops only when empty (already-archived partitions stay).
DROP SCHEMA IF EXISTS events_archive;
