package synccore_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	"github.com/warriorguo/memory_flow/backend/internal/synccore"
)

// seedIssue creates a project and an issue to hang assets on, returning the
// issue id.
func seedIssue(t *testing.T, db database.DB, projectKey string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	proj, err := repository.NewProjectRepo(db).Create(ctx, model.CreateProjectRequest{
		Key: projectKey, Name: projectKey + " project",
	})
	if err != nil {
		t.Fatal(err)
	}

	issueRepo := repository.NewIssueRepo(db)
	tx, err := issueRepo.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	issue, err := issueRepo.Create(ctx, tx, projectKey+"-1", proj.ID, model.CreateIssueRequest{
		Type: "requirement", Title: "Has assets",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return issue.ID
}

// transfer moves the bytes for every pending asset from src to dst, standing in
// for the HTTP round trip syncclient performs.
func transfer(t *testing.T, src, dst database.DB, pending []uuid.UUID) int {
	t.Helper()
	ctx := context.Background()

	moved := 0
	for _, id := range pending {
		content, _, err := synccore.AssetContent(ctx, src, id)
		if err != nil {
			t.Fatalf("read asset %s: %v", id, err)
		}
		if content == nil {
			continue
		}
		if err := synccore.PutAssetContent(ctx, dst, id, content); err != nil {
			t.Fatalf("store asset %s: %v", id, err)
		}
		moved++
	}
	return moved
}

// A snapshot must carry asset metadata but never asset bytes: base64 video in
// the JSON would make every sync pay for every attachment.
func TestSnapshotCarriesAssetMetadataNotBytes(t *testing.T) {
	ctx := context.Background()
	local := newDB(t, "local.db")
	issueID := seedIssue(t, local, "OZX")

	content := []byte("pretend this is a 20MB clip")
	assets := repository.NewAssetRepo(local, repository.NewDBContentStore())
	if _, err := assets.Create(ctx, issueID, model.PutAssetRequest{
		Filename: "clip.mp4", MimeType: "video/mp4", Content: content,
	}); err != nil {
		t.Fatal(err)
	}

	snap, err := synccore.Export(ctx, local)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Assets) != 1 {
		t.Fatalf("snapshot has %d assets, want 1", len(snap.Assets))
	}
	got := snap.Assets[0]
	if got.Filename != "clip.mp4" || got.SizeBytes != int64(len(content)) {
		t.Errorf("asset metadata = %+v", got)
	}
	if got.Checksum != synccore.Checksum(content) {
		t.Errorf("checksum = %q, want %q", got.Checksum, synccore.Checksum(content))
	}
}

// The metadata merge lands first and reports which blobs the receiver still
// needs; the bytes follow.
func TestAssetSyncMovesMetadataThenBytes(t *testing.T) {
	ctx := context.Background()
	local := newDB(t, "local.db")
	server := newDB(t, "server.db")

	issueID := seedIssue(t, local, "OZX")
	content := []byte{0x89, 'P', 'N', 'G', 0x00, 0xff}
	assets := repository.NewAssetRepo(local, repository.NewDBContentStore())
	created, err := assets.Create(ctx, issueID, model.PutAssetRequest{
		Filename: "enemy_ref.png", MimeType: "image/png", Content: content,
	})
	if err != nil {
		t.Fatal(err)
	}

	snap, err := synccore.Export(ctx, local)
	if err != nil {
		t.Fatal(err)
	}
	res, err := synccore.Import(ctx, server, snap)
	if err != nil {
		t.Fatal(err)
	}

	// Metadata is there, bytes are not — and the result says so.
	if len(res.AssetsPending) != 1 || res.AssetsPending[0] != created.ID {
		t.Fatalf("assets_pending = %v, want [%v]", res.AssetsPending, created.ID)
	}
	serverAssets := repository.NewAssetRepo(server, repository.NewDBContentStore())
	meta, err := serverAssets.GetByFilename(ctx, issueID, "enemy_ref.png")
	if err != nil || meta == nil {
		t.Fatalf("server missing asset metadata: %v", err)
	}
	if got, _ := serverAssets.GetContent(ctx, created.ID); got != nil {
		t.Errorf("server already has content before the transfer: %v", got)
	}

	// Transfer the bytes, and the server can serve the file.
	if moved := transfer(t, local, server, res.AssetsPending); moved != 1 {
		t.Fatalf("transferred %d assets, want 1", moved)
	}
	got, err := serverAssets.GetContent(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("transferred content = %v, want %v", got, content)
	}

	// Nothing is pending once the bytes have landed, so a repeat sync moves no
	// bytes at all.
	pending, err := synccore.PendingAssetContent(ctx, server)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("still pending after transfer: %v", pending)
	}

	snap2, _ := synccore.Export(ctx, local)
	res2, err := synccore.Import(ctx, server, snap2)
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.AssetsPending) != 0 {
		t.Errorf("unchanged asset re-requested: %v", res2.AssetsPending)
	}
}

