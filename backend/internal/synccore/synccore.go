// Package synccore implements engine-neutral export/import of the full Memory
// Flow dataset so a standalone (SQLite) instance and the PostgreSQL server can
// exchange data. Conflicts on mutable rows are resolved last-writer-wins by
// updated_at; append-only rows are inserted if absent.
//
// Import applies rows one at a time (no enclosing transaction) so a single bad
// row — e.g. an issue_key collision when both sides minted the same key
// offline — is skipped and reported rather than aborting the whole merge.
package synccore

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

// SnapshotVersion identifies the snapshot schema for forward compatibility.
const SnapshotVersion = 1

// SyncUser carries a user row including the password hash (model.User hides it
// from JSON, but sync must transfer it).
type SyncUser struct {
	ID           uuid.UUID `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	DisplayName  *string   `json:"display_name"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// TagRel is a row of one of the tag join tables.
type TagRel struct {
	LeftID uuid.UUID `json:"left_id"` // issue_id or memory_id
	TagID  uuid.UUID `json:"tag_id"`
}

// Snapshot is the complete, portable dataset.
type Snapshot struct {
	Version           int                     `json:"version"`
	Projects          []model.Project         `json:"projects"`
	Tags              []model.Tag             `json:"tags"`
	Issues            []model.Issue           `json:"issues"`
	IssueHistory      []model.IssueHistory    `json:"issue_history"`
	Memories          []model.Memory          `json:"memories"`
	IssueTagRel       []TagRel                `json:"issue_tag_rel"`
	MemoryTagRel      []TagRel                `json:"memory_tag_rel"`
	IssueDependencies []model.IssueDependency `json:"issue_dependencies"`
	Users             []SyncUser              `json:"users"`
}

// ImportResult summarizes a merge.
type ImportResult struct {
	Applied int      `json:"applied"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

// Export reads every table into a Snapshot.
func Export(ctx context.Context, db database.DB) (*Snapshot, error) {
	s := &Snapshot{Version: SnapshotVersion}
	var err error

	if s.Projects, err = exportProjects(ctx, db); err != nil {
		return nil, err
	}
	if s.Tags, err = exportTags(ctx, db); err != nil {
		return nil, err
	}
	if s.Issues, err = exportIssues(ctx, db); err != nil {
		return nil, err
	}
	if s.IssueHistory, err = exportIssueHistory(ctx, db); err != nil {
		return nil, err
	}
	if s.Memories, err = exportMemories(ctx, db); err != nil {
		return nil, err
	}
	if s.IssueTagRel, err = exportTagRel(ctx, db, "issue_tag_rel", "issue_id"); err != nil {
		return nil, err
	}
	if s.MemoryTagRel, err = exportTagRel(ctx, db, "memory_tag_rel", "memory_id"); err != nil {
		return nil, err
	}
	if s.IssueDependencies, err = exportDependencies(ctx, db); err != nil {
		return nil, err
	}
	if s.Users, err = exportUsers(ctx, db); err != nil {
		return nil, err
	}
	return s, nil
}

// Import merges a Snapshot into db using last-writer-wins for mutable rows.
func Import(ctx context.Context, db database.DB, s *Snapshot) (*ImportResult, error) {
	res := &ImportResult{}
	ts := timeBinder(db.Dialect())

	// Parents before children to satisfy foreign keys.
	for _, p := range s.Projects {
		apply(ctx, db, res, projectUpsert, p.ID,
			p.ID, p.Key, p.Name, p.Summary, p.Description, p.DesignPrinciples,
			p.GitURL, p.CICDURL, p.DocURL, p.OwnerID, p.Status, p.NextIssueNumber,
			ts(p.CreatedAt), ts(p.UpdatedAt))
	}
	for _, t := range s.Tags {
		apply(ctx, db, res, tagInsert, t.ID, t.ID, t.Name, t.Color, ts(t.CreatedAt))
	}
	for _, i := range s.Issues {
		apply(ctx, db, res, issueUpsert, i.ID,
			i.ID, i.IssueKey, i.ProjectID, i.Type, i.Title, i.Description, i.Priority,
			i.Status, i.AssigneeID, i.CreatorID, i.Source, i.Version, i.GitURL,
			i.PRURL, i.DocURL, ts(i.CreatedAt), ts(i.UpdatedAt))
	}
	for _, h := range s.IssueHistory {
		apply(ctx, db, res, historyInsert, h.ID,
			h.ID, h.IssueID, h.FieldName, h.OldValue, h.NewValue, h.OperatorID, ts(h.CreatedAt))
	}
	for _, m := range s.Memories {
		apply(ctx, db, res, memoryUpsert, m.ID,
			m.ID, m.ProjectID, m.Type, m.Title, m.Content, m.SourceObjectType,
			m.SourceObjectID, m.CreatorID, ts(m.CreatedAt), ts(m.UpdatedAt))
	}
	for _, r := range s.IssueTagRel {
		apply(ctx, db, res, issueTagInsert, r.LeftID, r.LeftID, r.TagID)
	}
	for _, r := range s.MemoryTagRel {
		apply(ctx, db, res, memoryTagInsert, r.LeftID, r.LeftID, r.TagID)
	}
	for _, d := range s.IssueDependencies {
		apply(ctx, db, res, depInsert, d.ID,
			d.ID, d.SourceIssueID, d.TargetIssueID, d.Type, d.Severity, ts(d.CreatedAt))
	}
	for _, u := range s.Users {
		apply(ctx, db, res, userInsert, u.ID,
			u.ID, u.Username, u.PasswordHash, u.DisplayName, u.Role, ts(u.CreatedAt))
	}
	return res, nil
}

// timeBinder returns a function that converts a time.Time into the value to
// bind for the target engine. SQLite stores timestamps as TEXT and its
// CURRENT_TIMESTAMP default produces UTC "YYYY-MM-DD HH:MM:SS"; we bind imported
// timestamps in that exact format so all rows share one lexically-ordered,
// chronologically-correct representation (keeping ORDER BY and the
// last-writer-wins comparison correct). PostgreSQL uses native timestamptz, so
// we bind the time.Time directly.
func timeBinder(dialect string) func(time.Time) any {
	if dialect == database.DialectSQLite {
		return func(t time.Time) any { return t.UTC().Format("2006-01-02 15:04:05") }
	}
	return func(t time.Time) any { return t }
}

// apply runs one upsert and records the outcome.
func apply(ctx context.Context, db database.DB, res *ImportResult, query string, key any, args ...any) {
	r, err := db.Exec(ctx, query, args...)
	if err != nil {
		res.Skipped++
		res.Errors = append(res.Errors, fmt.Sprintf("%v: %v", key, err))
		return
	}
	if r.RowsAffected() == 0 {
		// Existing row was newer (LWW skipped) or already present.
		res.Skipped++
		return
	}
	res.Applied++
}

// --- upsert statements (portable across PostgreSQL and SQLite) ---

const projectUpsert = `
INSERT INTO projects (id, key, name, summary, description, design_principles, git_url, cicd_url, doc_url, owner_id, status, next_issue_number, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
ON CONFLICT(id) DO UPDATE SET
  key=excluded.key, name=excluded.name, summary=excluded.summary, description=excluded.description,
  design_principles=excluded.design_principles, git_url=excluded.git_url, cicd_url=excluded.cicd_url,
  doc_url=excluded.doc_url, owner_id=excluded.owner_id, status=excluded.status,
  next_issue_number=excluded.next_issue_number, updated_at=excluded.updated_at
WHERE excluded.updated_at > projects.updated_at`

const issueUpsert = `
INSERT INTO issues (id, issue_key, project_id, type, title, description, priority, status, assignee_id, creator_id, source, version, git_url, pr_url, doc_url, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
ON CONFLICT(id) DO UPDATE SET
  issue_key=excluded.issue_key, project_id=excluded.project_id, type=excluded.type, title=excluded.title,
  description=excluded.description, priority=excluded.priority, status=excluded.status,
  assignee_id=excluded.assignee_id, creator_id=excluded.creator_id, source=excluded.source,
  version=excluded.version, git_url=excluded.git_url, pr_url=excluded.pr_url, doc_url=excluded.doc_url,
  updated_at=excluded.updated_at
WHERE excluded.updated_at > issues.updated_at`

const memoryUpsert = `
INSERT INTO memories (id, project_id, type, title, content, source_object_type, source_object_id, creator_id, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
ON CONFLICT(id) DO UPDATE SET
  project_id=excluded.project_id, type=excluded.type, title=excluded.title, content=excluded.content,
  source_object_type=excluded.source_object_type, source_object_id=excluded.source_object_id,
  creator_id=excluded.creator_id, updated_at=excluded.updated_at
WHERE excluded.updated_at > memories.updated_at`

const tagInsert = `INSERT INTO tags (id, name, color, created_at) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`
const historyInsert = `INSERT INTO issue_history (id, issue_id, field_name, old_value, new_value, operator_id, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`
const issueTagInsert = `INSERT INTO issue_tag_rel (issue_id, tag_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`
const memoryTagInsert = `INSERT INTO memory_tag_rel (memory_id, tag_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`
const depInsert = `INSERT INTO issue_dependencies (id, source_issue_id, target_issue_id, type, severity, created_at) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`
const userInsert = `INSERT INTO users (id, username, password_hash, display_name, role, created_at) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`
