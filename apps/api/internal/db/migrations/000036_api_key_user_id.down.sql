DROP INDEX IF EXISTS idx_api_keys_user_id;
ALTER TABLE api_keys DROP COLUMN user_id;
