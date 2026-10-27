package database_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	sqlitemigrate "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	sqlitemigrations "github.com/warriorguo/memory_flow/backend/migrations_sqlite"
)

// Migration 12 gives every color-less tag the palette color model.PaletteIndex
// picks, and leaves chosen colors alone.
func TestMigrationBackfillsTagColors(t *testing.T) {
	raw, err := database.OpenSQLiteDB(filepath.Join(t.TempDir(), "mf.db"))
	if err != nil {
		t.Fatal(err)
	}
	src, err := iofs.New(sqlitemigrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	driver, err := sqlitemigrate.WithInstance(raw, &sqlitemigrate.Config{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "sqlite", driver)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Migrate(11); err != nil {
		t.Fatalf("migrate to 11: %v", err)
	}

	for _, row := range [][]any{
		{uuid.NewString(), "room", nil},
		{uuid.NewString(), "enemy-ai", ""},
		{uuid.NewString(), "延后", nil},
		{uuid.NewString(), "kept", "#123456"},
	} {
		if _, err := raw.Exec(`INSERT INTO tags (id, name, color) VALUES (?, ?, ?)`, row...); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Migrate(12); err != nil {
		t.Fatalf("migrate to 12: %v", err)
	}

	want := map[string]string{
		"room":     model.TagPalette[model.PaletteIndex("room")],
		"enemy-ai": model.TagPalette[model.PaletteIndex("enemy-ai")],
		"延后":       model.TagPalette[model.PaletteIndex("延后")],
		"kept":     "#123456",
	}
	for name, color := range want {
		var got *string
		if err := raw.QueryRow(`SELECT color FROM tags WHERE name = ?`, name).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if deref(got) != color {
			t.Errorf("%s color = %s, want %s", name, deref(got), color)
		}
	}
}

// The tag repository paths MF-34 added, against real SQLite: auto color, name
// lookup ignoring case, rename/recolor, batch load, and AND filtering.
func TestTagRepoAndIssueFilterSQLite(t *testing.T) {
	raw, err := database.OpenSQLiteDB(filepath.Join(t.TempDir(), "mf.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateSQLite(raw, sqlitemigrations.FS); err != nil {
		t.Fatal(err)
	}
	db := database.WrapSQLite(raw)
	defer db.Close()
	ctx := context.Background()

	tags := repository.NewTagRepo(db)
	issues := repository.NewIssueRepo(db)
	projects := repository.NewProjectRepo(db)

	ui, err := tags.Create(ctx, model.CreateTagRequest{Name: "ui"})
	if err != nil {
		t.Fatal(err)
	}
	if ui.Color == nil || *ui.Color == "" {
		t.Fatalf("auto color missing: %+v", ui)
	}
	backend, err := tags.Create(ctx, model.CreateTagRequest{Name: "backend"})
	if err != nil {
		t.Fatal(err)
	}
	if *backend.Color == *ui.Color {
		t.Errorf("second tag reused the only used color %s", *ui.Color)
	}
	if _, err := tags.Create(ctx, model.CreateTagRequest{Name: "bad", Color: strPtr("blue")}); err == nil {
		t.Error("invalid color accepted")
	}

	if got, err := tags.GetByName(ctx, "UI"); err != nil || got == nil || got.ID != ui.ID {
		t.Fatalf("GetByName(UI) = %+v, %v", got, err)
	}
	if got, _ := tags.GetByName(ctx, "nope"); got != nil {
		t.Fatalf("GetByName(nope) = %+v", got)
	}

	renamed, err := tags.Update(ctx, ui.ID, model.UpdateTagRequest{Name: strPtr("frontend"), Color: strPtr("ABC")})
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Name != "frontend" || deref(renamed.Color) != "#aabbcc" {
		t.Errorf("update = %+v", renamed)
	}
	if _, err := tags.Update(ctx, ui.ID, model.UpdateTagRequest{Name: strPtr("Backend")}); err != repository.ErrTagNameTaken {
		t.Errorf("rename onto existing name: err = %v", err)
	}

	proj, err := projects.Create(ctx, model.CreateProjectRequest{Key: "MF", Name: "MF"})
	if err != nil {
		t.Fatal(err)
	}
	mk := func(key string) uuid.UUID {
		tx, err := issues.BeginTx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		iss, err := issues.Create(ctx, tx, key, proj.ID, model.CreateIssueRequest{Type: "bug", Title: key})
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		return iss.ID
	}
	both, onlyUI, none := mk("MF-1"), mk("MF-2"), mk("MF-3")
	for _, rel := range [][2]uuid.UUID{{both, ui.ID}, {both, backend.ID}, {onlyUI, ui.ID}} {
		if err := tags.AddToIssue(ctx, rel[0], rel[1]); err != nil {
			t.Fatal(err)
		}
	}

	byIssue, err := tags.GetByIssueIDs(ctx, []uuid.UUID{both, onlyUI, none})
	if err != nil {
		t.Fatal(err)
	}
	if len(byIssue[both]) != 2 || len(byIssue[onlyUI]) != 1 || len(byIssue[none]) != 0 {
		t.Errorf("GetByIssueIDs = %+v", byIssue)
	}

	count := func(ids ...uuid.UUID) int {
		got, total, err := issues.List(ctx, model.IssueFilter{ProjectID: &proj.ID, TagIDs: ids})
		if err != nil {
			t.Fatal(err)
		}
		if total != len(got) {
			t.Errorf("total %d != len %d", total, len(got))
		}
		return len(got)
	}
	if n := count(); n != 3 {
		t.Errorf("no tag filter: %d issues", n)
	}
	if n := count(ui.ID); n != 2 {
		t.Errorf("tag=frontend: %d issues, want 2", n)
	}
	if n := count(ui.ID, backend.ID); n != 1 {
		t.Errorf("tag=frontend AND backend: %d issues, want 1", n)
	}
}

func strPtr(s string) *string { return &s }
