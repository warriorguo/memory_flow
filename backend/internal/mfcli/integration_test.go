package mfcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/handler"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	"github.com/warriorguo/memory_flow/backend/internal/service"
	sqlitemigrations "github.com/warriorguo/memory_flow/backend/migrations_sqlite"
)

// This file exercises every mf command against the real API stack — the same
// router the standalone binary serves — backed by a throwaway SQLite database.
// The stub-based tests in cli_test.go pin specific request shapes; these pin
// end-to-end behavior, including the multi-call workflows (attach-git, done,
// dependencies, tagging) where the CLI is doing the real work.

// mfTest is a live server plus the helpers to drive the CLI against it.
type mfTest struct {
	t   *testing.T
	url string
}

func newMFTest(t *testing.T) *mfTest {
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

	projectRepo := repository.NewProjectRepo(db)
	issueRepo := repository.NewIssueRepo(db)
	historyRepo := repository.NewIssueHistoryRepo(db)
	memoryRepo := repository.NewMemoryRepo(db)
	tagRepo := repository.NewTagRepo(db)
	depRepo := repository.NewDependencyRepo(db)
	assetRepo := repository.NewAssetRepo(db, repository.NewDBContentStore())

	projectSvc := service.NewProjectService(projectRepo)
	issueSvc := service.NewIssueService(issueRepo, projectRepo, historyRepo)
	progressSvc := service.NewProgressService(issueRepo)
	memorySvc := service.NewMemoryService(memoryRepo)
	depSvc := service.NewDependencyService(depRepo, issueRepo, projectRepo)
	assetSvc := service.NewAssetService(assetRepo, 0)
	resolver := handler.NewIDResolver(projectSvc, issueSvc)

	router := handler.NewRouter(
		handler.NewProjectHandler(projectSvc, resolver),
		handler.NewIssueHandler(issueSvc, tagRepo, resolver),
		handler.NewProgressHandler(progressSvc, resolver),
		handler.NewMemoryHandler(memorySvc),
		handler.NewTagHandler(tagRepo, resolver),
		handler.NewDependencyHandler(depSvc, resolver),
		handler.NewSyncHandler(db, ""),
		handler.NewAssetHandler(assetSvc, issueSvc, resolver),
	)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return &mfTest{t: t, url: srv.URL}
}

// run executes a command and requires it to succeed, returning stdout.
func (m *mfTest) run(args ...string) string {
	m.t.Helper()
	stdout, stderr, code := runCLI(m.t, m.url, args...)
	if code != 0 {
		m.t.Fatalf("mf %s failed (exit %d): %s", strings.Join(args, " "), code, stderr)
	}
	return stdout
}

// fails executes a command that must fail, returning stderr.
func (m *mfTest) fails(args ...string) string {
	m.t.Helper()
	stdout, stderr, code := runCLI(m.t, m.url, args...)
	if code == 0 {
		m.t.Fatalf("mf %s should have failed, got:\n%s", strings.Join(args, " "), stdout)
	}
	return stderr
}

// json runs a command with --json and returns the decoded `data` field.
func (m *mfTest) json(args ...string) map[string]any {
	m.t.Helper()
	out := m.run(append(args, "--json")...)
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		m.t.Fatalf("mf %s --json: %v\n%s", strings.Join(args, " "), err, out)
	}
	return env.Data
}

func (m *mfTest) contains(out string, want ...string) {
	m.t.Helper()
	for _, w := range want {
		if !strings.Contains(out, w) {
			m.t.Errorf("output missing %q:\n%s", w, out)
		}
	}
}

func (m *mfTest) omits(out string, unwanted ...string) {
	m.t.Helper()
	for _, u := range unwanted {
		if strings.Contains(out, u) {
			m.t.Errorf("output should not contain %q:\n%s", u, out)
		}
	}
}

// seedProject creates a project with a git_url, the common starting point.
func (m *mfTest) seedProject(key, name string) {
	m.t.Helper()
	m.run("project", "create", key, "--name", name,
		"--summary", "Test project for "+key,
		"--git-url", "git@github.com:warriorguo/"+strings.ToLower(key)+".git",
		"--owner", "andrew")
}

// withStdin swaps os.Stdin for the duration of fn, so --desc-file - and
// --content-file - can be exercised.
func withStdin(t *testing.T, content string, fn func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdin")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	saved := os.Stdin
	os.Stdin = f
	defer func() { os.Stdin = saved }()
	fn()
}

// --- projects --------------------------------------------------------------

