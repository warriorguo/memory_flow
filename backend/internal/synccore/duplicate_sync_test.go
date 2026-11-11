package synccore_test

import (
	"context"
	"testing"

	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	"github.com/warriorguo/memory_flow/backend/internal/service"
	"github.com/warriorguo/memory_flow/backend/internal/synccore"
)

// MF-36: duplicate_of_id survives a sync even when the duplicate comes before
// the issue it points at in the snapshot (a foreign key the upsert alone
// would trip over).
func TestSyncCarriesDuplicateOf(t *testing.T) {
	ctx := context.Background()
	local := newDB(t, "local.db")
	server := newDB(t, "server.db")

	projects := repository.NewProjectRepo(local)
	issues := repository.NewIssueRepo(local)
	svc := service.NewIssueService(issues, projects, repository.NewIssueHistoryRepo(local))
	proj, err := projects.Create(ctx, model.CreateProjectRequest{Key: "MF", Name: "Demo"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := svc.Create(ctx, proj.ID, model.CreateIssueRequest{Type: "bug", Title: "original"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Create(ctx, proj.ID, model.CreateIssueRequest{Type: "bug", Title: "filed again"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.MarkDuplicate(ctx, b.ID, a.ID, nil); err != nil {
		t.Fatal(err)
	}

	snap, err := synccore.Export(ctx, local)
	if err != nil {
		t.Fatal(err)
	}
	// Put the duplicate first.
	for i, j := 0, len(snap.Issues)-1; i < j; i, j = i+1, j-1 {
		snap.Issues[i], snap.Issues[j] = snap.Issues[j], snap.Issues[i]
	}
	if snap.Issues[0].ID != b.ID {
		t.Fatalf("test setup: expected the duplicate first")
	}
	res, err := synccore.Import(ctx, server, snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("import errors: %v", res.Errors)
	}
	got, err := repository.NewIssueRepo(server).GetByID(ctx, b.ID)
	if err != nil || got == nil {
		t.Fatalf("server missing duplicate: %v", err)
	}
	if got.Status != model.StatusDuplicate || got.DuplicateOfID == nil || *got.DuplicateOfID != a.ID {
		t.Fatalf("server lost the duplicate link: status=%s dup=%v", got.Status, got.DuplicateOfID)
	}
}
