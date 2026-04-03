CREATE TABLE IF NOT EXISTS issue_dependencies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    target_issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL CHECK (type IN ('depends_on', 'blocks')),
    severity VARCHAR(20) NOT NULL DEFAULT 'recommended' CHECK (severity IN ('critical', 'recommended')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_issue_id, target_issue_id, type)
);

CREATE INDEX idx_issue_deps_source ON issue_dependencies(source_issue_id);
CREATE INDEX idx_issue_deps_target ON issue_dependencies(target_issue_id);
