ALTER TABLE alert_subscriptions
    DROP COLUMN IF EXISTS signing_secret,
    DROP COLUMN IF EXISTS signing_secret_hash,
    DROP COLUMN IF EXISTS signing_secret_created_at,
    DROP COLUMN IF EXISTS signing_secret_rotated_at;
