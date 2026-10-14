package repository

import (
	"context"
	"strconv"
	"testing"

	"github.com/warriorguo/memory_flow/backend/internal/model"
)

// The list endpoint must say which issue sits at each end, on either side of
// the relation and across projects, so the issue page can render "depends on
// ORT-50" without a fetch per dependency.
func TestDependencyListSummarizesBothEnds(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	projects := NewProjectRepo(db)
	issues := NewIssueRepo(db)
	deps := NewDependencyRepo(db)

	newIssue := func(projectKey, title, priority string) *model.Issue {
		t.Helper()
		p, err := projects.GetByKey(ctx, projectKey)
		if err != nil {
			t.Fatal(err)
		}
		if p == nil {
			if p, err = projects.Create(ctx, model.CreateProjectRequest{Key: projectKey, Name: projectKey}); err != nil {
				t.Fatal(err)
			}
		}
		tx, err := issues.BeginTx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		num, key, err := projects.IncrementIssueNumber(ctx, tx, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		iss, err := issues.Create(ctx, tx, key+"-"+strconv.Itoa(num), p.ID,
			model.CreateIssueRequest{Type: "bug", Title: title, Priority: &priority})
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		return iss
	}

	ozx := newIssue("OZX", "Room loads wrong template", "P1")
	ort := newIssue("ORT", "Export template layers", "P2")
	other := newIssue("OZX", "Boss arena", "P2")

	// OZX-1 depends on ORT-1 (OZX-1 is the source); OZX-2 blocks OZX-1 (OZX-1 is the target).
	if _, err := deps.Create(ctx, model.IssueDependency{SourceIssueID: ozx.ID, TargetIssueID: ort.ID,
		Type: "depends_on", Severity: "critical"}); err != nil {
		t.Fatal(err)
	}
	if _, err := deps.Create(ctx, model.IssueDependency{SourceIssueID: other.ID, TargetIssueID: ozx.ID,
		Type: "blocks", Severity: "recommended"}); err != nil {
		t.Fatal(err)
	}

	got, err := deps.ListByIssueID(ctx, ozx.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d dependencies, want 2", len(got))
	}
	for _, d := range got {
		if d.SourceIssue == nil || d.TargetIssue == nil {
			t.Fatalf("dependency %s is missing an end summary", d.ID)
		}
		if d.SourceIssue.ID != d.SourceIssueID || d.TargetIssue.ID != d.TargetIssueID {
			t.Errorf("summary ids do not match the dependency ends: %+v", d)
		}
	}

	// Both rows can share a created_at second, so find each by type.
	byType := map[string]model.IssueDependency{}
	for _, d := range got {
		byType[d.Type] = d
	}
	want := model.IssueRef{ID: ort.ID, IssueKey: "ORT-1", Title: "Export template layers", Status: "todo", Priority: "P2"}
	if got := byType["depends_on"].TargetIssue; got == nil || *got != want {
		t.Errorf("cross-project target = %+v, want %+v", got, want)
	}
	rev := byType["blocks"]
	if rev.SourceIssue.IssueKey != "OZX-2" || rev.TargetIssue.IssueKey != "OZX-1" {
		t.Errorf("reverse dependency ends = %s -> %s, want OZX-2 -> OZX-1",
			rev.SourceIssue.IssueKey, rev.TargetIssue.IssueKey)
	}
}
