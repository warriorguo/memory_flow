package synccore_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	"github.com/warriorguo/memory_flow/backend/internal/synccore"
	sqlitemigrations "github.com/warriorguo/memory_flow/backend/migrations_sqlite"
)

func newDB(t *testing.T, name string) database.DB {
	t.Helper()
	raw, err := database.OpenSQLiteDB(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateSQLite(raw, sqlitemigrations.FS); err != nil {
		t.Fatal(err)
	}
	db := database.WrapSQLite(raw)
	t.Cleanup(db.Close)
	return db
}

func TestSyncRoundTripAndLWW(t *testing.T) {
	ctx := context.Background()
	local := newDB(t, "local.db")
	server := newDB(t, "server.db")

	// Seed LOCAL with a project + issue.
	lp := repository.NewProjectRepo(local)
	li := repository.NewIssueRepo(local)
	proj, err := lp.Create(ctx, model.CreateProjectRequest{Key: "MF", Name: "Demo"})
	if err != nil {
		t.Fatal(err)
	}
	tx, _ := li.BeginTx(ctx)
	num, key, _ := lp.IncrementIssueNumber(ctx, tx, proj.ID)
	_, err = li.Create(ctx, tx, key+"-"+itoa(num), proj.ID, model.CreateIssueRequest{Type: "bug", Title: "from local"})
	if err != nil {
		t.Fatal(err)
	}
	_ = tx.Commit(ctx)

	// Seed SERVER with a different project.
	sp := repository.NewProjectRepo(server)
	if _, err := sp.Create(ctx, model.CreateProjectRequest{Key: "SRV", Name: "Server proj"}); err != nil {
		t.Fatal(err)
	}

	// PUSH: local -> server.
	snap, err := synccore.Export(ctx, local)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := synccore.Import(ctx, server, snap); err != nil {
		t.Fatal(err)
	}
	// Server now has both projects.
	if got, _ := sp.GetByKey(ctx, "MF"); got == nil {
		t.Fatalf("push: server missing MF project")
	}
	if _, total, _ := repository.NewIssueRepo(server).List(ctx, model.IssueFilter{}); total != 1 {
		t.Fatalf("push: server should have 1 issue, got %d", total)
	}

	// PULL: server -> local.
	snap2, err := synccore.Export(ctx, server)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := synccore.Import(ctx, local, snap2); err != nil {
		t.Fatal(err)
	}
	if got, _ := lp.GetByKey(ctx, "SRV"); got == nil {
		t.Fatalf("pull: local missing SRV project")
	}

	// LWW: update the MF project name on the server with a strictly newer
	// timestamp, then push server's snapshot back to local — local should adopt
	// the newer value.
	newer := time.Now().UTC().Add(1 * time.Hour)
	if _, err := server.Exec(ctx, `UPDATE projects SET name = $1, updated_at = $2 WHERE key = $3`,
		"Renamed on server", newer, "MF"); err != nil {
		t.Fatal(err)
	}
	snap3, _ := synccore.Export(ctx, server)
	res, err := synccore.Import(ctx, local, snap3)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := lp.GetByKey(ctx, "MF")
	if got == nil || got.Name != "Renamed on server" {
		t.Fatalf("LWW: expected local MF renamed, got %+v (result %+v)", got, res)
	}

	// Reverse LWW: an OLDER incoming row must NOT overwrite the local newer one.
	older := time.Now().UTC().Add(-1 * time.Hour)
	staleSnap := &synccore.Snapshot{Version: 1, Projects: []model.Project{{
		ID: got.ID, Key: got.Key, Name: "STALE", Status: got.Status,
		NextIssueNumber: got.NextIssueNumber, CreatedAt: got.CreatedAt, UpdatedAt: older,
	}}}
	if _, err := synccore.Import(ctx, local, staleSnap); err != nil {
		t.Fatal(err)
	}
	again, _ := lp.GetByKey(ctx, "MF")
	if again.Name == "STALE" {
		t.Fatalf("LWW: stale older row should not overwrite newer local row")
	}
}

// TestSyncIdempotentAndFormatConsistency guards the timestamp-format fix: a
// second identical import must be a no-op (no spurious updates), API-created and
// sync-imported rows must store the same timestamp format, and a stale (older)
// imported row must not overwrite a newer locally-edited row.
func TestSyncIdempotentAndFormatConsistency(t *testing.T) {
	ctx := context.Background()
	a := newDB(t, "a.db")
	b := newDB(t, "b.db")

	ap := repository.NewProjectRepo(a)
	proj, err := ap.Create(ctx, model.CreateProjectRequest{Key: "MF", Name: "orig"})
	if err != nil {
		t.Fatal(err)
	}

	// Push a -> b twice; the second push must apply nothing (idempotent).
	snap, _ := synccore.Export(ctx, a)
	if _, err := synccore.Import(ctx, b, snap); err != nil {
		t.Fatal(err)
	}
	snap, _ = synccore.Export(ctx, a)
	res, err := synccore.Import(ctx, b, snap)
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied != 0 {
		t.Fatalf("second identical import should apply 0 rows, applied=%d", res.Applied)
	}

	// b now edits the project (newer updated_at via now()).
	if _, err := b.Exec(ctx, `UPDATE projects SET name='edited on b', updated_at=now() WHERE key='MF'`); err != nil {
		t.Fatal(err)
	}

	// a re-pushes its ORIGINAL (older) row — must NOT overwrite b's newer edit.
	snap, _ = synccore.Export(ctx, a)
	if _, err := synccore.Import(ctx, b, snap); err != nil {
		t.Fatal(err)
	}
	bp := repository.NewProjectRepo(b)
	got, _ := bp.GetByKey(ctx, "MF")
	if got.Name != "edited on b" {
		t.Fatalf("stale older row overwrote newer edit: got name=%q", got.Name)
	}
	_ = proj
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
