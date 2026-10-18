-- ============================================================
-- 000034_contract_notes
--
-- Markdown notes attached to a tracked contract (issue #164).
-- A note is the institutional knowledge that does not belong in
-- any single indexed row: why a contract was migrated, who owns
-- it, what changed at which point in time.
--
-- id          generated here so both the API and the indexer can
--             insert without coordinating an ID scheme
-- author      the identity that wrote the note, as passed in
--             X-User-ID; only that identity may later edit or
--             delete the row
-- body        markdown source; rendering is the client's job
-- created_at  set on insert, never rewritten
-- updated_at  bumped by edits, so the detail page can show that
--             a note was revised after it was written
-- ============================================================

CREATE TABLE contract_notes (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    contract_id TEXT        NOT NULL REFERENCES contracts (id) ON DELETE CASCADE,
    author      TEXT        NOT NULL,
    body        TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- A note is always read back through its contract, and a body that is
    -- only whitespace has nothing to render.
    CONSTRAINT contract_notes_body_not_blank CHECK (length(btrim(body)) > 0)
);

-- The only read pattern: "the notes on this contract, newest first."
CREATE INDEX idx_contract_notes_contract_created
    ON contract_notes (contract_id, created_at DESC);

-- Supports "everything this contributor has written" and keeps deletes of a
-- departed author's notes from scanning the table.
CREATE INDEX idx_contract_notes_author ON contract_notes (author);
