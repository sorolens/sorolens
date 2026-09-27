-- ============================================================
-- 000013_contract_tags
--
-- User-defined multi-tags per contract (issue #163). A tag is a
-- free-form label such as "prod", "defi", or "staging" that lets an
-- operator organize a large contract fleet by role, environment, or
-- team, and filter the dashboard by tag.
--
-- The (contract_id, tag) primary key enforces uniqueness within a
-- contract and serves per-contract tag reads; idx_contract_tags_tag
-- backs the cross-contract "list contracts with tag X" filter.
-- ============================================================

CREATE TABLE contract_tags (
    contract_id TEXT        NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    tag         TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (contract_id, tag)
);

CREATE INDEX idx_contract_tags_tag ON contract_tags (tag);