func TestIntegrationProjectLifecycle(t *testing.T) {
	m := newMFTest(t)

	// Create, both the positional-key and --key spellings.
	m.contains(m.run("project", "create", "ALPHA", "--name", "Alpha Project",
		"--summary", "First", "--git-url", "https://github.com/x/alpha.git"), "created project ALPHA")
	m.contains(m.run("project", "create", "--key", "BETA", "--name", "Beta Project"), "created project BETA")

	// List, and filter by name and status.
	out := m.run("projects")
	m.contains(out, "ALPHA", "Alpha Project", "BETA", "active")
	m.contains(m.run("projects", "--name", "ALPHA"), "ALPHA")
	m.omits(m.run("projects", "--status", "archived"), "ALPHA")

	// Show renders the recorded metadata.
	m.contains(m.run("project", "show", "ALPHA"),
		"ALPHA", "Alpha Project", "https://github.com/x/alpha.git", "ALPHA-1", "First")

	// Update individual fields; unset flags must leave other fields alone.
	m.run("project", "update", "ALPHA", "--name", "Alpha Renamed")
	shown := m.run("project", "show", "ALPHA")
	m.contains(shown, "Alpha Renamed", "https://github.com/x/alpha.git")

	m.run("project", "update", "ALPHA", "--status", "paused")
	m.contains(m.run("project", "show", "ALPHA"), "paused")
	m.run("project", "update", "ALPHA", "--status", "active")

	// A description arriving on stdin.
	withStdin(t, "Line one\nLine two\n", func() {
		m.run("project", "update", "ALPHA", "--desc-file", "-")
	})
	m.contains(m.run("project", "show", "ALPHA"), "Line one", "Line two")

	// next_issue_number is the last number handed out, so `show` must report
	// the key the next issue will actually get.
	m.run("project", "update", "ALPHA", "--next-issue-number", "50")
	m.contains(m.run("project", "show", "ALPHA"), "ALPHA-51")
	created := m.json("issue", "create", "ALPHA", "--type", "bug", "--title", "After the bump")
	if key, _ := created["key"].(string); key != "ALPHA-51" {
		t.Errorf("issue key = %q, want ALPHA-51 after bumping the counter to 50", key)
	}

	// Archive.
	m.contains(m.run("project", "archive", "BETA"), "archived project BETA")
	m.contains(m.run("projects", "--status", "archived"), "BETA")
}

func TestIntegrationProjectErrors(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("ALPHA", "Alpha")

	m.contains(m.fails("project", "show", "NOPE"), "not found")
	m.contains(m.fails("project", "create", "--name", "No Key"), "--key and --name are required")
	m.contains(m.fails("project", "create", "GAMMA"), "--key and --name are required")
	m.contains(m.fails("project", "show"), "usage:")
	m.contains(m.fails("project", "show", "ALPHA", "extra"), "unexpected argument")
	m.contains(m.fails("project", "bogus", "ALPHA"), "unknown `mf project` subcommand")
	// The counter may not move backwards.
	m.fails("project", "update", "ALPHA", "--next-issue-number", "0")
}

func TestIntegrationProgress(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("ALPHA", "Alpha")

	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "Bug one", "--priority", "P0")
	m.run("issue", "create", "ALPHA", "--type", "requirement", "--title", "Req one", "--priority", "P1")
	m.run("issue", "create", "ALPHA", "--type", "requirement", "--title", "Req two")
	m.run("issue", "start", "ALPHA-1")

	out := m.run("project", "progress", "ALPHA")
	m.contains(out, "3 issue(s) total", "2 todo", "1 in_progress", "1 P0", "1 P1", "1 P2", "1 bug", "2 requirement")

	// The trend endpoint has its own SQL date handling; make sure it works.
	m.contains(m.run("project", "progress", "ALPHA", "--trend", "7"), "Trend (last 7 days)", "DATE", "CREATED", "DONE")
}

// --- issues ----------------------------------------------------------------

func TestIntegrationIssueLifecycle(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("ALPHA", "Alpha")

	// Create with every optional field populated.
	out := m.run("issue", "create", "ALPHA",
		"--type", "bug", "--title", "Everything set", "--desc", "A description",
		"--priority", "P0", "--assignee", "andrew", "--source", "user report",
		"--version", "1.2.3", "--doc-url", "https://docs.example/x")
	m.contains(out, "created ALPHA-1", "[bug]", "P0")

	shown := m.run("issue", "show", "ALPHA-1")
	m.contains(shown, "ALPHA-1", "Everything set", "bug", "todo", "P0", "andrew",
		"1.2.3", "user report", "https://docs.example/x", "A description")

	// A description arriving on stdin.
	withStdin(t, "Steps:\n1. do a thing\n2. observe\n", func() {
		m.run("issue", "create", "ALPHA", "--type", "requirement", "--title", "From stdin", "--desc-file", "-")
	})
	m.contains(m.run("issue", "show", "ALPHA-2"), "1. do a thing", "2. observe")

	// Update, field by field.
	m.run("issue", "update", "ALPHA-1", "--title", "Retitled", "--priority", "P2", "--assignee", "someone")
	m.contains(m.run("issue", "show", "ALPHA-1"), "Retitled", "P2", "someone")

	// History records each change.
	history := m.run("issue", "history", "ALPHA-1")
	m.contains(history, "title", "priority", "assignee_id", "Retitled")

	// Defaults: priority P2, no assignee.
	created := m.json("issue", "create", "ALPHA", "--type", "bug", "--title", "Defaults")
	if created["priority"] != "P2" {
		t.Errorf("default priority = %v, want P2", created["priority"])
	}
}

