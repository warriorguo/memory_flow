package mfcli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/warriorguo/memory_flow/backend/internal/model"
)

// closedStatuses are hidden from `mf issue list` unless --all or an explicit
// --status is given: the default question is "what is still open?".
var closedStatuses = map[string]bool{"done": true, "closed": true, "rejected": true}

func cmdIssueList(e *env, args []string) error {
	fs := newFlagSet("issue list")
	status := fs.String("status", "", "filter by status")
	issueType := fs.String("type", "", "bug or requirement")
	priority := fs.String("priority", "", "P0/P1/P2")
	assignee := fs.String("assignee", "", "filter by assignee id")
	keyword := fs.String("keyword", "", "search titles and descriptions")
	all := fs.Bool("all", false, "include done/closed/rejected issues")
	limit := fs.Int("limit", 50, "max issues to show")
	page := fs.Int("page", 1, "page number")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "issue list <PROJECT_KEY> [--status …] [--all]"); err != nil {
		return err
	}

	// Open-only is a client-side filter because the API filters on one exact
	// status, not a set. Over-fetch so the trimmed page still fills up.
	filterOpen := *status == "" && !*all
	pageSize := *limit
	if filterOpen && pageSize < 200 {
		pageSize = 200
	}

	var issues []model.Issue
	raw, err := e.client.get("/api/v1/projects/"+pos[0]+"/issues"+query(map[string]string{
		"status":      *status,
		"type":        *issueType,
		"priority":    *priority,
		"assignee_id": *assignee,
		"keyword":     *keyword,
		"page":        strconv.Itoa(*page),
		"page_size":   strconv.Itoa(pageSize),
	}), &issues)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}

	total := listTotal(raw)
	shown := issues
	if filterOpen {
		shown = shown[:0:0]
		for _, issue := range issues {
			if !closedStatuses[issue.Status] {
				shown = append(shown, issue)
			}
		}
	}
	truncated := false
	if len(shown) > *limit {
		shown, truncated = shown[:*limit], true
	}

	if len(shown) == 0 {
		if filterOpen && total > 0 {
			e.printf("no open issues in %s (%d total — use --all to include done/closed)\n", pos[0], total)
		} else {
			e.printf("no issues match in %s\n", pos[0])
		}
		return nil
	}

	table(e.out, issueHeader, issueRows(shown))
	switch {
	case truncated:
		e.printf("\nshowing %d (--limit) of %d matching; %d issues in project\n", len(shown), len(issues), total)
	case filterOpen:
		e.printf("\n%d open of %d total in %s\n", len(shown), total, pos[0])
	default:
		e.printf("\n%d shown of %d matching\n", len(shown), total)
	}
	return nil
}

func cmdIssueShow(e *env, args []string) error {
	fs := newFlagSet("issue show")
	withDeps := fs.Bool("deps", false, "also show dependencies")
	withHistory := fs.Bool("history", false, "also show change history")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "issue show <ISSUE_KEY> [--deps] [--history]"); err != nil {
		return err
	}

	issue, raw, err := e.fetchIssue(pos[0])
	if err != nil {
		return err
	}

	// Attachments are supplementary: an instance that cannot serve them (an
	// older server) should still show the issue.
	assets, _, assetErr := e.fetchAssets(pos[0])
	if assetErr != nil {
		assets = nil
	}
	if e.asJSON {
		raw = withAssets(raw, assets)
	}

	if e.emitJSON(raw) && !*withDeps && !*withHistory {
		return nil
	}
	if !e.asJSON {
		printIssue(e.out, issue)
		printAssets(e, assets)
	}

	if *withDeps {
		deps, depRaw, err := e.fetchDeps(pos[0])
		if err != nil {
			return err
		}
		if e.emitJSON(depRaw) {
			return nil
		}
		if len(deps) > 0 {
			e.printf("\nDependencies:\n")
			if err := e.printDeps(pos[0], deps); err != nil {
				return err
			}
		}
	}
	if *withHistory {
		if err := printHistory(e, pos[0]); err != nil {
			return err
		}
	}
	return nil
}

