-- ============================================================
-- 000016_report_subscriptions
--
-- Email digest subscriptions (issue #330). A subscriber receives a
-- daily or weekly summary of activity across tracked contracts.
--
-- frequency       'daily' | 'weekly'
-- day_of_week     0..6 (Sun..Sat); only meaningful for weekly digests
-- unsubscribed_at NULL while active; set on one-click unsubscribe so the
--                 row is retained for audit rather than hard-deleted.
-- ============================================================

CREATE TABLE report_subscriptions (
    id              TEXT        PRIMARY KEY,
    email           TEXT        NOT NULL,
    frequency       TEXT        NOT NULL
                                CHECK (frequency IN ('daily', 'weekly')),
    day_of_week     SMALLINT    NOT NULL DEFAULT 1
                                CHECK (day_of_week BETWEEN 0 AND 6),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    unsubscribed_at TIMESTAMPTZ
);

-- One active subscription per (email, frequency).
CREATE UNIQUE INDEX idx_report_subscriptions_email_freq
    ON report_subscriptions (email, frequency)
    WHERE unsubscribed_at IS NULL;

-- The digest cron scans active subscriptions by frequency.
CREATE INDEX idx_report_subscriptions_due
    ON report_subscriptions (frequency)
    WHERE unsubscribed_at IS NULL;
