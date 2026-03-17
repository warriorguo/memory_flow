CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key VARCHAR(20) NOT NULL UNIQUE,
    name VARCHAR(200) NOT NULL,
    summary TEXT,
    description TEXT,
    design_principles TEXT,
    git_url VARCHAR(500),
    cicd_url VARCHAR(500),
    doc_url VARCHAR(500),
    owner_id VARCHAR(100),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    next_issue_number INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
