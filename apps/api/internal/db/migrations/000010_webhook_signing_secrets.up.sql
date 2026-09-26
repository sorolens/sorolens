-- ============================================================
-- 000010_webhook_signing_secrets
--
-- Backs the HMAC-SHA256 webhook signature feature. Every outgoing watchdog
-- delivery is signed with the subscription's secret so a receiver can prove the
-- payload came from Sorolens (see docs/webhooks.md).
--
-- Two secret columns are kept on purpose:
--   * signing_secret      - the recoverable "whsec_..." key. HMAC signing needs
--                           the raw key, so unlike an API key it cannot be
--                           stored as a one-way hash alone.
--   * signing_secret_hash - SHA-256 of the key, persisted alongside it as the
--                           auditable digest of the stored secret.
--
-- signing_secret_created_at starts the 5-minute window during which
-- GET .../signing-secret will return the secret; signing_secret_rotated_at is
-- stamped on every rotation and re-opens that window.
-- ============================================================

ALTER TABLE alert_subscriptions
    ADD COLUMN IF NOT EXISTS signing_secret            TEXT,
    ADD COLUMN IF NOT EXISTS signing_secret_hash       TEXT,
    ADD COLUMN IF NOT EXISTS signing_secret_created_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS signing_secret_rotated_at TIMESTAMPTZ;

-- Subscriptions created before this migration have no secret. Generate one for
-- each so every delivery can be signed: two random UUIDs give 64 hex characters
-- (256 bits) of entropy, and sha256()/gen_random_uuid() are core Postgres
-- functions (no pgcrypto required).
UPDATE alert_subscriptions AS s
SET signing_secret            = g.secret,
    signing_secret_hash       = encode(sha256(g.secret::bytea), 'hex'),
    signing_secret_created_at = NOW()
FROM (
    SELECT id,
           'whsec_' || replace(gen_random_uuid()::text, '-', '')
                    || replace(gen_random_uuid()::text, '-', '') AS secret
    FROM alert_subscriptions
) AS g
WHERE s.id = g.id
  AND s.signing_secret IS NULL;

ALTER TABLE alert_subscriptions
    ALTER COLUMN signing_secret            SET NOT NULL,
    ALTER COLUMN signing_secret_hash       SET NOT NULL,
    ALTER COLUMN signing_secret_created_at SET NOT NULL;

ALTER TABLE alert_subscriptions
    ALTER COLUMN signing_secret_created_at SET DEFAULT NOW();
