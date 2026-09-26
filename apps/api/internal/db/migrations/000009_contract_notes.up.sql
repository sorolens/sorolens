-- ============================================================
-- contract_notes
-- Markdown notes ("institutional knowledge") attached to a
-- tracked contract and rendered on its detail page.
-- ============================================================
CREATE TABLE contract_notes (
    id          TEXT        PRIMARY KEY,
    contract_id TEXT        NOT NULL REFERENCES contracts (id) ON DELETE CASCADE,
    author      TEXT        NOT NULL DEFAULT 'anonymous',
    body        TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The detail page always lists notes for one contract, newest first.
CREATE INDEX idx_contract_notes_contract ON contract_notes (contract_id, created_at DESC);