func TestIntegrationIssueList(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("ALPHA", "Alpha")

	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "Open bug", "--priority", "P0", "--assignee", "andrew")
	m.run("issue", "create", "ALPHA", "--type", "requirement", "--title", "Open req", "--priority", "P2")
	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "Finished work")
	m.run("issue", "done", "ALPHA-3", "--git", "https://github.com/x/y/commit/abc")

	// Open-only by default.
	out := m.run("issues", "ALPHA")
	m.contains(out, "ALPHA-1", "ALPHA-2", "2 open of 3 total")
	m.omits(out, "ALPHA-3")

	// --all includes the closed one.
	m.contains(m.run("issues", "ALPHA", "--all"), "ALPHA-3")

	// An explicit --status disables the open-only filter.
	statusOut := m.run("issues", "ALPHA", "--status", "done")
	m.contains(statusOut, "ALPHA-3")
	m.omits(statusOut, "ALPHA-1")

	// Field filters.
	m.contains(m.run("issues", "ALPHA", "--type", "bug"), "ALPHA-1")
	m.omits(m.run("issues", "ALPHA", "--type", "requirement"), "ALPHA-1")
	m.contains(m.run("issues", "ALPHA", "--priority", "P0"), "ALPHA-1")
	m.contains(m.run("issues", "ALPHA", "--assignee", "andrew"), "ALPHA-1")
	m.contains(m.run("issues", "ALPHA", "--keyword", "Open req"), "ALPHA-2")

	// --limit trims and says so.
	m.contains(m.run("issues", "ALPHA", "--limit", "1"), "--limit")

	// An empty result reports the total rather than looking broken.
	m.contains(m.run("issues", "ALPHA", "--assignee", "nobody"), "no issues match")

	// `mf issue list` is the same command.
	m.contains(m.run("issue", "list", "ALPHA"), "ALPHA-1")
}

func TestIntegrationStatusTransitions(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("ALPHA", "Alpha")
	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "Walk me")

	// Single legal hop.
	m.contains(m.run("issue", "start", "ALPHA-1"), "todo → in_progress")

	// Multi-hop: in_progress -> review -> testing, then testing -> done.
	m.contains(m.run("issue", "status", "ALPHA-1", "review"), "in_progress → review")
	m.contains(m.run("issue", "status", "ALPHA-1", "done"), "review → testing → done")
	m.contains(m.run("issue", "show", "ALPHA-1"), "done")

	// Already in the target status is a no-op, not an error.
	m.contains(m.run("issue", "status", "ALPHA-1", "done"), "already done")

	// --direct refuses to walk.
	m.run("issue", "status", "ALPHA-1", "in_progress")
	m.run("issue", "status", "ALPHA-1", "todo")
	m.contains(m.fails("issue", "status", "ALPHA-1", "done", "--direct"), "not a legal transition")

	// A target that is unreachable from here.
	m.contains(m.fails("issue", "status", "ALPHA-1", "nonsense"), "no path")

	// The full todo -> done walk, which is the case `done` depends on.
	m.contains(m.run("issue", "status", "ALPHA-1", "done"), "todo → in_progress → done")
}

func TestIntegrationAttachGitAndDone(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("ALPHA", "Alpha") // git_url: git@github.com:warriorguo/alpha.git

	// A bare sha becomes a commit URL built from the project's git_url, with
	// the ssh remote normalized.
	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "Sha form")
	m.contains(m.run("issue", "attach-git", "ALPHA-1", "5253083"),
		"https://github.com/warriorguo/alpha/commit/5253083")

	// A full URL is recorded verbatim.
	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "URL form")
	m.contains(m.run("issue", "attach-git", "ALPHA-2", "https://example.com/anything"),
		"https://example.com/anything")

	// --repo-url overrides the project's git_url.
	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "Override form")
	m.contains(m.run("issue", "attach-git", "ALPHA-3", "abc1234", "--repo-url", "https://gitlab.com/g/p.git"),
		"https://gitlab.com/g/p/-/commit/abc1234")

	// --pr records a pull request alongside the commit.
	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "With PR")
	out := m.run("issue", "attach-git", "ALPHA-4", "abc1234", "--pr", "42")
	m.contains(out, "/commit/abc1234", "https://github.com/warriorguo/alpha/pull/42")

	// done refuses without a commit link, and does not move the status.
	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "No commit")
	m.contains(m.fails("issue", "done", "ALPHA-5"), "no git_url", "attach-git", "--force")
	m.contains(m.run("issue", "show", "ALPHA-5"), "todo")

	// --force closes it anyway.
	m.contains(m.run("issue", "done", "ALPHA-5", "--force"), "done")

	// done --git attaches and closes in one step, walking todo -> in_progress -> done.
	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "One step")
	out = m.run("issue", "done", "ALPHA-6", "--git", "deadbee")
	m.contains(out, "https://github.com/warriorguo/alpha/commit/deadbee", "todo → in_progress → done")

	// A ref git cannot resolve, in a directory that is not a repo.
	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "Bad ref")
	m.contains(m.fails("issue", "attach-git", "ALPHA-7", "some-branch", "--repo", t.TempDir()),
		"cannot resolve")
}

