-- ============================================================
-- 000009_groups
--
-- User-owned contract groups (portfolios). A group is a named
-- set of tracked contracts; a contract may belong to any number
-- of groups. Aggregate portfolio statistics (events,
-- invocations, storage entries, average health score) are derived
-- from the existing per-contract tables, so only membership is
-- stored here.
-- ============================================================
CREATE TABLE groups (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id   TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The portfolio list is always scoped to one owner, newest first.
CREATE INDEX idx_groups_owner_created ON groups (owner_id, created_at DESC);

CREATE TABLE group_contracts (
    group_id    UUID        NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    contract_id TEXT        NOT NULL REFERENCES contracts (id) ON DELETE CASCADE,
    added_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, contract_id)
);

-- Supports "which groups is this contract in?" lookups from the
-- contract detail page without scanning the join table.
CREATE INDEX idx_group_contracts_contract ON group_contracts (contract_id);
