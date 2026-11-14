package synccore_test

import (
	"context"
	"testing"

	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	"github.com/warriorguo/memory_flow/backend/internal/service"
	"github.com/warriorguo/memory_flow/backend/internal/synccore"
)

// MF-39: epic membership and memory read state survive a sync, with the
// child placed before its epic in the snapshot.
func TestSyncCarriesEpics(t *testing.T) {
	ctx := context.Background()
	local := newDB(t, "local.db")
	server := newDB(t, "server.db")

	projects := repository.NewProjectRepo(local)
	issues := repository.NewIssueRepo(local)
	history := repository.NewIssueHistoryRepo(local)
	memories := repository.NewMemoryRepo(local)
	svc := service.NewIssueService(issues, projects, history)
	epics := service.NewEpicService(issues, history, memories, repository.NewEpicRepo(local))

	proj, err := projects.Create(ctx, model.CreateProjectRequest{Key: "OZX", Name: "Game"})
	if err != nil {
		t.Fatal(err)
	}
	epic, err := svc.Create(ctx, proj.ID, model.CreateIssueRequest{Type: model.IssueTypeEpic, Title: "endless"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := svc.Create(ctx, proj.ID, model.CreateIssueRequest{Type: "requirement", Title: "waves"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := epics.SetEpic(ctx, child.ID, &epic.ID, nil); err != nil {
		t.Fatal(err)
	}
	src := model.IssueTypeEpic
	mem, err := memories.Create(ctx, model.CreateMemoryRequest{ProjectID: &proj.ID, Type: "recall", Title: "death rule", Content: "down until next wave", SourceObjectType: &src, SourceObjectID: &epic.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := epics.MarkMemoriesRead(ctx, model.MarkMemoriesReadRequest{Reader: "bob", Memories: []model.MemoryReadMark{{MemoryID: mem.ID}}}); err != nil {
		t.Fatal(err)
	}

	snap, err := synccore.Export(ctx, local)
	if err != nil {
		t.Fatal(err)
	}
	for i, j := 0, len(snap.Issues)-1; i < j; i, j = i+1, j-1 {
		snap.Issues[i], snap.Issues[j] = snap.Issues[j], snap.Issues[i]
	}
	if snap.Issues[0].ID != child.ID {
		t.Fatalf("test setup: expected the child first")
	}
	res, err := synccore.Import(ctx, server, snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("import errors: %v", res.Errors)
	}
	got, err := repository.NewIssueRepo(server).GetByID(ctx, child.ID)
	if err != nil || got == nil || got.EpicID == nil || *got.EpicID != epic.ID {
		t.Fatalf("server lost the epic link: %+v %v", got, err)
	}
	sIssues := repository.NewIssueRepo(server)
	sHistory := repository.NewIssueHistoryRepo(server)
	sEpics := service.NewEpicService(sIssues, sHistory, repository.NewMemoryRepo(server), repository.NewEpicRepo(server))
	seen, err := sEpics.IssueMemories(ctx, child.ID, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0].Scope != model.MemoryScopeEpic || seen[0].Unread || seen[0].Updated {
		t.Fatalf("bob's read state did not sync: %+v", seen)
	}
}