// attach-git resolves a commit-ish against a real repository, which is the
// `--git HEAD` path used at the end of every piece of work.
func TestIntegrationAttachGitFromRealRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	m := newMFTest(t)

	// A project with no git_url, so the origin remote is the only source.
	m.run("project", "create", "ALPHA", "--name", "Alpha")
	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "From HEAD")

	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"remote", "add", "origin", "git@github.com:warriorguo/from-remote.git"},
		{"commit", "-q", "--allow-empty", "-m", "seed"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	head, err := gitOutput(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	// HEAD expands to the full sha, and the URL comes from the origin remote.
	out := m.run("issue", "attach-git", "ALPHA-1", "HEAD", "--repo", repo)
	m.contains(out, "https://github.com/warriorguo/from-remote/commit/"+head)

	// The default ref is HEAD.
	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "Default ref")
	m.contains(m.run("issue", "attach-git", "ALPHA-2", "--repo", repo), "/commit/"+head)

	// A short sha expands to the full one too.
	m.contains(m.run("issue", "attach-git", "ALPHA-2", head[:7], "--repo", repo), "/commit/"+head)
}

// With no project git_url and no repository, there is nothing to build a URL
// from, and the error should say so rather than recording something wrong.
func TestIntegrationAttachGitWithNoRepoSource(t *testing.T) {
	m := newMFTest(t)
	m.run("project", "create", "ALPHA", "--name", "Alpha")
	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "Nowhere")

	stderr := m.fails("issue", "attach-git", "ALPHA-1", "abc1234", "--repo", t.TempDir())
	m.contains(stderr, "cannot build a commit URL", "--repo-url")
}

func TestIntegrationIssueErrors(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("ALPHA", "Alpha")
	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "Exists")

	m.contains(m.fails("issue", "show", "ALPHA-999"), "not found")
	m.contains(m.fails("issue", "show"), "usage:")
	m.contains(m.fails("issue", "create", "ALPHA", "--title", "No type"), "--type and --title are required")
	m.contains(m.fails("issue", "create", "ALPHA", "--type", "bug"), "--type and --title are required")
	m.contains(m.fails("issue", "create", "ALPHA", "--type", "nonsense", "--title", "Bad type"), "type")
	m.contains(m.fails("issue", "create", "ALPHA", "--type", "bug", "--title", "Bad pri", "--priority", "P9"), "priority")
	m.contains(m.fails("issue", "update", "ALPHA-1"), "nothing to update")
	m.contains(m.fails("issue", "bogus", "ALPHA-1"), "unknown `mf issue` subcommand")
	m.contains(m.fails("issue", "create", "NOPE", "--type", "bug", "--title", "No project"), "not found")
	m.contains(m.fails("issue", "list"), "usage:")
	m.contains(m.fails("issue", "status", "ALPHA-1"), "usage:")
	m.contains(m.fails("issues", "ALPHA", "--nonsense"), "flag provided but not defined")
}

// --- dependencies ----------------------------------------------------------

func TestIntegrationDependencies(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("ALPHA", "Alpha")
	m.seedProject("BETA", "Beta")

	m.run("issue", "create", "ALPHA", "--type", "requirement", "--title", "Backend API", "--priority", "P0")
	m.run("issue", "create", "ALPHA", "--type", "requirement", "--title", "Frontend for it", "--priority", "P2")
	m.run("issue", "create", "BETA", "--type", "requirement", "--title", "Cross-project work", "--priority", "P2")

	// ALPHA-2 depends on ALPHA-1, given as keys on both sides.
	m.contains(m.run("issue", "dep", "add", "ALPHA-2", "ALPHA-1"), "ALPHA-2 depends_on ALPHA-1", "critical")

	// A cross-project dependency with an explicit type and severity.
	m.contains(m.run("issue", "dep", "add", "BETA-1", "ALPHA-1", "--type", "depends_on", "--severity", "recommended"),
		"BETA-1 depends_on ALPHA-1")

	// Listed from the dependent side.
	depsOfTwo := m.run("issue", "dep", "list", "ALPHA-2")
	m.contains(depsOfTwo, "depends_on", "ALPHA-1", "critical", "Backend API")

	// Listed from the depended-on side, the relation is restated.
	depsOfOne := m.run("issue", "dep", "list", "ALPHA-1")
	m.contains(depsOfOne, "blocks", "ALPHA-2", "BETA-1")

	// The tree shows both directions.
	tree := m.run("issue", "dep", "tree", "ALPHA-1")
	m.contains(tree, "ALPHA-1", "blocks →", "ALPHA-2")
	m.contains(m.run("issue", "dep", "tree", "ALPHA-2"), "depends on →", "ALPHA-1")

	// A critical dependency lifts the dependent issue's effective priority.
	m.contains(m.run("issue", "priority", "ALPHA-2"), "P0")
	// A recommended one does not.
	m.contains(m.run("issue", "priority", "BETA-1"), "P2")

	// --deps folds the list into `issue show`.
	m.contains(m.run("issue", "show", "ALPHA-2", "--deps"), "Dependencies:", "ALPHA-1")

	// Remove by the id the list prints.
	depID := extractDepID(t, depsOfTwo, "ALPHA-1")
	m.contains(m.run("issue", "dep", "rm", "ALPHA-2", depID), "removed dependency")
	m.contains(m.run("issue", "dep", "list", "ALPHA-2"), "no dependencies")

	// Error paths.
	m.contains(m.fails("issue", "dep", "add", "ALPHA-1", "NOPE-1"), "resolve dependency target")
	m.contains(m.fails("issue", "dep", "add", "ALPHA-1", "ALPHA-1"), "itself")
	m.contains(m.fails("issue", "dep", "add", "ALPHA-1"), "usage:")
	m.contains(m.fails("issue", "dep", "bogus", "ALPHA-1"), "unknown `mf issue dep` subcommand")
}

