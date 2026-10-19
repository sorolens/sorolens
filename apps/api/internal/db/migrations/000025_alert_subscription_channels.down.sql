ALTER TABLE alert_subscriptions
    DROP COLUMN IF EXISTS routing_key,
    DROP COLUMN IF EXISTS channel_type;
