-- Issue #805: per-invocation fee breakdown.
--
-- Store the full fee components parsed from the transaction result meta so the
-- API and stats endpoint can expose inclusion fee, resource fee, refundable
-- portion and actual refund. All values are integer stroops. The columns are
-- added nullable so existing rows stay valid; the indexer fills them going
-- forward and may backfill historical rows on a re-index.
ALTER TABLE invocations ADD COLUMN IF NOT EXISTS inclusion_fee            BIGINT;
ALTER TABLE invocations ADD COLUMN IF NOT EXISTS resource_fee             BIGINT;
ALTER TABLE invocations ADD COLUMN IF NOT EXISTS resource_fee_refundable  BIGINT;
ALTER TABLE invocations ADD COLUMN IF NOT EXISTS fee_refund               BIGINT;
