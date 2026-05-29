CREATE TABLE issue_history (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    field_name VARCHAR(50) NOT NULL,
    old_value TEXT,
    new_value TEXT,
    operator_id VARCHAR(100),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_issue_history_issue_id ON issue_history(issue_id);
