package database_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	sqlitemigrations "github.com/warriorguo/memory_flow/backend/migrations_sqlite"
)

// TestSQLiteMigrationsAndRepos runs the embedded SQLite migrations and exercises
// the shared repositories end-to-end against SQLite, validating the dialect
// rewrite (placeholders, ILIKE, now(), window functions, RETURNING, recursive
// CTE) and the portable GetTrend.
func TestSQLiteMigrationsAndRepos(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mf.db")

	rawDB, err := database.OpenSQLiteDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateSQLite(rawDB, sqlitemigrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db := database.WrapSQLite(rawDB)
	defer db.Close()
	ctx := context.Background()

	projectRepo := repository.NewProjectRepo(db)
	issueRepo := repository.NewIssueRepo(db)
	historyRepo := repository.NewIssueHistoryRepo(db)
	memoryRepo := repository.NewMemoryRepo(db)
	tagRepo := repository.NewTagRepo(db)
	depRepo := repository.NewDependencyRepo(db)

	// Project create + get by key.
	name := "Demo"
	proj, err := projectRepo.Create(ctx, model.CreateProjectRequest{Key: "MF", Name: name})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if got, _ := projectRepo.GetByKey(ctx, "MF"); got == nil || got.ID != proj.ID {
		t.Fatalf("get by key mismatch")
	}

	// Issue create inside a tx (mirrors IssueService.Create).
	tx, err := issueRepo.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	num, key, err := projectRepo.IncrementIssueNumber(ctx, tx, proj.ID)
	if err != nil {
		t.Fatalf("increment: %v", err)
	}
	issueKey := key + "-" + itoa(num)
	bug := "bug"
	iss, err := issueRepo.Create(ctx, tx, issueKey, proj.ID, model.CreateIssueRequest{Type: bug, Title: "boom"})
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// Keyword search exercises the reused-placeholder ILIKE -> LIKE rewrite.
	kw := "boom"
	issues, total, err := issueRepo.List(ctx, model.IssueFilter{Keyword: &kw})
	if err != nil {
		t.Fatalf("list issues: %v", err)
	}
	if total != 1 || len(issues) != 1 {
		t.Fatalf("keyword search got total=%d len=%d", total, len(issues))
	}

	// Status counts (GROUP BY) + trend (Go bucketing).
	counts, err := issueRepo.CountByStatus(ctx, proj.ID)
	if err != nil || counts["todo"] != 1 {
		t.Fatalf("count by status: %v counts=%v", err, counts)
	}
	if _, err := issueRepo.GetTrend(ctx, proj.ID, 7); err != nil {
		t.Fatalf("trend: %v", err)
	}

	// History insert via tx.
	tx2, _ := issueRepo.BeginTx(ctx)
	if err := historyRepo.Create(ctx, tx2, iss.ID, "status", strp("todo"), strp("done"), nil); err != nil {
		t.Fatalf("history create: %v", err)
	}
	_ = tx2.Commit(ctx)

	// Memory create + keyword list (window function COUNT(*) OVER()).
	mem, err := memoryRepo.Create(ctx, model.CreateMemoryRequest{Type: "recall", Title: "note", Content: "remember boom", ProjectID: &proj.ID})
	if err != nil {
		t.Fatalf("create memory: %v", err)
	}
	mkw := "boom"
	_, mtotal, err := memoryRepo.List(ctx, model.MemoryFilter{Keyword: &mkw})
	if err != nil || mtotal != 1 {
		t.Fatalf("memory list: %v total=%d", err, mtotal)
	}

	// Tag + relation (ON CONFLICT DO NOTHING).
	tag, err := tagRepo.Create(ctx, model.CreateTagRequest{Name: "urgent"})
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if err := tagRepo.AddToMemory(ctx, mem.ID, tag.ID); err != nil {
		t.Fatalf("add tag: %v", err)
	}
	if err := tagRepo.AddToMemory(ctx, mem.ID, tag.ID); err != nil {
		t.Fatalf("add tag again (on conflict): %v", err)
	}

	// Second issue + dependency + recursive CTE HasPath.
	tx3, _ := issueRepo.BeginTx(ctx)
	num2, key2, _ := projectRepo.IncrementIssueNumber(ctx, tx3, proj.ID)
	iss2, err := issueRepo.Create(ctx, tx3, key2+"-"+itoa(num2), proj.ID, model.CreateIssueRequest{Type: bug, Title: "second"})
	if err != nil {
		t.Fatal(err)
	}
	_ = tx3.Commit(ctx)
	if _, err := depRepo.Create(ctx, model.IssueDependency{SourceIssueID: iss.ID, TargetIssueID: iss2.ID, Type: "depends_on", Severity: "critical"}); err != nil {
		t.Fatalf("create dep: %v", err)
	}
	hasPath, err := depRepo.HasPath(ctx, iss.ID, iss2.ID)
	if err != nil || !hasPath {
		t.Fatalf("has path: %v got=%v", err, hasPath)
	}
}

// MF-17: MaxIssueNumber reads the highest issue-key suffix, and Update persists
// a bumped next_issue_number through a real SQLite DB.
func TestMaxIssueNumberAndBump(t *testing.T) {
	dir := t.TempDir()
	rawDB, err := database.OpenSQLiteDB(filepath.Join(dir, "mf.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateSQLite(rawDB, sqlitemigrations.FS); err != nil {
		t.Fatal(err)
	}
	db := database.WrapSQLite(rawDB)
	defer db.Close()
	ctx := context.Background()

	projectRepo := repository.NewProjectRepo(db)
	issueRepo := repository.NewIssueRepo(db)

	proj, err := projectRepo.Create(ctx, model.CreateProjectRequest{Key: "MF", Name: "Demo"})
	if err != nil {
		t.Fatal(err)
	}

	// No issues yet -> max 0.
	if n, err := projectRepo.MaxIssueNumber(ctx, proj.ID); err != nil || n != 0 {
		t.Fatalf("empty MaxIssueNumber got %d err %v", n, err)
	}

	// Create three issues (MF-1, MF-2, MF-3).
	for i := 0; i < 3; i++ {
		tx, _ := issueRepo.BeginTx(ctx)
		num, key, err := projectRepo.IncrementIssueNumber(ctx, tx, proj.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := issueRepo.Create(ctx, tx, key+"-"+itoa(num), proj.ID, model.CreateIssueRequest{Type: "bug", Title: "x"}); err != nil {
			t.Fatal(err)
		}
		_ = tx.Commit(ctx)
	}
	if n, _ := projectRepo.MaxIssueNumber(ctx, proj.ID); n != 3 {
		t.Fatalf("MaxIssueNumber got %d want 3", n)
	}

	// Bump the counter to 460 and confirm it persists.
	want := 460
	updated, err := projectRepo.Update(ctx, proj.ID, model.UpdateProjectRequest{NextIssueNumber: &want})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.NextIssueNumber != 460 {
		t.Fatalf("persisted next_issue_number got %d want 460", updated.NextIssueNumber)
	}
	// Re-read to be sure.
	again, _ := projectRepo.GetByID(ctx, proj.ID)
	if again.NextIssueNumber != 460 {
		t.Fatalf("reloaded next_issue_number got %d want 460", again.NextIssueNumber)
	}

	// The next created issue should be MF-461.
	tx, _ := issueRepo.BeginTx(ctx)
	num, key, _ := projectRepo.IncrementIssueNumber(ctx, tx, proj.ID)
	_ = tx.Commit(ctx)
	if got := key + "-" + itoa(num); got != "MF-461" {
		t.Fatalf("next issue key got %s want MF-461", got)
	}
}

func strp(s string) *string { return &s }
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
