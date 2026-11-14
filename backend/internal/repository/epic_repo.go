package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

// EpicRepo holds the queries behind epics (MF-39): an epic's issues, the
// dependencies around them, and the per-reader read state of memories.
type EpicRepo struct {
	db database.DB
}

func NewEpicRepo(db database.DB) *EpicRepo {
	return &EpicRepo{db: db}
}

// ListEpics returns a project's epics, newest first.
func (r *EpicRepo) ListEpics(ctx context.Context, projectID uuid.UUID) ([]model.Issue, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`SELECT %s FROM issues WHERE project_id = $1 AND type = $2 ORDER BY created_at DESC`, issueColumns),
		projectID, model.IssueTypeEpic)
	if err != nil {
		return nil, fmt.Errorf("list epics: %w", err)
	}
	defer rows.Close()
	var out []model.Issue
	for rows.Next() {
		i, err := scanIssue(rows)
		if err != nil {
			return nil, fmt.Errorf("scan epic: %w", err)
		}
		out = append(out, *i)
	}
	return out, rows.Err()
}

// StatusCountsByEpic tallies the issues of each epic by status.
func (r *EpicRepo) StatusCountsByEpic(ctx context.Context, epicIDs []uuid.UUID) (map[uuid.UUID]map[string]int, error) {
	out := make(map[uuid.UUID]map[string]int, len(epicIDs))
	if len(epicIDs) == 0 {
		return out, nil
	}
	in, args := uuidIn(epicIDs, 1)
	rows, err := r.db.Query(ctx, `SELECT epic_id, status, COUNT(*) FROM issues WHERE epic_id IN (`+in+`) GROUP BY epic_id, status`, args...)
	if err != nil {
		return nil, fmt.Errorf("count epic issues: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var epicID uuid.UUID
		var status string
		var n int
		if err := rows.Scan(&epicID, &status, &n); err != nil {
			return nil, fmt.Errorf("scan epic count: %w", err)
		}
		if out[epicID] == nil {
			out[epicID] = map[string]int{}
		}
		out[epicID][status] = n
	}
	return out, rows.Err()
}

// CountChildren is the number of issues in an epic.
func (r *EpicRepo) CountChildren(ctx context.Context, epicID uuid.UUID) (int, error) {
	var n int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM issues WHERE epic_id = $1`, epicID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count epic issues: %w", err)
	}
	return n, nil
}

// ListChildren returns the issues of an epic, from every project, oldest
// first so the list reads in the order the epic was broken down.
func (r *EpicRepo) ListChildren(ctx context.Context, epicID uuid.UUID) ([]model.EpicChild, error) {
	cols := make([]string, 0, 20)
	for _, c := range strings.Split(issueColumns, ",") {
		cols = append(cols, "i."+strings.TrimSpace(c))
	}
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT %s, p.key
		FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE i.epic_id = $1
		ORDER BY i.created_at, i.issue_key`, strings.Join(cols, ", ")), epicID)
	if err != nil {
		return nil, fmt.Errorf("list epic issues: %w", err)
	}
	defer rows.Close()
	var out []model.EpicChild
	for rows.Next() {
		var c model.EpicChild
		i := &c.Issue
		if err := rows.Scan(
			&i.ID, &i.IssueKey, &i.ProjectID, &i.Type, &i.Title, &i.Description,
			&i.Priority, &i.Status, &i.AssigneeID, &i.CreatorID, &i.Source,
			&i.Version, &i.GitURL, &i.PRURL, &i.DocURL, &i.CreatedAt, &i.UpdatedAt, &i.DuplicateOfID, &i.EpicID,
			&c.ProjectKey,
		); err != nil {
			return nil, fmt.Errorf("scan epic issue: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// EpicDependencyRow is a dependency touching at least one issue of an epic,
// with both ends summarized.
type EpicDependencyRow struct {
	Type, Severity     string
	Source, Target     model.IssueRef
	SourceIn, TargetIn bool
}

// ListDependencies returns every dependency with at least one end in the epic.
func (r *EpicRepo) ListDependencies(ctx context.Context, epicID uuid.UUID) ([]EpicDependencyRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT d.type, d.severity,
		       s.id, s.issue_key, s.title, s.status, s.priority, s.epic_id,
		       t.id, t.issue_key, t.title, t.status, t.priority, t.epic_id
		FROM issue_dependencies d
		JOIN issues s ON s.id = d.source_issue_id
		JOIN issues t ON t.id = d.target_issue_id
		WHERE s.epic_id = $1 OR t.epic_id = $1
		ORDER BY s.issue_key, t.issue_key`, epicID)
	if err != nil {
		return nil, fmt.Errorf("list epic dependencies: %w", err)
	}
	defer rows.Close()
	var out []EpicDependencyRow
	for rows.Next() {
		var d EpicDependencyRow
		var sEpic, tEpic *uuid.UUID
		if err := rows.Scan(&d.Type, &d.Severity,
			&d.Source.ID, &d.Source.IssueKey, &d.Source.Title, &d.Source.Status, &d.Source.Priority, &sEpic,
			&d.Target.ID, &d.Target.IssueKey, &d.Target.Title, &d.Target.Status, &d.Target.Priority, &tEpic,
		); err != nil {
			return nil, fmt.Errorf("scan epic dependency: %w", err)
		}
		d.SourceIn = sEpic != nil && *sEpic == epicID
		d.TargetIn = tEpic != nil && *tEpic == epicID
		out = append(out, d)
	}
	return out, rows.Err()
}

// DoneDays returns, per UTC day since from, how many of the epic's issues
// moved to done.
func (r *EpicRepo) DoneDays(ctx context.Context, epicID uuid.UUID, from time.Time) (map[string]int, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ih.created_at
		FROM issue_history ih JOIN issues i ON i.id = ih.issue_id
		WHERE i.epic_id = $1 AND ih.field_name = 'status' AND ih.new_value = 'done' AND ih.created_at >= $2`,
		epicID, from)
	if err != nil {
		return nil, fmt.Errorf("epic trend: %w", err)
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var ts time.Time
		if err := rows.Scan(&ts); err != nil {
			return nil, err
		}
		counts[ts.UTC().Format("2006-01-02")]++
	}
	return counts, rows.Err()
}

// MemoryReads returns the content hash each memory had when reader last read
// it; memories the reader never read are absent.
func (r *EpicRepo) MemoryReads(ctx context.Context, reader string, memoryIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(memoryIDs))
	if reader == "" || len(memoryIDs) == 0 {
		return out, nil
	}
	in, args := uuidIn(memoryIDs, 2)
	rows, err := r.db.Query(ctx, `SELECT memory_id, content_hash FROM memory_reads WHERE reader = $1 AND memory_id IN (`+in+`)`,
		append([]any{reader}, args...)...)
	if err != nil {
		return nil, fmt.Errorf("memory reads: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var hash string
		if err := rows.Scan(&id, &hash); err != nil {
			return nil, err
		}
		out[id] = hash
	}
	return out, rows.Err()
}

// MarkMemoryRead records that reader has read this version of a memory.
func (r *EpicRepo) MarkMemoryRead(ctx context.Context, memoryID uuid.UUID, reader, hash string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO memory_reads (memory_id, reader, content_hash, read_at) VALUES ($1, $2, $3, now())
		ON CONFLICT (memory_id, reader) DO UPDATE SET content_hash = excluded.content_hash, read_at = excluded.read_at`,
		memoryID, reader, hash)
	if err != nil {
		return fmt.Errorf("mark memory read: %w", err)
	}
	return nil
}

// MoveMemory re-points a memory at another source object and project.
func (r *EpicRepo) MoveMemory(ctx context.Context, memoryID, sourceID uuid.UUID, sourceType string, projectID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE memories SET source_object_id = $2, source_object_type = $3, project_id = $4, updated_at = now() WHERE id = $1`,
		memoryID, sourceID, sourceType, projectID)
	if err != nil {
		return fmt.Errorf("move memory: %w", err)
	}
	return nil
}

// uuidIn renders "$n, $n+1, …" for ids, starting at placeholder start.
func uuidIn(ids []uuid.UUID, start int) (string, []any) {
	ph := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		ph[i] = fmt.Sprintf("$%d", start+i)
		args[i] = id
	}
	return strings.Join(ph, ", "), args
}
