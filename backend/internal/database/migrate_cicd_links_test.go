package database_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	sqlitemigrate "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	sqlitemigrations "github.com/warriorguo/memory_flow/backend/migrations_sqlite"
)

// Migration 11 turns each existing cicd_url into one unlabeled cicd link, and
// leaves projects without a URL with no links.
func TestMigrationBackfillsCICDLinks(t *testing.T) {
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
	if err := m.Migrate(10); err != nil {
		t.Fatalf("migrate to 10: %v", err)
	}

	for _, row := range [][]any{
		{"11111111-1111-1111-1111-111111111111", "WITH", "https://cicd/apps/36"},
		{"22222222-2222-2222-2222-222222222222", "NONE", nil},
		{"33333333-3333-3333-3333-333333333333", "BLANK", ""},
	} {
		if _, err := raw.Exec(`INSERT INTO projects (id, key, name, cicd_url) VALUES (?, ?, ?, ?)`,
			row[0], row[1], row[1], row[2]); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Migrate(11); err != nil {
		t.Fatalf("migrate to 11: %v", err)
	}

	var stored *string
	if err := raw.QueryRow(`SELECT cicd_links FROM projects WHERE key = 'WITH'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == nil || *stored != `[{"label":"","url":"https://cicd/apps/36"}]` {
		t.Errorf("backfilled cicd_links = %v", deref(stored))
	}

	repo := repository.NewProjectRepo(database.WrapSQLite(raw))
	ctx := context.Background()
	want := map[string][]model.CICDLink{
		"WITH":  {{URL: "https://cicd/apps/36"}},
		"NONE":  {},
		"BLANK": {},
	}
	for key, links := range want {
		p, err := repo.GetByKey(ctx, key)
		if err != nil || p == nil {
			t.Fatalf("get %s: %v", key, err)
		}
		if !reflect.DeepEqual(p.CICDLinks, links) {
			t.Errorf("%s cicd_links = %+v, want %+v", key, p.CICDLinks, links)
		}
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
