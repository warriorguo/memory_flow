-- MF-39: epics. An epic is an issue of type 'epic'; any other issue (in any
-- project) may belong to at most one epic through epic_id. Membership is not a
-- dependency: it carries no ordering and no priority inheritance.
ALTER TABLE issues ADD COLUMN epic_id TEXT REFERENCES issues(id) ON DELETE SET NULL;
CREATE INDEX idx_issues_epic ON issues(epic_id);

-- Which version of a memory each reader last read; see the PostgreSQL
-- migration for why this stores a content hash rather than comparing times.
CREATE TABLE IF NOT EXISTS memory_reads (
    memory_id TEXT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    reader VARCHAR(255) NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    read_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (memory_id, reader)
);
