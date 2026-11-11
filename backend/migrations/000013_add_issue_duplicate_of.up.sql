-- MF-36: an issue closed as a duplicate points at the issue it duplicates.
-- Only meaningful while status = 'duplicate'; reopening clears it.
ALTER TABLE issues ADD COLUMN duplicate_of_id UUID REFERENCES issues(id) ON DELETE SET NULL;
CREATE INDEX idx_issues_duplicate_of ON issues(duplicate_of_id);
