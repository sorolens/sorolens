DROP TABLE IF EXISTS webhook_deliveries;
ALTER TABLE alert_subscriptions DROP COLUMN IF EXISTS last_delivery_status;
ALTER TABLE alert_subscriptions DROP COLUMN IF EXISTS last_delivery_at;
