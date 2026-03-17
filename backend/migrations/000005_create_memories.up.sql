CREATE TABLE memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id),
    type VARCHAR(20) NOT NULL,
    title VARCHAR(500) NOT NULL,
    content TEXT NOT NULL,
    source_object_type VARCHAR(20),
    source_object_id UUID,
    creator_id VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_memories_project_id ON memories(project_id);
CREATE INDEX idx_memories_type ON memories(type);
CREATE INDEX idx_memories_source ON memories(source_object_type, source_object_id);