func cmdIssueCreate(e *env, args []string) error {
	fs := newFlagSet("issue create")
	issueType := fs.String("type", "", "bug or requirement (required)")
	title := fs.String("title", "", "issue title (required)")
	description := fs.String("desc", "", "description")
	descFile := fs.String("desc-file", "", "read description from file ('-' for stdin)")
	priority := fs.String("priority", "", "P0/P1/P2 (default P2)")
	assignee := fs.String("assignee", "", "assignee id")
	source := fs.String("source", "", "where the issue came from")
	version := fs.String("version", "", "affected version")
	gitURL := fs.String("git-url", "", "commit or PR URL")
	prURL := fs.String("pr-url", "", "pull request URL")
	docURL := fs.String("doc-url", "", "documentation URL")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, `issue create <PROJECT_KEY> --type <bug|requirement> --title "…"`); err != nil {
		return err
	}
	if *issueType == "" || *title == "" {
		return fmt.Errorf("--type and --title are required")
	}

	desc, err := readContent(*description, *descFile)
	if err != nil {
		return err
	}

	req := model.CreateIssueRequest{Type: *issueType, Title: *title}
	setIfNonEmpty(&req.Description, desc)
	setIfNonEmpty(&req.Priority, *priority)
	setIfNonEmpty(&req.AssigneeID, *assignee)
	setIfNonEmpty(&req.Source, *source)
	setIfNonEmpty(&req.Version, *version)
	setIfNonEmpty(&req.GitURL, *gitURL)
	setIfNonEmpty(&req.PRURL, *prURL)
	setIfNonEmpty(&req.DocURL, *docURL)

	var issue model.Issue
	raw, err := e.client.post("/api/v1/projects/"+pos[0]+"/issues", req, &issue)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	e.printf("created %s  [%s] %s %s\n", issue.IssueKey, issue.Type, issue.Priority, issue.Title)
	return nil
}

func cmdIssueUpdate(e *env, args []string) error {
	fs := newFlagSet("issue update")
	issueType := fs.String("type", "", "bug or requirement")
	title := fs.String("title", "", "issue title")
	description := fs.String("desc", "", "description")
	descFile := fs.String("desc-file", "", "read description from file ('-' for stdin)")
	priority := fs.String("priority", "", "P0/P1/P2")
	assignee := fs.String("assignee", "", "assignee id")
	source := fs.String("source", "", "where the issue came from")
	version := fs.String("version", "", "affected version")
	gitURL := fs.String("git-url", "", "commit or PR URL")
	prURL := fs.String("pr-url", "", "pull request URL")
	docURL := fs.String("doc-url", "", "documentation URL")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, `issue update <ISSUE_KEY> [--title "…"] [--priority P1] …`); err != nil {
		return err
	}

	req := model.UpdateIssueRequest{
		Type:       strPtr(fs, "type", issueType),
		Title:      strPtr(fs, "title", title),
		Priority:   strPtr(fs, "priority", priority),
		AssigneeID: strPtr(fs, "assignee", assignee),
		Source:     strPtr(fs, "source", source),
		Version:    strPtr(fs, "version", version),
		GitURL:     strPtr(fs, "git-url", gitURL),
		PRURL:      strPtr(fs, "pr-url", prURL),
		DocURL:     strPtr(fs, "doc-url", docURL),
	}
	if flagWasSet(fs, "desc") || flagWasSet(fs, "desc-file") {
		desc, err := readContent(*description, *descFile)
		if err != nil {
			return err
		}
		req.Description = &desc
	}
	if req == (model.UpdateIssueRequest{}) {
		return fmt.Errorf("nothing to update: pass at least one field flag (see `mf help issue`)")
	}

	var issue model.Issue
	raw, err := e.client.put("/api/v1/issues/"+pos[0], req, &issue)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	e.printf("updated %s\n", issue.IssueKey)
	return nil
}

func cmdIssueStatus(e *env, args []string) error {
	fs := newFlagSet("issue status")
	direct := fs.Bool("direct", false, "fail instead of walking through intermediate statuses")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 2, "issue status <ISSUE_KEY> <STATUS>"); err != nil {
		return err
	}
	_, err = e.transition(pos[0], pos[1], *direct)
	return err
}

func cmdIssueStart(e *env, args []string) error {
	fs := newFlagSet("issue start")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "issue start <ISSUE_KEY>"); err != nil {
		return err
	}
	_, err = e.transition(pos[0], "in_progress", false)
	return err
}

