-- ============================================================
-- 000006_user_roles
--
-- Role-based access control. Users get one of:
--   viewer      - read-only (default)
--   contributor - can register contracts / update labels
--   admin       - everything, plus API key management and the
--                 /api/v1/admin/* surface.
--
-- The initial admin is seeded from INITIAL_ADMIN_GITHUB_ID (see main.go).
-- ============================================================

ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'viewer';