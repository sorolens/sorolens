ALTER TABLE api_keys
    DROP COLUMN IF EXISTS allowed_origins,
    DROP COLUMN IF EXISTS allowed_cidrs;
