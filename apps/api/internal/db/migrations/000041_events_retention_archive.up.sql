-- ============================================================
-- 000041_events_retention_archive
--
-- Retention archive for events older than a configurable threshold
-- (ROADMAP v0.3). Archived events are preserved in events_archive rather
-- than permanently deleted.
-- Includes helper functions for pre-creating future partitions
-- and archiving events beyond retention window.
-- ============================================================

CREATE TABLE IF NOT EXISTS events_archive (
    id                 TEXT        NOT NULL,
    contract_id        TEXT        NOT NULL,
    network            TEXT        NOT NULL DEFAULT 'testnet',
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
    archived_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, ledger_closed_at)
);

CREATE INDEX IF NOT EXISTS idx_events_archive_contract_ledger ON events_archive (contract_id, ledger DESC);
CREATE INDEX IF NOT EXISTS idx_events_archive_ledger_closed_at ON events_archive (ledger_closed_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_archive_tx_hash ON events_archive (tx_hash);

CREATE OR REPLACE VIEW events_with_archive AS
SELECT id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
       topic_xdr, value_xdr, topic_decoded, value_decoded,
       in_successful_call, inserted_at, false AS is_archived
FROM events
UNION ALL
SELECT id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
       topic_xdr, value_xdr, topic_decoded, value_decoded,
       in_successful_call, inserted_at, true AS is_archived
FROM events_archive;

CREATE OR REPLACE FUNCTION maintain_events_partitions(p_months_ahead int DEFAULT 6)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  _start timestamptz;
  _end   timestamptz;
  _year  int;
  _month int;
  _partition_name text;
  _i     int;
BEGIN
  FOR _i IN 0..p_months_ahead LOOP
    _start := date_trunc('month', NOW() + (_i || ' month')::interval);
    _end   := date_trunc('month', NOW() + ((_i + 1) || ' month')::interval);
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
        INSERT INTO events_partitions (year, month) VALUES (_year, _month) ON CONFLICT DO NOTHING;
      END IF;
    END IF;
  END LOOP;
END;
$$;

CREATE OR REPLACE FUNCTION archive_old_events(p_retention_days int)
RETURNS int
LANGUAGE plpgsql
AS $$
DECLARE
  _cutoff timestamptz := NOW() - (p_retention_days || ' days')::interval;
  _moved int := 0;
BEGIN
  WITH moved AS (
    DELETE FROM events
    WHERE ledger_closed_at < _cutoff
    RETURNING id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
              topic_xdr, value_xdr, topic_decoded, value_decoded,
              in_successful_call, inserted_at
  )
  INSERT INTO events_archive (
    id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
    topic_xdr, value_xdr, topic_decoded, value_decoded,
    in_successful_call, inserted_at, archived_at
  )
  SELECT id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
         topic_xdr, value_xdr, topic_decoded, value_decoded,
         in_successful_call, inserted_at, NOW()
  FROM moved
  ON CONFLICT (id, ledger_closed_at) DO NOTHING;

  GET DIAGNOSTICS _moved = ROW_COUNT;
  RETURN _moved;
END;
$$;
