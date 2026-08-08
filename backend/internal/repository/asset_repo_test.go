package repository

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

// newAssetRepo returns an asset repo plus the id of an issue to hang assets on.
func newAssetRepo(t *testing.T) (*AssetRepo, database.DB, uuid.UUID) {
	t.Helper()
	db := newTestDB(t)
	return NewAssetRepo(db, NewDBContentStore()), db, seedIssue(t, db, "OZX-1")
}

func seedIssue(t *testing.T, db database.DB, issueKey string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	projectID := uuid.New()
	_, err := db.Exec(ctx, `INSERT INTO projects (id, key, name) VALUES ($1, $2, $3)`,
		projectID, "P"+issueKey, "Test Project "+issueKey)
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}

	issueID := uuid.New()
	_, err = db.Exec(ctx, `INSERT INTO issues (id, issue_key, project_id, type, title) VALUES ($1, $2, $3, $4, $5)`,
		issueID, issueKey, projectID, "requirement", "Test issue")
	if err != nil {
		t.Fatalf("seed issue: %v", err)
	}
	return issueID
}

func TestAssetCreateAndRead(t *testing.T) {
	r, _, issueID := newAssetRepo(t)
	ctx := context.Background()

	// Binary content with a NUL byte and a high byte, so a store that quietly
	// round-trips through a string would be caught.
	content := []byte{0x89, 'P', 'N', 'G', 0x00, 0x1a, 0xff}
	asset, err := r.Create(ctx, issueID, model.PutAssetRequest{
		Filename: "reference-art.png",
		MimeType: "image/png",
		Content:  content,
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if asset.SizeBytes != int64(len(content)) {
		t.Errorf("size = %d, want %d", asset.SizeBytes, len(content))
	}
	if asset.Checksum != Checksum(content) {
		t.Errorf("checksum = %q, want %q", asset.Checksum, Checksum(content))
	}

	got, err := r.GetContent(ctx, asset.ID)
	if err != nil {
		t.Fatalf("get content: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content = %v, want %v", got, content)
	}

	found, err := r.GetByFilename(ctx, issueID, "reference-art.png")
	if err != nil || found == nil {
		t.Fatalf("get by filename: %v (asset %v)", err, found)
	}
	if found.ID != asset.ID {
		t.Errorf("id = %v, want %v", found.ID, asset.ID)
	}

	missing, err := r.GetByFilename(ctx, issueID, "nope.png")
	if err != nil {
		t.Fatalf("get missing: %v", err)
	}
	if missing != nil {
		t.Errorf("missing filename returned %v, want nil", missing)
	}
}

// A second upload under the same filename must be refused rather than silently
// overwriting — replacing is a separate, explicit call.
func TestAssetCreateRejectsDuplicateFilename(t *testing.T) {
	r, db, issueID := newAssetRepo(t)
	ctx := context.Background()

	req := model.PutAssetRequest{Filename: "spec.md", MimeType: "text/markdown", Content: []byte("v1")}
	if _, err := r.Create(ctx, issueID, req); err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if _, err := r.Create(ctx, issueID, req); !errors.Is(err, ErrAssetExists) {
		t.Errorf("duplicate create err = %v, want ErrAssetExists", err)
	}

	// The same filename on a different issue is fine — uniqueness is per issue.
	other := seedIssue(t, db, "OZX-2")
	if _, err := r.Create(ctx, other, req); err != nil {
		t.Errorf("same filename on another issue: %v", err)
	}
}

// Replace keeps the asset id — descriptions reference assets by filename, and
// any link already handed out must keep resolving.
func TestAssetReplaceKeepsIDAndUpdatesContent(t *testing.T) {
	r, _, issueID := newAssetRepo(t)
	ctx := context.Background()

	original, err := r.Create(ctx, issueID, model.PutAssetRequest{
		Filename: "enemy.png", MimeType: "image/png", Content: []byte("draft"),
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}

	revised := []byte("final artwork bytes")
	updated, err := r.Replace(ctx, issueID, "enemy.png", model.PutAssetRequest{
		Filename: "enemy.png", MimeType: "image/png", Content: revised,
	})
	if err != nil {
		t.Fatalf("replace asset: %v", err)
	}
	if updated.ID != original.ID {
		t.Errorf("replace changed id: %v → %v", original.ID, updated.ID)
	}
	if updated.Checksum == original.Checksum {
		t.Error("checksum unchanged after replacing the content")
	}
	if updated.SizeBytes != int64(len(revised)) {
		t.Errorf("size = %d, want %d", updated.SizeBytes, len(revised))
	}

	got, err := r.GetContent(ctx, updated.ID)
	if err != nil {
		t.Fatalf("get content: %v", err)
	}
	if !bytes.Equal(got, revised) {
		t.Errorf("content = %q, want %q", got, revised)
	}

	if _, err := r.Replace(ctx, issueID, "absent.png", model.PutAssetRequest{Content: []byte("x")}); !errors.Is(err, ErrAssetNotFound) {
		t.Errorf("replace missing err = %v, want ErrAssetNotFound", err)
	}
}

func TestAssetListOmitsContentAndOrders(t *testing.T) {
	r, db, issueID := newAssetRepo(t)
	ctx := context.Background()

	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if _, err := r.Create(ctx, issueID, model.PutAssetRequest{
			Filename: name, MimeType: "text/plain", Content: []byte(name),
		}); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}

	assets, err := r.ListByIssueID(ctx, issueID)
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	if len(assets) != 3 {
		t.Fatalf("listed %d assets, want 3", len(assets))
	}
	for i, want := range []string{"a.txt", "b.txt", "c.txt"} {
		if assets[i].Filename != want {
			t.Errorf("assets[%d] = %q, want %q", i, assets[i].Filename, want)
		}
	}

	// The list query must not select the blob column at all. Proving that from
	// the outside is awkward, so assert the shape the API depends on: metadata
	// is populated and content is only ever reachable through GetContent.
	if assets[0].Checksum == "" || assets[0].SizeBytes == 0 {
		t.Error("list returned incomplete metadata")
	}
	content, err := r.GetContent(ctx, assets[0].ID)
	if err != nil || string(content) != "a.txt" {
		t.Errorf("GetContent = %q, %v", content, err)
	}

	empty := seedIssue(t, db, "OZX-EMPTY")
	if assets, err := r.ListByIssueID(ctx, empty); err != nil || len(assets) != 0 {
		t.Errorf("empty issue list = %v, %v", assets, err)
	}
}

func TestAssetDelete(t *testing.T) {
	r, _, issueID := newAssetRepo(t)
	ctx := context.Background()

	if _, err := r.Create(ctx, issueID, model.PutAssetRequest{
		Filename: "crash.log", MimeType: "text/plain", Content: []byte("stack trace"),
	}); err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if err := r.Delete(ctx, issueID, "crash.log"); err != nil {
		t.Fatalf("delete asset: %v", err)
	}
	if got, err := r.GetByFilename(ctx, issueID, "crash.log"); err != nil || got != nil {
		t.Errorf("asset still present after delete: %v, %v", got, err)
	}
	if err := r.Delete(ctx, issueID, "crash.log"); !errors.Is(err, ErrAssetNotFound) {
		t.Errorf("delete missing err = %v, want ErrAssetNotFound", err)
	}
}

// Deleting an issue must take its assets with it, or the blobs leak.
func TestAssetCascadesWithIssue(t *testing.T) {
	r, db, issueID := newAssetRepo(t)
	ctx := context.Background()

	if _, err := r.Create(ctx, issueID, model.PutAssetRequest{
		Filename: "clip.mp4", MimeType: "video/mp4", Content: []byte("frames"),
	}); err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM issues WHERE id = $1`, issueID); err != nil {
		t.Fatalf("delete issue: %v", err)
	}

	var count int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM issue_assets WHERE issue_id = $1`, issueID).Scan(&count); err != nil {
		t.Fatalf("count assets: %v", err)
	}
	if count != 0 {
		t.Errorf("%d assets survived their issue, want 0", count)
	}
}
