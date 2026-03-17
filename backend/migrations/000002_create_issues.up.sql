CREATE TABLE issues (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_key VARCHAR(30) NOT NULL UNIQUE,
    project_id UUID NOT NULL REFERENCES projects(id),
    type VARCHAR(20) NOT NULL,
    title VARCHAR(500) NOT NULL,
    description TEXT,
    priority VARCHAR(5) NOT NULL DEFAULT 'P2',
    status VARCHAR(20) NOT NULL DEFAULT 'todo',
    assignee_id VARCHAR(100),
    creator_id VARCHAR(100),
    source VARCHAR(200),
    version VARCHAR(50),
    git_url VARCHAR(500),
    pr_url VARCHAR(500),
    doc_url VARCHAR(500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_issues_project_id ON issues(project_id);
CREATE INDEX idx_issues_status ON issues(status);
CREATE INDEX idx_issues_type ON issues(type);
CREATE INDEX idx_issues_assignee ON issues(assignee_id);
CREATE INDEX idx_issues_priority ON issues(priority);