// extractDepID pulls the DEP_ID column out of a `dep list` row.
func extractDepID(t *testing.T, listOutput, issueKey string) string {
	t.Helper()
	for _, line := range strings.Split(listOutput, "\n") {
		if !strings.Contains(line, issueKey) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			return fields[len(fields)-1]
		}
	}
	t.Fatalf("no dependency row for %s in:\n%s", issueKey, listOutput)
	return ""
}

// A critical dependency cycle is rejected by the API; the CLI must surface it.
func TestIntegrationDependencyCycleRejected(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("ALPHA", "Alpha")
	m.run("issue", "create", "ALPHA", "--type", "requirement", "--title", "One")
	m.run("issue", "create", "ALPHA", "--type", "requirement", "--title", "Two")

	m.run("issue", "dep", "add", "ALPHA-2", "ALPHA-1")
	m.fails("issue", "dep", "add", "ALPHA-1", "ALPHA-2")
}

// --- tags ------------------------------------------------------------------

func TestIntegrationTags(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("ALPHA", "Alpha")
	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "Tag me")

	m.contains(m.run("tags"), "no tags defined")

	// Explicit creation.
	m.contains(m.run("tag", "create", "frontend", "--color", "#1890ff"), "created tag frontend")
	m.contains(m.run("tags"), "frontend", "#1890ff")

	// Attaching creates any tag that does not exist yet, and reuses the one
	// that does — several at once.
	m.contains(m.run("issue", "tag", "ALPHA-1", "frontend", "backend", "urgent"), "tagged ALPHA-1")
	m.contains(m.run("tags"), "frontend", "backend", "urgent")
	m.contains(m.run("issue", "show", "ALPHA-1"), "frontend", "backend", "urgent")

	// Tag names match case-insensitively, so this must not create a duplicate.
	m.run("issue", "tag", "ALPHA-1", "FRONTEND")
	if n := strings.Count(m.run("tags"), "frontend"); n != 1 {
		t.Errorf("expected one 'frontend' tag, found %d", n)
	}

	// Removal by name.
	m.contains(m.run("issue", "untag", "ALPHA-1", "backend"), "removed tag backend")
	m.omits(m.run("issue", "show", "ALPHA-1"), "backend")

	m.contains(m.fails("issue", "untag", "ALPHA-1", "nonexistent"), "no tag named")
	m.contains(m.fails("issue", "tag", "ALPHA-1"), "usage:")
	m.contains(m.fails("tag", "bogus"), "unknown `mf tag` subcommand")
}

// --- assets ------------------------------------------------------------------

// writeTempFile drops content on disk and returns its path, standing in for the
// art, clip, or log a user would be uploading.
func writeTempFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestIntegrationAssetLifecycle(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("OZX", "OZX Game")
	m.run("issue", "create", "OZX", "--type", "requirement", "--title", "New enemy")

	// Binary content, so a byte-mangling round trip would show up.
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x00, 0x1a, 0xff, 0xfe}
	art := writeTempFile(t, "enemy_ref.png", png)

	m.contains(m.run("asset", "add", "OZX-1", art), "uploaded enemy_ref.png", "OZX-1")
	m.contains(m.run("asset", "list", "OZX-1"), "enemy_ref.png", "image/png")

	// The download must be byte-identical, not merely similar.
	dir := t.TempDir()
	m.run("asset", "get", "OZX-1", "enemy_ref.png", "-o", filepath.Join(dir, "got.png"))
	got, err := os.ReadFile(filepath.Join(dir, "got.png"))
	if err != nil {
		t.Fatalf("read downloaded asset: %v", err)
	}
	if !bytes.Equal(got, png) {
		t.Errorf("downloaded bytes = %v, want %v", got, png)
	}

	// Replacing keeps the name, so anything referencing it stays valid.
	v2 := writeTempFile(t, "enemy_v2.png", []byte("final artwork"))
	m.contains(m.run("asset", "replace", "OZX-1", "enemy_ref.png", v2), "replaced enemy_ref.png")
	m.run("asset", "get", "OZX-1", "enemy_ref.png", "-o", filepath.Join(dir, "v2.png"))
	if got, _ := os.ReadFile(filepath.Join(dir, "v2.png")); string(got) != "final artwork" {
		t.Errorf("content after replace = %q", got)
	}

	m.contains(m.run("asset", "rm", "OZX-1", "enemy_ref.png", "--yes"), "deleted enemy_ref.png")
	m.contains(m.run("asset", "list", "OZX-1"), "no assets")
}