// cmdIssueDone enforces the completion workflow in one command: the commit link
// must be recorded before an issue closes, and the status walk to `done` may
// need intermediate hops the API rejects individually.
func cmdIssueDone(e *env, args []string) error {
	fs := newFlagSet("issue done")
	gitRef := fs.String("git", "", "commit-ish or URL to record as git_url before closing")
	repo := fs.String("repo", "", "git repository directory for --git (default: current directory)")
	repoURL := fs.String("repo-url", "", "override the repository URL used to build the commit link")
	force := fs.Bool("force", false, "close even without a git_url")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "issue done <ISSUE_KEY> [--git <sha|url>]"); err != nil {
		return err
	}
	key := pos[0]

	issue, _, err := e.fetchIssue(key)
	if err != nil {
		return err
	}

	if *gitRef != "" {
		issue, err = e.attachGit(key, *gitRef, *repo, *repoURL)
		if err != nil {
			return err
		}
		e.printf("git_url  %s\n", deref(issue.GitURL))
	}

	if deref(issue.GitURL) == "" && !*force {
		return fmt.Errorf("%s has no git_url, and the completion workflow requires one.\n"+
			"  record the commit:  mf issue attach-git %s <sha>\n"+
			"  or do both at once: mf issue done %s --git <sha>\n"+
			"  or override:        mf issue done %s --force", key, key, key, key)
	}

	_, err = e.transition(key, "done", false)
	return err
}

func cmdIssueAttachGit(e *env, args []string) error {
	fs := newFlagSet("issue attach-git")
	repo := fs.String("repo", "", "git repository directory (default: current directory)")
	repoURL := fs.String("repo-url", "", "override the repository URL used to build the link")
	pr := fs.String("pr", "", "also record a pull request (number or URL)")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(pos) == 0 || len(pos) > 2 {
		return fmt.Errorf("usage: mf issue attach-git <ISSUE_KEY> [<sha|url>]  (default: HEAD)")
	}
	key := pos[0]
	ref := "HEAD"
	if len(pos) == 2 {
		ref = pos[1]
	}

	issue, err := e.attachGit(key, ref, *repo, *repoURL)
	if err != nil {
		return err
	}

	if *pr != "" {
		prLink := *pr
		if !IsURL(prLink) {
			base := *repoURL
			if base == "" {
				base = e.repoURLFor(key, *repo)
			}
			prLink, err = PRURL(base, strings.TrimPrefix(*pr, "#"))
			if err != nil {
				return err
			}
		}
		var updated model.Issue
		if _, err := e.client.put("/api/v1/issues/"+key, model.UpdateIssueRequest{PRURL: &prLink}, &updated); err != nil {
			return err
		}
		issue = &updated
	}

	if e.asJSON {
		raw, err := e.client.get("/api/v1/issues/"+key, nil)
		if err != nil {
			return err
		}
		e.emitJSON(raw)
		return nil
	}
	e.printf("%s git_url  %s\n", issue.IssueKey, deref(issue.GitURL))
	if v := deref(issue.PRURL); v != "" {
		e.printf("%s pr_url   %s\n", issue.IssueKey, v)
	}
	return nil
}

func cmdIssueHistory(e *env, args []string) error {
	fs := newFlagSet("issue history")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "issue history <ISSUE_KEY>"); err != nil {
		return err
	}
	return printHistory(e, pos[0])
}

func cmdIssuePriority(e *env, args []string) error {
	fs := newFlagSet("issue priority")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "issue priority <ISSUE_KEY>"); err != nil {
		return err
	}

	var result struct {
		EffectivePriority string `json:"effective_priority"`
		OwnPriority       string `json:"own_priority"`
		Priority          string `json:"priority"`
	}
	raw, err := e.client.get("/api/v1/issues/"+pos[0]+"/effective-priority", &result)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	effective := result.EffectivePriority
	if effective == "" {
		effective = result.Priority
	}
	e.printf("%s effective priority: %s", pos[0], effective)
	if result.OwnPriority != "" && result.OwnPriority != effective {
		e.printf(" (own: %s, inherited through a critical dependency)", result.OwnPriority)
	}
	e.printf("\n")
	return nil
}

// --- shared issue helpers --------------------------------------------------

func (e *env) fetchIssue(key string) (*model.Issue, []byte, error) {
	var issue model.Issue
	raw, err := e.client.get("/api/v1/issues/"+key, &issue)
	if err != nil {
		return nil, nil, err
	}
	return &issue, raw, nil
}

