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
	"crypto/sha256"
	"encoding/hex"
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
//
// Assets carry metadata only. Their bytes move separately (see
// [PendingAssetContent]) because base64-encoding megabytes of video into this
// JSON document would make every sync pay for every attachment, every time.
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
	Assets            []model.IssueAsset      `json:"assets"`
}

// ImportResult summarizes a merge.
type ImportResult struct {
	Applied int      `json:"applied"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
	// AssetsPending lists assets whose metadata landed here but whose bytes this
	// instance does not hold yet — the peer should send them.
	AssetsPending []uuid.UUID `json:"assets_pending,omitempty"`
	// AssetsTransferred counts blobs actually moved after the metadata merge.
	AssetsTransferred int `json:"assets_transferred,omitempty"`
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
	if s.Assets, err = exportAssets(ctx, db); err != nil {
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
	// Assets last: their rows reference issues, and the upsert drops the stored
	// bytes whenever the incoming checksum differs, so the merge leaves behind an
	// explicit "needs content" marker rather than metadata describing stale bytes.
	for _, a := range s.Assets {
		apply(ctx, db, res, assetUpsert, a.ID,
			a.ID, a.IssueID, a.Filename, a.MimeType, a.SizeBytes, a.Checksum,
			ts(a.CreatedAt), ts(a.UpdatedAt))
	}

	pending, err := PendingAssetContent(ctx, db)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("list pending asset content: %v", err))
	}
	res.AssetsPending = pending

	return res, nil
}

// PendingAssetContent lists assets whose row exists but whose bytes do not —
// either freshly imported metadata, or an earlier transfer that did not finish.
// This is the work list for the blob half of a sync.
func PendingAssetContent(ctx context.Context, db database.DB) ([]uuid.UUID, error) {
	rows, err := db.Query(ctx, `SELECT id FROM issue_assets WHERE content IS NULL ORDER BY size_bytes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// AssetContent returns an asset's bytes and its recorded checksum. Content is
// nil when this instance has the metadata but not the bytes.
func AssetContent(ctx context.Context, db database.DB, id uuid.UUID) ([]byte, string, error) {
	var content []byte
	var checksum string
	err := db.QueryRow(ctx, `SELECT content, checksum FROM issue_assets WHERE id = $1`, id).Scan(&content, &checksum)
	if err != nil {
		if err == database.ErrNoRows {
			return nil, "", nil
		}
		return nil, "", err
	}
	return content, checksum, nil
}

// PutAssetContent stores transferred bytes, refusing content that does not
// match the checksum the metadata row already agreed on — a mismatch means the
// two sides disagree about what this asset is, and writing it would make the
// row lie about its own contents.
func PutAssetContent(ctx context.Context, db database.DB, id uuid.UUID, content []byte) error {
	var want string
	err := db.QueryRow(ctx, `SELECT checksum FROM issue_assets WHERE id = $1`, id).Scan(&want)
	if err != nil {
		if err == database.ErrNoRows {
			return fmt.Errorf("asset %s is not known here", id)
		}
		return err
	}
	if got := Checksum(content); got != want {
		return fmt.Errorf("asset %s content checksum %s does not match the recorded %s", id, got, want)
	}
	_, err = db.Exec(ctx, `UPDATE issue_assets SET content = $1 WHERE id = $2`, content, id)
	return err
}

// Checksum is the digest recorded on every asset; sync compares it to decide
// which blobs actually need moving.
func Checksum(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
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

// assetUpsert merges asset metadata. The content column is deliberately absent
// from the insert and cleared on any checksum change: bytes travel separately,
// and a row must never describe content it no longer holds.
const assetUpsert = `
INSERT INTO issue_assets (id, issue_id, filename, mime_type, size_bytes, checksum, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT(id) DO UPDATE SET
  issue_id=excluded.issue_id, filename=excluded.filename, mime_type=excluded.mime_type,
  size_bytes=excluded.size_bytes, checksum=excluded.checksum, updated_at=excluded.updated_at,
  content=CASE WHEN excluded.checksum = issue_assets.checksum THEN issue_assets.content ELSE NULL END
WHERE excluded.updated_at > issue_assets.updated_at`

const tagInsert = `INSERT INTO tags (id, name, color, created_at) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`
const historyInsert = `INSERT INTO issue_history (id, issue_id, field_name, old_value, new_value, operator_id, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`
const issueTagInsert = `INSERT INTO issue_tag_rel (issue_id, tag_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`
const memoryTagInsert = `INSERT INTO memory_tag_rel (memory_id, tag_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`
const depInsert = `INSERT INTO issue_dependencies (id, source_issue_id, target_issue_id, type, severity, created_at) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`
const userInsert = `INSERT INTO users (id, username, password_hash, display_name, role, created_at) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`
