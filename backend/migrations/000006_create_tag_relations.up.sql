CREATE TABLE issue_tag_rel (
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, tag_id)
);
CREATE TABLE memory_tag_rel (
    memory_id UUID NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (memory_id, tag_id)
);
