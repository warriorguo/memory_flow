CREATE TABLE IF NOT EXISTS issue_dependencies (
    id TEXT PRIMARY KEY,
    source_issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    target_issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL CHECK (type IN ('depends_on', 'blocks')),
    severity VARCHAR(20) NOT NULL DEFAULT 'recommended' CHECK (severity IN ('critical', 'recommended')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (source_issue_id, target_issue_id, type)
);

CREATE INDEX idx_issue_deps_source ON issue_dependencies(source_issue_id);
CREATE INDEX idx_issue_deps_target ON issue_dependencies(target_issue_id);
