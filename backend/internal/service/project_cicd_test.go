package service

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	sqlitemigrations "github.com/warriorguo/memory_flow/backend/migrations_sqlite"
)

// newCICDTestService runs ProjectService over a real SQLite repo: the point of
// these tests is that cicd_links and the legacy cicd_url stay one value through
// the round trip to the database, which a mock cannot show.
func newCICDTestService(t *testing.T) (*ProjectService, database.DB) {
	t.Helper()
	raw, err := database.OpenSQLiteDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.MigrateSQLite(raw, sqlitemigrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db := database.WrapSQLite(raw)
	t.Cleanup(db.Close)
	return NewProjectService(repository.NewProjectRepo(db)), db
}

func assertCICD(t *testing.T, p *model.Project, wantURL string, want []model.CICDLink) {
	t.Helper()
	if got := deref(p.CICDURL); got != wantURL {
		t.Errorf("cicd_url = %q, want %q", got, wantURL)
	}
	if !reflect.DeepEqual(p.CICDLinks, want) {
		t.Errorf("cicd_links = %+v, want %+v", p.CICDLinks, want)
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func TestProjectCICDLinksRoundTrip(t *testing.T) {
	svc, _ := newCICDTestService(t)
	ctx := context.Background()

	server := model.CICDLink{Label: "server", URL: "https://cicd/apps/36"}
	admin := model.CICDLink{Label: "admin", URL: "https://cicd/apps/38"}

	p, err := svc.Create(ctx, model.CreateProjectRequest{Key: "APM", Name: "APM",
		CICDLinks: []model.CICDLink{{Label: " server ", URL: " https://cicd/apps/36 "}, admin}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Trimmed, order kept, and cicd_url mirrors the first link.
	assertCICD(t, p, server.URL, []model.CICDLink{server, admin})

	got, err := svc.GetByKey(ctx, "APM")
	if err != nil {
		t.Fatal(err)
	}
	assertCICD(t, got, server.URL, []model.CICDLink{server, admin})

	// Replacing the list moves cicd_url to the new first link.
	links := []model.CICDLink{admin}
	p, err = svc.Update(ctx, p.ID, model.UpdateProjectRequest{CICDLinks: &links})
	if err != nil {
		t.Fatalf("update links: %v", err)
	}
	assertCICD(t, p, admin.URL, []model.CICDLink{admin})

	// Clearing the list clears cicd_url too, so the legacy fallback does not
	// bring the old URL back on the next read.
	empty := []model.CICDLink{}
	if _, err := svc.Update(ctx, p.ID, model.UpdateProjectRequest{CICDLinks: &empty}); err != nil {
		t.Fatalf("clear links: %v", err)
	}
	got, _ = svc.GetByID(ctx, p.ID)
	assertCICD(t, got, "", []model.CICDLink{})
}

// A client that only knows cicd_url keeps working: what it writes becomes a
// single unlabeled link.
func TestProjectLegacyCICDURL(t *testing.T) {
	svc, _ := newCICDTestService(t)
	ctx := context.Background()

	url := "https://cicd/apps/1"
	p, err := svc.Create(ctx, model.CreateProjectRequest{Key: "OLD", Name: "Old", CICDURL: &url})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	assertCICD(t, p, url, []model.CICDLink{{URL: url}})

	next := "https://cicd/apps/2"
	p, err = svc.Update(ctx, p.ID, model.UpdateProjectRequest{CICDURL: &next})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	assertCICD(t, p, next, []model.CICDLink{{URL: next}})

	// cicd_links wins when a client sends both.
	links := []model.CICDLink{{Label: "web", URL: "https://cicd/apps/3"}}
	p, err = svc.Update(ctx, p.ID, model.UpdateProjectRequest{CICDURL: &next, CICDLinks: &links})
	if err != nil {
		t.Fatalf("update both: %v", err)
	}
	assertCICD(t, p, "https://cicd/apps/3", links)

	// No CI/CD at all reads back as an empty list, never null.
	bare, err := svc.Create(ctx, model.CreateProjectRequest{Key: "BARE", Name: "Bare"})
	if err != nil {
		t.Fatalf("create bare: %v", err)
	}
	assertCICD(t, bare, "", []model.CICDLink{})
}

// A row whose cicd_links is NULL — written before the column existed, or
// synced in from a peer that predates it — reads its link from cicd_url.
func TestProjectCICDLinksFallBackToLegacyColumn(t *testing.T) {
	svc, db := newCICDTestService(t)
	ctx := context.Background()

	p, err := svc.Create(ctx, model.CreateProjectRequest{Key: "LEG", Name: "Legacy"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE projects SET cicd_url = $1, cicd_links = NULL WHERE id = $2`,
		"https://cicd/apps/9", p.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.GetByID(ctx, p.ID)
	assertCICD(t, got, "https://cicd/apps/9", []model.CICDLink{{URL: "https://cicd/apps/9"}})
}

func TestProjectCICDLinksValidation(t *testing.T) {
	svc, _ := newCICDTestService(t)
	ctx := context.Background()

	cases := map[string][]model.CICDLink{
		"missing url":     {{Label: "server", URL: "  "}},
		"duplicate label": {{Label: "a", URL: "https://x/1"}, {Label: "a", URL: "https://x/2"}},
	}
	for name, links := range cases {
		if _, err := svc.Create(ctx, model.CreateProjectRequest{Key: "BAD", Name: "Bad", CICDLinks: links}); err == nil {
			t.Errorf("%s: create succeeded, want an error", name)
		}
	}

	// Several unlabeled links are fine; only labels must be unique.
	p, err := svc.Create(ctx, model.CreateProjectRequest{Key: "OK", Name: "Ok",
		CICDLinks: []model.CICDLink{{URL: "https://x/1"}, {URL: "https://x/2"}}})
	if err != nil {
		t.Fatalf("unlabeled links: %v", err)
	}
	if len(p.CICDLinks) != 2 {
		t.Errorf("cicd_links = %+v, want two links", p.CICDLinks)
	}
}
