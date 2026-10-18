-- ============================================================
-- Partition the events table by month on ledger_closed_at.
-- This migration is idempotent: running it twice is safe.
-- ============================================================

DO $$
DECLARE
  _start      timestamptz;
  _end        timestamptz;
  _year       int;
  _month      int;
  _partition_name text;
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_class WHERE relname = 'events' AND relkind = 'r'
  ) AND NOT EXISTS (
    SELECT 1 FROM pg_inherits WHERE inhrelid = 'events'::regclass
  ) THEN

    -- Rename the original table so we can recreate it as partitioned.
    ALTER TABLE events RENAME TO events_old;

    -- Create the partitioned table with the same columns.
    -- Primary key must include the partition key (ledger_closed_at).
    CREATE TABLE events (
      id                 TEXT        NOT NULL,
      contract_id        TEXT        NOT NULL REFERENCES contracts (id),
      ledger             BIGINT      NOT NULL,
      ledger_closed_at   TIMESTAMPTZ NOT NULL,
      tx_hash            TEXT        NOT NULL,
      type               TEXT        NOT NULL,
      topic_xdr          JSONB       NOT NULL,
      value_xdr          TEXT        NOT NULL,
      topic_decoded      JSONB,
      value_decoded      JSONB,
      in_successful_call BOOLEAN     NOT NULL DEFAULT TRUE,
      inserted_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      PRIMARY KEY (id, ledger_closed_at)
    ) PARTITION BY RANGE (ledger_closed_at);

    -- Copy all existing data into the partitioned table.
    INSERT INTO events
      SELECT id, contract_id, ledger, ledger_closed_at, tx_hash, type,
             topic_xdr, value_xdr, topic_decoded, value_decoded,
             in_successful_call, inserted_at
      FROM events_old;

    -- Create monthly partition for the current month.
    _start := date_trunc('month', NOW());
    _end   := date_trunc('month', NOW() + INTERVAL '1 month');
    _year  := EXTRACT(YEAR FROM _start)::int;
    _month := EXTRACT(MONTH FROM _start)::int;
    _partition_name := format('events_%s_%s', _year, LPAD(_month::text, 2, '0'));
    EXECUTE format(
      'CREATE TABLE %I PARTITION OF events FOR VALUES FROM (%L) TO (%L)',
      _partition_name, _start, _end
    );

    -- Create monthly partition for next month.
    _start := date_trunc('month', NOW() + INTERVAL '1 month');
    _end   := date_trunc('month', NOW() + INTERVAL '2 month');
    _year  := EXTRACT(YEAR FROM _start)::int;
    _month := EXTRACT(MONTH FROM _start)::int;
    _partition_name := format('events_%s_%s', _year, LPAD(_month::text, 2, '0'));
    EXECUTE format(
      'CREATE TABLE %I PARTITION OF events FOR VALUES FROM (%L) TO (%L)',
      _partition_name, _start, _end
    );

    -- Recreate the indexes on the partitioned table.
    -- Indexes on the parent propagate to all partitions automatically.
    -- We recreate them explicitly to ensure they match the originals.
    CREATE INDEX idx_events_contract_ledger ON events (contract_id, ledger DESC);
    CREATE INDEX idx_events_tx_hash ON events (tx_hash);
    CREATE INDEX idx_events_ledger_closed_at ON events (ledger_closed_at DESC);

    -- Drop the old table.
    DROP TABLE events_old;

  END IF;
END $$;

-- ============================================================
-- Partition tracking table and helper function for auto-creating
-- future partitions.
-- ============================================================

CREATE TABLE IF NOT EXISTS events_partitions (
  year  SMALLINT  NOT NULL,
  month SMALLINT  NOT NULL,
  PRIMARY KEY (year, month)
);

-- Insert the current month and next month as already provisioned.
INSERT INTO events_partitions (year, month)
VALUES
  (EXTRACT(YEAR FROM NOW())::int, EXTRACT(MONTH FROM NOW())::int),
  (EXTRACT(YEAR FROM (NOW() + INTERVAL '1 month'))::int, EXTRACT(MONTH FROM (NOW() + INTERVAL '1 month'))::int)
ON CONFLICT DO NOTHING;

-- ============================================================
-- Function for auto-creating next month's partition.
-- Can be called from a scheduled job (e.g., pg_cron) or from
-- the indexer on each cycle.
-- ============================================================
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
