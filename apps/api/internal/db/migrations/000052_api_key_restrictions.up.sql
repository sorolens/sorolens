-- Per-key network and browser-origin restrictions (issue #793).
--
-- Both columns are nullable and default to NULL, so every existing key keeps
-- its current unrestricted behaviour until a restriction is set.
ALTER TABLE api_keys ADD COLUMN allowed_cidrs TEXT[];
ALTER TABLE api_keys ADD COLUMN allowed_origins TEXT[];

COMMENT ON COLUMN api_keys.allowed_cidrs IS
  'Source networks allowed to use this key. NULL or empty means any address.';
COMMENT ON COLUMN api_keys.allowed_origins IS
  'Browser Origins allowed to use this key. NULL or empty means any origin.';
