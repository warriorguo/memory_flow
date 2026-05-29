CREATE TABLE memories (
    id TEXT PRIMARY KEY,
    project_id TEXT REFERENCES projects(id),
    type VARCHAR(20) NOT NULL,
    title VARCHAR(500) NOT NULL,
    content TEXT NOT NULL,
    source_object_type VARCHAR(20),
    source_object_id TEXT,
    creator_id VARCHAR(100),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_memories_project_id ON memories(project_id);
CREATE INDEX idx_memories_type ON memories(type);
CREATE INDEX idx_memories_source ON memories(source_object_type, source_object_id);
