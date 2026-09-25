ALTER TABLE alert_subscriptions
ADD COLUMN IF NOT EXISTS last_delivery_status TEXT,
ADD COLUMN IF NOT EXISTS last_delivery_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id TEXT PRIMARY KEY,
    subscription_id TEXT NOT NULL REFERENCES alert_subscriptions(id) ON DELETE CASCADE,
    payload JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    attempt INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    response_code INT DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_sub ON webhook_deliveries(subscription_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_retry ON webhook_deliveries(status, next_attempt_at);
