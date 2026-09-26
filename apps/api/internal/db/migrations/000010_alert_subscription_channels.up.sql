-- ============================================================
-- 000010_alert_subscription_channels
--
-- Pluggable notification channels (issue #127). Each alert
-- subscription picks a channel that decides the payload format:
--   webhook   - generic JSON POST (the original behaviour)
--   slack     - Slack incoming webhook, Block Kit message
--   discord   - Discord webhook, embed message
--   pagerduty - PagerDuty Events API v2 trigger event
--
-- routing_key holds the PagerDuty integration key. It is a secret
-- and is never returned by the API.
-- ============================================================

ALTER TABLE alert_subscriptions
    ADD COLUMN channel_type TEXT NOT NULL DEFAULT 'webhook'
        CHECK (channel_type IN ('webhook', 'slack', 'discord', 'pagerduty')),
    ADD COLUMN routing_key  TEXT NOT NULL DEFAULT '';