// An upload must not silently clobber an existing file of the same name.
func TestIntegrationAssetOverwriteIsExplicit(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("OZX", "OZX Game")
	m.run("issue", "create", "OZX", "--type", "bug", "--title", "Crash")

	first := writeTempFile(t, "log.txt", []byte("first"))
	m.run("asset", "add", "OZX-1", first)

	second := writeTempFile(t, "log.txt", []byte("second"))
	m.contains(m.fails("asset", "add", "OZX-1", second), "already exists")

	m.run("asset", "add", "OZX-1", second, "--overwrite")
	out := m.run("asset", "get", "OZX-1", "log.txt", "-o", "-")
	if out != "second" {
		t.Errorf("content after --overwrite = %q, want second", out)
	}
}

// The agent workflow: pull every asset into a working directory before coding.
func TestIntegrationAssetGetAll(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("OZX", "OZX Game")
	m.run("issue", "create", "OZX", "--type", "requirement", "--title", "Boss fight")

	for _, f := range []struct {
		name    string
		content string
	}{
		{"design.md", "# boss"},
		{"theme.wav", "RIFFdata"},
		{"sheet.png", "pixels"},
	} {
		m.run("asset", "add", "OZX-1", writeTempFile(t, f.name, []byte(f.content)))
	}

	dir := filepath.Join(t.TempDir(), "assets")
	m.contains(m.run("asset", "get", "OZX-1", "--all", "-o", dir), "3 asset(s)")

	for name, want := range map[string]string{"design.md": "# boss", "theme.wav": "RIFFdata", "sheet.png": "pixels"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("missing %s: %v", name, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	// --all needs somewhere to write.
	m.contains(m.fails("asset", "get", "OZX-1", "--all"), "-o <DIR> is required")
}

// Text content arriving on stdin, named explicitly — how an agent attaches a
// generated report without touching the filesystem.
func TestIntegrationAssetFromStdin(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("OZX", "OZX Game")
	m.run("issue", "create", "OZX", "--type", "bug", "--title", "Repro notes")

	withStdin(t, "step 1\nstep 2\n", func() {
		m.run("asset", "add", "OZX-1", "--file", "-", "--name", "repro.txt")
	})
	if out := m.run("asset", "get", "OZX-1", "repro.txt", "-o", "-"); out != "step 1\nstep 2\n" {
		t.Errorf("stdin-uploaded content = %q", out)
	}

	// Stdin has no name of its own.
	withStdin(t, "orphan", func() {
		m.contains(m.fails("asset", "add", "OZX-1", "--file", "-"), "--name is required")
	})
}

// `mf issue show` must surface attachments — otherwise an agent reading an
// issue never learns the material exists.
func TestIntegrationAssetsAppearInIssueShow(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("OZX", "OZX Game")
	m.run("issue", "create", "OZX", "--type", "requirement", "--title", "New enemy")
	m.run("asset", "add", "OZX-1", writeTempFile(t, "enemy_ref.png", []byte("pixels")))

	m.contains(m.run("issue", "show", "OZX-1"), "Assets:", "enemy_ref.png")

	data := m.json("issue", "show", "OZX-1")
	assets, ok := data["assets"].([]any)
	if !ok || len(assets) != 1 {
		t.Fatalf("issue show --json assets = %v", data["assets"])
	}
	if first, _ := assets[0].(map[string]any); first["filename"] != "enemy_ref.png" {
		t.Errorf("assets[0] = %v", assets[0])
	}
}

// A description can point at the issue's own files by name. `mf issue show`
// marks each reference so the reader can tell a live one from a dangling one.
func TestIntegrationAssetReferencesInDescription(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("OZX", "OZX Game")
	m.run("issue", "create", "OZX", "--type", "requirement", "--title", "New enemy",
		"--desc", "参考图 asset:enemy_ref.png，音效 asset:hit.wav（尚未上传）")
	m.run("asset", "add", "OZX-1", writeTempFile(t, "enemy_ref.png", []byte("pixels")))

	out := m.run("issue", "show", "OZX-1")
	m.contains(out, "asset:enemy_ref.png  (asset)", "asset:hit.wav  ⚠ missing asset")

	// Deleting a referenced file is allowed, but the dangling reference is
	// called out rather than left to be discovered later.
	m.contains(m.run("asset", "rm", "OZX-1", "enemy_ref.png", "--yes"),
		"deleted enemy_ref.png", "still references asset:enemy_ref.png")
}

func TestIntegrationAssetErrors(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("OZX", "OZX Game")
	m.run("issue", "create", "OZX", "--type", "bug", "--title", "Errors")

	m.contains(m.fails("asset", "list", "OZX-99"), "not found")
	m.contains(m.fails("asset", "get", "OZX-1", "absent.png"), "not found")
	m.contains(m.fails("asset", "rm", "OZX-1", "absent.png", "--yes"), "not found")
	m.contains(m.fails("asset", "add", "OZX-1", "/nonexistent/path.png"), "no such file")
	m.contains(m.fails("asset", "nonsense", "OZX-1"), "unknown `mf asset` subcommand")

	// A name that would escape the issue's namespace is refused by the server.
	m.contains(m.fails("asset", "add", "OZX-1", writeTempFile(t, "ok.txt", []byte("x")), "--name", "../escape.txt"), "filename")
}

// --- memories --------------------------------------------------------------

func TestIntegrationMemories(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("ALPHA", "Alpha")
	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "Investigated")

	m.contains(m.run("memory", "search"), "no memories match")

	// Attached to a project.
	m.contains(m.run("memory", "add", "--title", "Why we chose SQLite",
		"--content", "Single-user standalone needs no server.", "--project", "ALPHA"),
		"recorded recall memory")

	// Attached to an issue, which implies the project.
	m.run("memory", "add", "--title", "Root cause of ALPHA-1",
		"--content", "An off-by-one in the pager.", "--issue", "ALPHA-1", "--type", "write")

	// Content from stdin, and the positional-title spelling.
	withStdin(t, "Long form notes\nacross lines\n", func() {
		m.run("memory", "add", "Notes from stdin", "--content-file", "-", "--project", "ALPHA")
	})

	// Search: unfiltered, by keyword, by type, and scoped to a project.
	all := m.run("memory", "search")
	m.contains(all, "Why we chose SQLite", "Root cause of ALPHA-1", "Notes from stdin", "3 shown of 3")
	m.contains(m.run("memory", "search", "SQLite"), "Why we chose SQLite")
	m.omits(m.run("memory", "search", "SQLite"), "Root cause")
	m.contains(m.run("memory", "search", "--type", "write"), "Root cause of ALPHA-1")
	m.omits(m.run("memory", "search", "--type", "write"), "Why we chose SQLite")
	m.contains(m.run("memory", "search", "--project", "ALPHA"), "ALPHA")

	// --full prints content rather than a truncated column.
	m.contains(m.run("memory", "search", "--keyword", "stdin", "--full"), "Long form notes", "across lines")

	// Show, update, delete by id.
	id := firstMemoryID(t, m.run("memory", "search", "SQLite", "--json"))
	m.contains(m.run("memory", "show", id), "Why we chose SQLite", "Single-user standalone", "ALPHA")
	m.contains(m.run("memory", "update", id, "--title", "SQLite decision"), "updated memory")
	m.contains(m.run("memory", "show", id), "SQLite decision")
	m.contains(m.run("memory", "rm", id), "deleted memory")
	m.contains(m.fails("memory", "show", id), "not found")

	// Errors.
	m.contains(m.fails("memory", "add", "--content", "no title"), "--title is required")
	m.contains(m.fails("memory", "add", "--title", "no content"), "--content or --content-file is required")
	m.contains(m.fails("memory", "add", "--title", "t", "--content", "c", "--project", "NOPE"), "not found")
	m.contains(m.fails("memory", "rm", "not-a-uuid"), "not a memory id")
	m.contains(m.fails("memory", "update", "not-a-uuid"), "nothing to update")
	m.contains(m.fails("memory", "bogus"), "unknown `mf memory` subcommand")
}

func firstMemoryID(t *testing.T, jsonOut string) string {
	t.Helper()
	var env struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &env); err != nil {
		t.Fatalf("decode memory list: %v\n%s", err, jsonOut)
	}
	if len(env.Data) == 0 {
		t.Fatalf("no memories in:\n%s", jsonOut)
	}
	return env.Data[0].ID
}

