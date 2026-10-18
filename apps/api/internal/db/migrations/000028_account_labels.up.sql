CREATE TABLE labels_public (
    label TEXT PRIMARY KEY,
    value TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (length(value) = 56)
);

CREATE TABLE labels_workspace (
    workspace_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    value TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, label),
    UNIQUE (workspace_id, value),
    CHECK (length(value) = 56)
);

CREATE INDEX idx_labels_public_value ON labels_public (value);
CREATE INDEX idx_labels_workspace_value ON labels_workspace (workspace_id, value);

INSERT INTO labels_public (label, value) VALUES
    ('sorolens-admin', 'GAZ3HN2QNDKWLOI2OQEG65KBJEAUP4PROR3FJNXNDY34UH547MN4CJUI'),
    ('sorolens-watchdog', 'CACXRL67WL5KRD6HKWGYADHEUF6RQOCODUN26UQE7MGFZEMIR7PAX6R7')
ON CONFLICT DO NOTHING;