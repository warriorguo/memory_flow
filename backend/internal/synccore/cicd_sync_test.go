package synccore_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	"github.com/warriorguo/memory_flow/backend/internal/synccore"
)

// CI/CD links travel with their project, and a snapshot from a peer that
// predates cicd_links (only cicd_url set) still lands as one unlabeled link.
func TestSyncCarriesCICDLinks(t *testing.T) {
	ctx := context.Background()
	local := newDB(t, "local.db")
	server := newDB(t, "server.db")

	links := []model.CICDLink{
		{Label: "server", URL: "https://cicd/apps/36"},
		{Label: "admin", URL: "https://cicd/apps/38"},
	}
	primary := links[0].URL
	proj, err := repository.NewProjectRepo(local).Create(ctx, model.CreateProjectRequest{
		Key: "APM", Name: "APM", CICDURL: &primary, CICDLinks: links})
	if err != nil {
		t.Fatal(err)
	}

	snap, err := synccore.Export(ctx, local)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := synccore.Import(ctx, server, snap); err != nil {
		t.Fatal(err)
	}
	sp := repository.NewProjectRepo(server)
	got, _ := sp.GetByKey(ctx, "APM")
	if got == nil || !reflect.DeepEqual(got.CICDLinks, links) {
		t.Fatalf("server cicd_links = %+v, want %+v", got, links)
	}

	legacyURL := "https://cicd/apps/99"
	old := &synccore.Snapshot{Version: 1, Projects: []model.Project{{
		ID: proj.ID, Key: "APM", Name: "APM", Status: "active", CICDURL: &legacyURL,
		CreatedAt: proj.CreatedAt, UpdatedAt: time.Now().UTC().Add(time.Hour),
	}}}
	if _, err := synccore.Import(ctx, server, old); err != nil {
		t.Fatal(err)
	}
	got, _ = sp.GetByKey(ctx, "APM")
	want := []model.CICDLink{{URL: legacyURL}}
	if !reflect.DeepEqual(got.CICDLinks, want) {
		t.Fatalf("after legacy snapshot cicd_links = %+v, want %+v", got.CICDLinks, want)
	}
}