// --- cross-cutting ---------------------------------------------------------

func TestIntegrationCtxAndEndpoint(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("ALPHA", "Alpha Project")
	m.seedProject("BETA", "Beta Project")

	out := m.run("ctx")
	m.contains(out, "Memory Flow at", m.url, "2 project(s)", "ALPHA", "Alpha Project", "BETA")
	// A pinned --url is not a local fallback, so no staleness warning.
	m.omits(out, "may lag")

	m.contains(m.run("endpoint"), m.url, "--url")
}

// Every command must produce valid JSON under --json, since that is the escape
// hatch when the text view omits a field.
func TestIntegrationJSONOutput(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("ALPHA", "Alpha")
	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "JSON check")
	m.run("issue", "tag", "ALPHA-1", "sometag")
	m.run("memory", "add", "--title", "A memory", "--content", "body", "--project", "ALPHA")

	commands := [][]string{
		{"ctx"},
		{"projects"},
		{"project", "show", "ALPHA"},
		{"project", "progress", "ALPHA"},
		{"issues", "ALPHA"},
		{"issue", "show", "ALPHA-1"},
		{"issue", "history", "ALPHA-1"},
		{"issue", "priority", "ALPHA-1"},
		{"issue", "dep", "list", "ALPHA-1"},
		{"issue", "dep", "tree", "ALPHA-1"},
		{"memory", "search"},
		{"asset", "list", "ALPHA-1"},
		{"tags"},
	}
	for _, args := range commands {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			out := m.run(append(args, "--json")...)
			var parsed any
			if err := json.Unmarshal([]byte(out), &parsed); err != nil {
				t.Errorf("mf %s --json emitted invalid JSON: %v\n%s", strings.Join(args, " "), err, out)
			}
		})
	}
}

