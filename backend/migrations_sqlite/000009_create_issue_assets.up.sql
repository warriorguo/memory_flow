CREATE TABLE IF NOT EXISTS issue_assets (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    filename VARCHAR(255) NOT NULL,
    mime_type VARCHAR(255) NOT NULL,
    size_bytes BIGINT NOT NULL,
    checksum CHAR(64) NOT NULL,
    content BLOB,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (issue_id, filename)
);

CREATE INDEX idx_issue_assets_issue_id ON issue_assets(issue_id);
