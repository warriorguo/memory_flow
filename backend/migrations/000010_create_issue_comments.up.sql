CREATE TABLE IF NOT EXISTS issue_comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    author_id VARCHAR(255),
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_issue_comments_issue_id ON issue_comments(issue_id, created_at);

-- One row per (comment, reader): a read receipt.
--
-- The obvious alternative — one "last read at" timestamp per reader — is not
-- exact: SQLite's CURRENT_TIMESTAMP has second resolution, so a comment written
-- in the same second as the marker is indistinguishable from one the reader has
-- already seen, and would silently never be reported as new. Receipts cost a row
-- per comment per reader and answer the question exactly.
CREATE TABLE IF NOT EXISTS issue_comment_reads (
    comment_id UUID NOT NULL REFERENCES issue_comments(id) ON DELETE CASCADE,
    reader VARCHAR(255) NOT NULL,
    read_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (comment_id, reader)
);
