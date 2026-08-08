package repository

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	sqlitemigrations "github.com/warriorguo/memory_flow/backend/migrations_sqlite"
)

// newTestDB returns a throwaway SQLite database with every migration applied.
func newTestDB(t *testing.T) database.DB {
	t.Helper()

	raw, err := database.OpenSQLiteDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.MigrateSQLite(raw, sqlitemigrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db := database.WrapSQLite(raw)
	t.Cleanup(func() { db.Close() })
	return db
}

func newProjectRepo(t *testing.T) *ProjectRepo {
	t.Helper()
	return NewProjectRepo(newTestDB(t))
}

// seed creates a project and stamps its timestamps, so ordering assertions do
// not depend on how fast the test runs — SQLite's CURRENT_TIMESTAMP only has
// second resolution, and everything here is created within the same second.
func seedProjectAt(t *testing.T, r *ProjectRepo, key, created, updated string) {
	t.Helper()
	ctx := context.Background()

	p, err := r.Create(ctx, model.CreateProjectRequest{Key: key, Name: key + " Project"})
	if err != nil {
		t.Fatalf("create %s: %v", key, err)
	}
	_, err = r.db.Exec(ctx, `UPDATE projects SET created_at = $1, updated_at = $2 WHERE id = $3`,
		created, updated, p.ID)
	if err != nil {
		t.Fatalf("stamp %s: %v", key, err)
	}
}

func listKeys(t *testing.T, r *ProjectRepo, filter model.ProjectFilter) []string {
	t.Helper()
	projects, _, err := r.List(context.Background(), filter)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	keys := make([]string, len(projects))
	for i, p := range projects {
		keys[i] = p.Key
	}
	return keys
}

func assertKeys(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("project order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("project order = %v, want %v", got, want)
		}
	}
}

// The list is ordered by update time, newest first — the project touched most
// recently leads, regardless of when it was created.
func TestProjectListOrdersByUpdatedAtDesc(t *testing.T) {
	r := newProjectRepo(t)

	seedProjectAt(t, r, "OLD", "2026-01-01 00:00:00", "2026-03-01 00:00:00")
	seedProjectAt(t, r, "NEW", "2026-02-01 00:00:00", "2026-02-01 00:00:00")
	seedProjectAt(t, r, "STALE", "2026-01-15 00:00:00", "2026-01-15 00:00:00")

	assertKeys(t, listKeys(t, r, model.ProjectFilter{}), []string{"OLD", "NEW", "STALE"})

	// Touching the oldest project floats it to the top.
	stale, err := r.GetByKey(context.Background(), "STALE")
	if err != nil {
		t.Fatalf("get STALE: %v", err)
	}
	name := "Renamed"
	if _, err := r.Update(context.Background(), stale.ID, model.UpdateProjectRequest{Name: &name}); err != nil {
		t.Fatalf("update STALE: %v", err)
	}

	assertKeys(t, listKeys(t, r, model.ProjectFilter{}), []string{"STALE", "OLD", "NEW"})
}

// Paging must not repeat or skip rows: the ordering has to be total, which is
// what the created_at and id tie-breakers are for.
func TestProjectListPaginationIsStableAcrossTiedTimestamps(t *testing.T) {
	r := newProjectRepo(t)

	// Same updated_at throughout, so the tie-breakers decide the order.
	for _, key := range []string{"A", "B", "C", "D"} {
		seedProjectAt(t, r, key, "2026-01-01 00:00:00", "2026-02-01 00:00:00")
	}

	all := listKeys(t, r, model.ProjectFilter{})
	if len(all) != 4 {
		t.Fatalf("listed %d projects, want 4", len(all))
	}

	var paged []string
	for page := 1; page <= 2; page++ {
		paged = append(paged, listKeys(t, r, model.ProjectFilter{Page: page, PageSize: 2})...)
	}
	assertKeys(t, paged, all)
}
