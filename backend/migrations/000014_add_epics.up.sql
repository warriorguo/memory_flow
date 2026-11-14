-- MF-39: epics. An epic is an issue of type 'epic'; any other issue (in any
-- project) may belong to at most one epic through epic_id. Membership is not a
-- dependency: it carries no ordering and no priority inheritance.
ALTER TABLE issues ADD COLUMN epic_id UUID REFERENCES issues(id) ON DELETE SET NULL;
CREATE INDEX idx_issues_epic ON issues(epic_id);

-- Which version of a memory each reader last read. content_hash fingerprints
-- the title and content at read time, so "changed since you read it" is a
-- comparison of hashes rather than of timestamps (second resolution on SQLite
-- cannot order an edit and a read made in the same second).
CREATE TABLE IF NOT EXISTS memory_reads (
    memory_id UUID NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    reader VARCHAR(255) NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    read_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (memory_id, reader)
);
