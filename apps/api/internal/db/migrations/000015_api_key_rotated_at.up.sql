-- ============================================================
-- 000015_api_key_rotated_at
--
-- Support atomic API key rotation (issue #328). Rotating a key
-- regenerates its secret in place: the key_hash is replaced with
-- the hash of a fresh token and rotated_at records when it last
-- happened. The key id, name, scopes and created_at are untouched,
-- so integrations keep the same key identity across a rotation.
--
-- rotated_at  NULL until the key has been rotated at least once.
-- ============================================================

ALTER TABLE api_keys ADD COLUMN rotated_at TIMESTAMPTZ;
