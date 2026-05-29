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
