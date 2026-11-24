-- Issue #828: webhook delivery log with payload inspector and manual retry.
--
-- webhook_deliveries records every attempted delivery for an alert
-- subscription, so operators can inspect what was sent and retry failures from
-- the UI.  Deliveries are written by the indexer's delivery worker and read by
-- the API.
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id               TEXT        NOT NULL PRIMARY KEY,
    subscription_id  TEXT        NOT NULL REFERENCES alert_subscriptions(id) ON DELETE CASCADE,
    -- attempt_number starts at 1; retries increment it.
    attempt_number   INT         NOT NULL DEFAULT 1,
    status           TEXT        NOT NULL DEFAULT 'pending',
    -- signed_payload is the JSON body that was (or will be) sent.
    -- The signing secret is NOT stored here; only the signed payload.
    signed_payload   JSONB       NOT NULL DEFAULT '{}',
    -- response fields are NULL until the delivery attempt completes.
    response_status  INT,
    response_body    TEXT,
    -- last_error is the last transport-level error (non-HTTP failure).
    last_error       TEXT,
    -- next_retry_at is set by the backoff worker after a failure.
    next_retry_at    TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Speed up browsing by subscription (the primary UI access pattern).
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_subscription_created
    ON webhook_deliveries (subscription_id, created_at DESC);

-- Speed up the backoff worker's "what needs retrying" scan.
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_next_retry
    ON webhook_deliveries (next_retry_at)
    WHERE status = 'failed' AND next_retry_at IS NOT NULL;
