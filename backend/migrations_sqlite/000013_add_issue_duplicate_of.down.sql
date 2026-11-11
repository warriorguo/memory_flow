DROP INDEX IF EXISTS idx_issues_duplicate_of;
ALTER TABLE issues DROP COLUMN duplicate_of_id;
