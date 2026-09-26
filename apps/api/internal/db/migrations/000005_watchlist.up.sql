CREATE TABLE users (
    id TEXT PRIMARY KEY,
    github_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE watchlist_items (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    contract_id TEXT NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, contract_id)
);
