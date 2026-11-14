DROP TABLE IF EXISTS memory_reads;
DROP INDEX IF EXISTS idx_issues_epic;
ALTER TABLE issues DROP COLUMN epic_id;
