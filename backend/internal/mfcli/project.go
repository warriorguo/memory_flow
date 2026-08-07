package mfcli

import (
	"fmt"
	"strconv"

	"github.com/warriorguo/memory_flow/backend/internal/model"
)

// cmdCtx is the one-shot activation bootstrap: it resolves the endpoint and
// lists every project, so an agent can learn the landscape in a single call.
func cmdCtx(e *env, args []string) error {
	fs := newFlagSet("ctx")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}

	var projects []model.Project
	raw, err := e.client.get("/api/v1/projects"+query(map[string]string{"page_size": "200"}), &projects)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}

	base, _ := e.client.Base()
	e.printf("Memory Flow at %s (%s)\n", base, e.client.Source())
	if e.client.Source() != "remote" && e.client.Source() != "--url" && e.client.Source() != "MEMORY_FLOW_URL" {
		e.printf("NOTE: this is a local instance — its data may lag the home server until synced.\n")
	}
	e.printf("\n%d project(s):\n\n", len(projects))

	rows := make([][]string, 0, len(projects))
	for _, p := range projects {
		rows = append(rows, []string{p.Key, p.Name, p.Status, truncate(deref(p.Summary), 90)})
	}
	table(e.out, []string{"KEY", "NAME", "STATUS", "SUMMARY"}, rows)
	return nil
}

func cmdEndpoint(e *env, args []string) error {
	fs := newFlagSet("endpoint")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	base, err := e.client.Base()
	if err != nil {
		return err
	}
	e.printf("%s  (%s)\n", base, e.client.Source())
	return nil
}

func cmdProjectList(e *env, args []string) error {
	fs := newFlagSet("project list")
	status := fs.String("status", "", "filter by status (active/paused/archived)")
	name := fs.String("name", "", "filter by name or key")
	pageSize := fs.Int("limit", 200, "max projects to return")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}

	var projects []model.Project
	raw, err := e.client.get("/api/v1/projects"+query(map[string]string{
		"status":    *status,
		"name":      *name,
		"page_size": strconv.Itoa(*pageSize),
	}), &projects)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}

	rows := make([][]string, 0, len(projects))
	for _, p := range projects {
		rows = append(rows, []string{p.Key, p.Name, p.Status, truncate(deref(p.Summary), 90)})
	}
	table(e.out, []string{"KEY", "NAME", "STATUS", "SUMMARY"}, rows)
	return nil
}

func cmdProjectShow(e *env, args []string) error {
	fs := newFlagSet("project show")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "project show <PROJECT_KEY>"); err != nil {
		return err
	}

	var p model.Project
	raw, err := e.client.get("/api/v1/projects/"+pos[0], &p)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}

	e.printf("%s  %s\n", p.Key, p.Name)
	e.printf("  status        %s\n", p.Status)
	e.printf("  git_url       %s\n", dash(p.GitURL))
	e.printf("  cicd_url      %s\n", dash(p.CICDURL))
	e.printf("  doc_url       %s\n", dash(p.DocURL))
	e.printf("  owner         %s\n", dash(p.OwnerID))
	// next_issue_number holds the last number handed out — the repository
	// increments it and uses the result — so the next key is one past it.
	e.printf("  next issue    %s-%d\n", p.Key, p.NextIssueNumber+1)
	if v := deref(p.Summary); v != "" {
		e.printf("\nSummary:\n%s\n", v)
	}
	if v := deref(p.Description); v != "" {
		e.printf("\nDescription:\n%s\n", v)
	}
	if v := deref(p.DesignPrinciples); v != "" {
		e.printf("\nDesign principles:\n%s\n", v)
	}
	return nil
}

func cmdProjectCreate(e *env, args []string) error {
	fs := newFlagSet("project create")
	key := fs.String("key", "", "project key (uppercase, 2-10 chars)")
	name := fs.String("name", "", "project name")
	summary := fs.String("summary", "", "one-line summary")
	description := fs.String("desc", "", "long description")
	descFile := fs.String("desc-file", "", "read description from file ('-' for stdin)")
	principles := fs.String("design-principles", "", "design principles")
	gitURL := fs.String("git-url", "", "repository URL")
	cicdURL := fs.String("cicd-url", "", "CI/CD URL")
	docURL := fs.String("doc-url", "", "documentation URL")
	owner := fs.String("owner", "", "owner id")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	// `mf project create MF --name …` is accepted as shorthand for --key.
	if len(pos) > 0 && *key == "" {
		*key = pos[0]
		pos = pos[1:]
	}
	if err := need(pos, 0, "project create <KEY> --name <NAME> [--summary …]"); err != nil {
		return err
	}
	if *key == "" || *name == "" {
		return fmt.Errorf("--key and --name are required")
	}

	desc, err := readContent(*description, *descFile)
	if err != nil {
		return err
	}

	req := model.CreateProjectRequest{Key: *key, Name: *name}
	setIfNonEmpty(&req.Summary, *summary)
	setIfNonEmpty(&req.Description, desc)
	setIfNonEmpty(&req.DesignPrinciples, *principles)
	setIfNonEmpty(&req.GitURL, *gitURL)
	setIfNonEmpty(&req.CICDURL, *cicdURL)
	setIfNonEmpty(&req.DocURL, *docURL)
	setIfNonEmpty(&req.OwnerID, *owner)

	var p model.Project
	raw, err := e.client.post("/api/v1/projects", req, &p)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	e.printf("created project %s (%s)\n", p.Key, p.Name)
	return nil
}

