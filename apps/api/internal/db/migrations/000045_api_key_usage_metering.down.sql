-- 000045_api_key_usage_metering.down.sql
DROP TABLE IF EXISTS api_key_usage;
ALTER TABLE api_keys DROP COLUMN IF EXISTS monthly_quota;