// Replacing a file on one side must invalidate the other side's stale bytes,
// or the receiver keeps serving old artwork under a checksum that says
// otherwise.
func TestAssetSyncReplacementInvalidatesStaleBytes(t *testing.T) {
	ctx := context.Background()
	local := newDB(t, "local.db")
	server := newDB(t, "server.db")

	issueID := seedIssue(t, local, "OZX")
	localAssets := repository.NewAssetRepo(local, repository.NewDBContentStore())
	created, err := localAssets.Create(ctx, issueID, model.PutAssetRequest{
		Filename: "enemy_ref.png", MimeType: "image/png", Content: []byte("draft"),
	})
	if err != nil {
		t.Fatal(err)
	}

	snap, _ := synccore.Export(ctx, local)
	res, _ := synccore.Import(ctx, server, snap)
	transfer(t, local, server, res.AssetsPending)

	// Art revises the file under the same name, with a strictly newer timestamp
	// so last-writer-wins picks it.
	revised := []byte("final artwork")
	if _, err := localAssets.Replace(ctx, issueID, "enemy_ref.png", model.PutAssetRequest{
		Filename: "enemy_ref.png", MimeType: "image/png", Content: revised,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := local.Exec(ctx, `UPDATE issue_assets SET updated_at = '2030-01-01 00:00:00' WHERE id = $1`, created.ID); err != nil {
		t.Fatal(err)
	}

	snap2, _ := synccore.Export(ctx, local)
	res2, err := synccore.Import(ctx, server, snap2)
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.AssetsPending) != 1 {
		t.Fatalf("replacement did not mark the asset pending: %v", res2.AssetsPending)
	}

	serverAssets := repository.NewAssetRepo(server, repository.NewDBContentStore())
	if got, _ := serverAssets.GetContent(ctx, created.ID); got != nil {
		t.Errorf("stale bytes survived the metadata update: %q", got)
	}

	transfer(t, local, server, res2.AssetsPending)
	got, _ := serverAssets.GetContent(ctx, created.ID)
	if !bytes.Equal(got, revised) {
		t.Errorf("content after re-transfer = %q, want %q", got, revised)
	}
}

// Bytes that do not hash to the checksum the metadata agreed on are refused —
// storing them would make the row lie about its own contents.
func TestPutAssetContentRejectsChecksumMismatch(t *testing.T) {
	ctx := context.Background()
	db := newDB(t, "local.db")
	issueID := seedIssue(t, db, "OZX")

	assets := repository.NewAssetRepo(db, repository.NewDBContentStore())
	created, err := assets.Create(ctx, issueID, model.PutAssetRequest{
		Filename: "a.txt", MimeType: "text/plain", Content: []byte("right"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := synccore.PutAssetContent(ctx, db, created.ID, []byte("wrong")); err == nil {
		t.Error("mismatched content was accepted")
	}
	if err := synccore.PutAssetContent(ctx, db, uuid.New(), []byte("x")); err == nil {
		t.Error("content for an unknown asset was accepted")
	}
}

// An asset whose issue never made it across must be skipped, not silently
// attached to nothing.
func TestAssetSyncSkipsAssetWithoutItsIssue(t *testing.T) {
	ctx := context.Background()
	server := newDB(t, "server.db")

	orphan := &synccore.Snapshot{Version: 1, Assets: []model.IssueAsset{{
		ID: uuid.New(), IssueID: uuid.New(), Filename: "orphan.png",
		MimeType: "image/png", SizeBytes: 3, Checksum: synccore.Checksum([]byte("abc")),
	}}}

	res, err := synccore.Import(ctx, server, orphan)
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied != 0 || res.Skipped != 1 {
		t.Errorf("applied=%d skipped=%d, want 0/1", res.Applied, res.Skipped)
	}
}