func cmdProjectUpdate(e *env, args []string) error {
	fs := newFlagSet("project update")
	name := fs.String("name", "", "project name")
	summary := fs.String("summary", "", "one-line summary")
	description := fs.String("desc", "", "long description")
	descFile := fs.String("desc-file", "", "read description from file ('-' for stdin)")
	principles := fs.String("design-principles", "", "design principles")
	gitURL := fs.String("git-url", "", "repository URL")
	cicdURL := fs.String("cicd-url", "", "CI/CD URL")
	docURL := fs.String("doc-url", "", "documentation URL")
	owner := fs.String("owner", "", "owner id")
	status := fs.String("status", "", "active/paused/archived")
	nextIssue := fs.Int("next-issue-number", 0, "set the last-used issue number (the next issue becomes N+1); may only increase")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "project update <PROJECT_KEY> [--name …]"); err != nil {
		return err
	}

	req := model.UpdateProjectRequest{
		Name:             strPtr(fs, "name", name),
		Summary:          strPtr(fs, "summary", summary),
		DesignPrinciples: strPtr(fs, "design-principles", principles),
		GitURL:           strPtr(fs, "git-url", gitURL),
		CICDURL:          strPtr(fs, "cicd-url", cicdURL),
		DocURL:           strPtr(fs, "doc-url", docURL),
		OwnerID:          strPtr(fs, "owner", owner),
		Status:           strPtr(fs, "status", status),
	}
	if flagWasSet(fs, "next-issue-number") {
		req.NextIssueNumber = nextIssue
	}
	if flagWasSet(fs, "desc") || flagWasSet(fs, "desc-file") {
		desc, err := readContent(*description, *descFile)
		if err != nil {
			return err
		}
		req.Description = &desc
	}

	var p model.Project
	raw, err := e.client.put("/api/v1/projects/"+pos[0], req, &p)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	e.printf("updated project %s\n", p.Key)
	return nil
}

func cmdProjectArchive(e *env, args []string) error {
	fs := newFlagSet("project archive")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "project archive <PROJECT_KEY>"); err != nil {
		return err
	}
	raw, err := e.client.del("/api/v1/projects/" + pos[0])
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	e.printf("archived project %s\n", pos[0])
	return nil
}

func cmdProjectProgress(e *env, args []string) error {
	fs := newFlagSet("project progress")
	days := fs.Int("trend", 0, "also show the daily created/done trend over N days")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "project progress <PROJECT_KEY> [--trend <DAYS>]"); err != nil {
		return err
	}

	var summary model.ProgressSummary
	raw, err := e.client.get("/api/v1/projects/"+pos[0]+"/progress/summary", &summary)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) && *days == 0 {
		return nil
	}
	if !e.asJSON {
		e.printf("%s: %d issue(s) total\n", pos[0], summary.Total)
		e.printf("  status    %s\n", sortedCounts(summary.StatusCounts,
			[]string{"todo", "in_progress", "review", "testing", "done", "closed", "suspended", "rejected"}))
		e.printf("  priority  %s\n", sortedCounts(summary.PriorityCounts, []string{"P0", "P1", "P2"}))
		e.printf("  type      %s\n", sortedCounts(summary.TypeCounts, []string{"bug", "requirement"}))
	}

	if *days > 0 {
		var trend []model.TrendPoint
		trendRaw, err := e.client.get("/api/v1/projects/"+pos[0]+"/progress/trend"+
			query(map[string]string{"days": strconv.Itoa(*days)}), &trend)
		if err != nil {
			return err
		}
		if e.emitJSON(trendRaw) {
			return nil
		}
		e.printf("\nTrend (last %d days):\n", *days)
		rows := make([][]string, 0, len(trend))
		for _, p := range trend {
			rows = append(rows, []string{p.Date, strconv.Itoa(p.Created), strconv.Itoa(p.Done)})
		}
		table(e.out, []string{"DATE", "CREATED", "DONE"}, rows)
	}
	return nil
}

// setIfNonEmpty assigns a value to an optional field only when it was given,
// leaving the field nil (and so omitted server-side) otherwise.
func setIfNonEmpty(field **string, value string) {
	if value == "" {
		return
	}
	v := value
	*field = &v
}
