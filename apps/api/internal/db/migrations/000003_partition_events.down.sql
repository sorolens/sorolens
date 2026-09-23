-- ============================================================
-- Revert the events table from partitioned back to a single table.
-- This migration is idempotent: running it twice is safe.
-- ============================================================

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_class WHERE relname = 'events' AND relkind = 'p'
  ) THEN

    -- Rename the partitioned table.
    ALTER TABLE events RENAME TO events_new;

    -- Recreate the original single table.
    CREATE TABLE events (
      id                 TEXT        PRIMARY KEY,
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
      inserted_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

    -- Copy data back.
    INSERT INTO events
      SELECT id, contract_id, ledger, ledger_closed_at, tx_hash, type,
             topic_xdr, value_xdr, topic_decoded, value_decoded,
             in_successful_call, inserted_at
      FROM events_new;

    -- Recreate the original indexes.
    CREATE INDEX idx_events_contract_ledger ON events (contract_id, ledger DESC);
    CREATE INDEX idx_events_tx_hash ON events (tx_hash);
    CREATE INDEX idx_events_ledger_closed_at ON events (ledger_closed_at DESC);

    -- Drop the partitioned table and its partitions.
    DROP TABLE events_new;

  END IF;
END $$;

-- Clean up partition tracking and helper function.
DROP FUNCTION IF EXISTS create_next_month_partition();
DROP TABLE IF EXISTS events_partitions;