// transition moves an issue to target, walking intermediate statuses when the
// direct hop is not legal (todo -> done becomes todo -> in_progress -> done).
func (e *env) transition(key, target string, direct bool) (*model.Issue, error) {
	issue, _, err := e.fetchIssue(key)
	if err != nil {
		return nil, err
	}
	if issue.Status == target {
		e.printf("%s is already %s\n", issue.IssueKey, target)
		return issue, nil
	}

	steps := []string{target}
	if !direct {
		path, ok := model.TransitionPath(issue.Status, target)
		if !ok {
			return nil, fmt.Errorf("%s is %s, and there is no path to %s in the workflow", key, issue.Status, target)
		}
		steps = path
	} else if !model.CanTransition(issue.Status, target) {
		return nil, fmt.Errorf("%s is %s: %s -> %s is not a legal transition (drop --direct to walk through intermediate statuses)",
			key, issue.Status, issue.Status, target)
	}

	from := issue.Status
	var raw []byte
	for _, step := range steps {
		var updated model.Issue
		raw, err = e.client.patch("/api/v1/issues/"+key+"/status", model.TransitionStatusRequest{Status: step}, &updated)
		if err != nil {
			return nil, err
		}
		issue = &updated
	}
	if e.emitJSON(raw) {
		return issue, nil
	}
	if len(steps) > 1 {
		e.printf("%s: %s → %s\n", issue.IssueKey, from, strings.Join(steps, " → "))
	} else {
		e.printf("%s: %s → %s\n", issue.IssueKey, from, issue.Status)
	}
	return issue, nil
}

// attachGit records a commit link on an issue. ref may be a full URL (used
// as-is), a commit-ish resolved against the local repo, or a bare sha.
func (e *env) attachGit(key, ref, repo, repoURLOverride string) (*model.Issue, error) {
	link := ref
	if !IsURL(ref) {
		sha, err := ExpandSHA(repo, ref)
		if err != nil {
			return nil, err
		}
		base := repoURLOverride
		if base == "" {
			base = e.repoURLFor(key, repo)
		}
		if base == "" {
			return nil, fmt.Errorf("cannot build a commit URL for %s: the project has no git_url and %s has no origin remote.\n"+
				"  pass a full URL, or --repo-url <repo>", key, dirLabel(repo))
		}
		link, err = CommitURL(base, sha)
		if err != nil {
			return nil, err
		}
	}

	var issue model.Issue
	if _, err := e.client.put("/api/v1/issues/"+key, model.UpdateIssueRequest{GitURL: &link}, &issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

// repoURLFor finds the repository for an issue: the project's recorded git_url
// first (authoritative, and correct even when run from another directory),
// falling back to the local checkout's origin remote.
func (e *env) repoURLFor(issueKey, repoDir string) string {
	if projectKey := projectKeyOf(issueKey); projectKey != "" {
		var p model.Project
		if _, err := e.client.get("/api/v1/projects/"+projectKey, &p); err == nil {
			if url := deref(p.GitURL); url != "" {
				return url
			}
		}
	}
	return RepoRemote(repoDir)
}

var issueKeyPattern = regexp.MustCompile(`^([A-Za-z0-9]{2,10})-\d+$`)

// projectKeyOf extracts the project key from an issue key ("ORT-100" -> "ORT"),
// returning "" when the argument is a UUID or otherwise not a key.
func projectKeyOf(issueKey string) string {
	if m := issueKeyPattern.FindStringSubmatch(issueKey); m != nil {
		return m[1]
	}
	return ""
}

func printHistory(e *env, key string) error {
	var history []model.IssueHistory
	raw, err := e.client.get("/api/v1/issues/"+key+"/history", &history)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	if len(history) == 0 {
		e.printf("\nno recorded changes for %s\n", key)
		return nil
	}
	e.printf("\nHistory:\n")
	rows := make([][]string, 0, len(history))
	for _, h := range history {
		rows = append(rows, []string{
			shortTime(h.CreatedAt),
			h.FieldName,
			truncate(dash(h.OldValue), 40),
			truncate(dash(h.NewValue), 40),
			dash(h.OperatorID),
		})
	}
	table(e.out, []string{"WHEN", "FIELD", "FROM", "TO", "BY"}, rows)
	return nil
}