// Mutating commands must honor --json too, and report the object they changed.
func TestIntegrationJSONOnMutations(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("ALPHA", "Alpha")

	created := m.json("issue", "create", "ALPHA", "--type", "bug", "--title", "Created via JSON")
	if created["key"] != "ALPHA-1" {
		t.Errorf("create --json data.key = %v, want ALPHA-1", created["key"])
	}
	updated := m.json("issue", "update", "ALPHA-1", "--priority", "P0")
	if updated["priority"] != "P0" {
		t.Errorf("update --json data.priority = %v, want P0", updated["priority"])
	}
	attached := m.json("issue", "attach-git", "ALPHA-1", "abc1234")
	if !strings.Contains(fmt.Sprint(attached["git_url"]), "/commit/abc1234") {
		t.Errorf("attach-git --json data.git_url = %v", attached["git_url"])
	}
	done := m.json("issue", "done", "ALPHA-1")
	if done["status"] != "done" {
		t.Errorf("done --json data.status = %v, want done", done["status"])
	}
}

func TestIntegrationGlobalFlagPlacement(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("ALPHA", "Alpha")
	m.run("issue", "create", "ALPHA", "--type", "bug", "--title", "Flag placement")

	// --json is equivalent before and after the command.
	before := m.run("--json", "issue", "show", "ALPHA-1")
	after := m.run("issue", "show", "ALPHA-1", "--json")
	if before != after {
		t.Errorf("global flag position changed the output:\n%s\nvs\n%s", before, after)
	}

	// --timeout is accepted and does not leak into subcommand parsing.
	m.run("--timeout", "10", "issue", "show", "ALPHA-1")
	m.run("issue", "show", "ALPHA-1", "--timeout=10")
}

func TestIntegrationHelpTopics(t *testing.T) {
	var stdout, stderr strings.Builder
	for _, args := range [][]string{
		{}, {"help"}, {"-h"}, {"--help"},
		{"help", "issue"}, {"help", "project"}, {"help", "memory"},
		{"help", "asset"}, {"help", "tag"}, {"help", "workflow"}, {"help", "nonsense"},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Main(args, &stdout, &stderr); code != 0 {
			t.Errorf("mf %v exited %d", args, code)
		}
		if stdout.Len() == 0 {
			t.Errorf("mf %v printed no help", args)
		}
	}

	// Each topic covers its own commands.
	stdout.Reset()
	Main([]string{"help", "issue"}, &stdout, &stderr)
	for _, want := range []string{"attach-git", "--desc-file", "dep add", "untag"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("issue help missing %q", want)
		}
	}
	stdout.Reset()
	Main([]string{"help", "asset"}, &stdout, &stderr)
	for _, want := range []string{"asset add", "--all", "asset:", "replace"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("asset help missing %q", want)
		}
	}
	stdout.Reset()
	Main([]string{"help", "workflow"}, &stdout, &stderr)
	for _, want := range []string{"todo", "in_progress", "--direct", "git_url"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("workflow help missing %q", want)
		}
	}
}

func TestIntegrationVersion(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := Main([]string{"version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("version exited %d", code)
	}
	if !strings.HasPrefix(stdout.String(), "mf ") {
		t.Errorf("version = %q", stdout.String())
	}
}

// A UUID works anywhere an issue or project key does.
func TestIntegrationUUIDsAreAccepted(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("ALPHA", "Alpha")
	created := m.json("issue", "create", "ALPHA", "--type", "bug", "--title", "By UUID")

	issueID, _ := created["id"].(string)
	m.contains(m.run("issue", "show", issueID), "ALPHA-1", "By UUID")

	project := m.json("project", "show", "ALPHA")
	projectID, _ := project["id"].(string)
	m.contains(m.run("issues", projectID), "ALPHA-1")
}

// An unreachable instance must fail with guidance, not hang or panic.
func TestIntegrationUnreachableInstance(t *testing.T) {
	_, stderr, code := runCLI(t, "http://127.0.0.1:1", "ctx")
	if code == 0 {
		t.Fatal("an unreachable instance should exit non-zero")
	}
	if stderr == "" {
		t.Error("an unreachable instance should explain itself")
	}
}
