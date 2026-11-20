-- ============================================================
-- 000038_api_key_allowlists
--
-- Source-address and browser-origin restrictions for API keys
-- (issue #793). A leaked key restricted to the owner's networks
-- and origins is far less dangerous than an unrestricted one.
--
-- allowed_cidrs    NULL or empty means no address restriction.
-- allowed_origins  NULL or empty means no Origin restriction.
-- Both are TEXT[] so a key can carry a small, ordered set; the
-- API normalises CIDRs to their network form and origins to
-- scheme://host[:port] before storing them.
-- ============================================================

ALTER TABLE api_keys
    ADD COLUMN allowed_cidrs TEXT[],
    ADD COLUMN allowed_origins TEXT[];

COMMENT ON COLUMN api_keys.allowed_cidrs IS
    'CIDR ranges the key may be used from. NULL/empty means unrestricted.';
COMMENT ON COLUMN api_keys.allowed_origins IS
    'Browser origins the key may be used from. NULL/empty means unrestricted.';
