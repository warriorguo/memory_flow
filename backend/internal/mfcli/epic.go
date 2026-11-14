package mfcli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/warriorguo/memory_flow/backend/internal/model"
)

// Epics (MF-39). An epic is an issue of type "epic"; other issues, from any
// project, join it through `mf epic add`. Its memories are shared with every
// issue in it.

func dispatchEpic(e *env, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mf epic <list|show|create|add|rm|progress|close> …")
	}
	switch args[0] {
	case "list", "ls":
		return cmdEpicList(e, args[1:])
	case "show", "get":
		return cmdEpicShow(e, args[1:])
	case "create", "new":
		return cmdEpicCreate(e, args[1:])
	case "add":
		return cmdEpicAdd(e, args[1:])
	case "rm", "remove":
		return cmdEpicRemove(e, args[1:])
	case "progress":
		return cmdEpicProgress(e, args[1:])
	case "close", "done":
		return cmdEpicClose(e, args[1:])
	default:
		return unknownf("unknown `mf epic` subcommand %q", args[0])
	}
}

// epicRouteMissing turns the router's bare 404 into "upgrade the server".
func epicRouteMissing(err error) error {
	if err != nil && strings.Contains(err.Error(), "404 page not found") {
		return fmt.Errorf("this Memory Flow server predates epics (MF-39); upgrade it to use `mf epic`")
	}
	return err
}

func percent(r float64) string { return fmt.Sprintf("%.0f%%", r*100) }

func cmdEpicList(e *env, args []string) error {
	fs := newFlagSet("epic list")
	all := fs.Bool("all", false, "include done/closed epics")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "epic list <PROJECT_KEY> [--all]"); err != nil {
		return err
	}
	var epics []model.EpicSummary
	raw, err := e.client.get("/api/v1/projects/"+pos[0]+"/epics", &epics)
	if err != nil {
		return epicRouteMissing(err)
	}
	if e.emitJSON(raw) {
		return nil
	}
	rows := [][]string{}
	hidden := 0
	for _, ep := range epics {
		if !*all && closedStatuses[ep.Status] {
			hidden++
			continue
		}
		rows = append(rows, []string{
			ep.IssueKey, truncate(ep.Title, 50), ep.Status,
			fmt.Sprintf("%d/%d", ep.Counts.Done, ep.Counts.Total), percent(ep.Counts.DoneRatio),
		})
	}
	if len(rows) == 0 {
		if hidden > 0 {
			e.printf("no open epics in %s (%d done/closed — use --all)\n", pos[0], hidden)
		} else {
			e.printf("no epics in %s (create one with: mf epic create %s --title \"…\")\n", pos[0], pos[0])
		}
		return nil
	}
	table(e.out, []string{"KEY", "TITLE", "STATUS", "DONE", "%"}, rows)
	if hidden > 0 {
		e.printf("\n%d done/closed epic(s) hidden — use --all\n", hidden)
	}
	return nil
}

func (e *env) fetchEpic(key string) (*model.EpicDetail, []byte, error) {
	var d model.EpicDetail
	raw, err := e.client.get("/api/v1/epics/"+key, &d)
	if err != nil {
		return nil, nil, epicRouteMissing(err)
	}
	return &d, raw, nil
}

func cmdEpicShow(e *env, args []string) error {
	fs := newFlagSet("epic show")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "epic show <EPIC_KEY>"); err != nil {
		return err
	}
	d, raw, err := e.fetchEpic(pos[0])
	if err != nil {
		return err
	}
	assets, _, assetErr := e.fetchAssets(pos[0])
	if assetErr != nil {
		assets = nil
	}
	memories, _ := e.fetchIssueMemories(d.Epic.ID)
	if e.asJSON {
		raw = withAssets(raw, assets)
		raw = withMemories(raw, memories)
		e.emitJSON(raw)
		return nil
	}
	printIssue(e.out, d.Epic)
	printEpicBody(e, d)
	printAssets(e, assets)
	if len(memories) > 0 {
		e.printf("\nEpic memories (shared with every issue above):\n")
		printMemoryRows(e, memories)
	}
	return nil
}

// printEpicBody renders an epic's issues, progress, dependencies and blocks.
func printEpicBody(e *env, d *model.EpicDetail) {
	c := d.Counts
	e.printf("\nProgress: %d/%d done (%s)  —  %s\n", c.Done, c.Total, percent(c.DoneRatio),
		sortedCounts(c.StatusCounts, statusOrder))
	if len(d.Children) == 0 {
		e.printf("\nNo issues yet — add some with: mf epic add %s <ISSUE_KEY>…\n", d.Epic.IssueKey)
		return
	}
	issues := make([]model.Issue, len(d.Children))
	for i, ch := range d.Children {
		issues[i] = ch.Issue
	}
	e.printf("\nIssues:\n")
	e.issueTable(issues)
	if len(d.Dependencies) > 0 {
		e.printf("\nDependencies:\n")
		for _, dep := range d.Dependencies {
			verb := "depends on"
			if dep.Type == "blocks" {
				verb = "blocks"
			}
			e.printf("  %s %s %s (%s)\n", dep.Source, verb, dep.Target, dep.Severity)
		}
	}
	printBlocked(e, d.Blocked)
}

func printBlocked(e *env, blocked []model.BlockedIssue) {
	if len(blocked) == 0 {
		return
	}
	e.printf("\nBlocked (a critical dependency is not done yet):\n")
	for _, b := range blocked {
		by := make([]string, len(b.BlockedBy))
		for i, r := range b.BlockedBy {
			by[i] = fmt.Sprintf("%s [%s]", r.IssueKey, r.Status)
		}
		e.printf("  %s [%s] waits on %s\n", b.Issue.IssueKey, b.Issue.Status, strings.Join(by, ", "))
	}
}

var statusOrder = []string{"todo", "in_progress", "review", "testing", "done", "closed", "suspended", "rejected", "duplicate"}

func cmdEpicCreate(e *env, args []string) error {
	// An epic is created like any issue, with the type fixed.
	for _, a := range args {
		if a == "--type" || strings.HasPrefix(a, "--type=") {
			return fmt.Errorf("mf epic create sets --type epic itself; drop --type")
		}
	}
	return cmdIssueCreate(e, append(append([]string{}, args...), "--type", model.IssueTypeEpic))
}

func (e *env) setEpic(issueKey, epic string) (*model.SetEpicResult, []byte, error) {
	var res model.SetEpicResult
	raw, err := e.client.put("/api/v1/issues/"+issueKey+"/epic", model.SetEpicRequest{Epic: epic}, &res)
	if err != nil {
		return nil, nil, epicRouteMissing(err)
	}
	return &res, raw, nil
}

func cmdEpicAdd(e *env, args []string) error {
	fs := newFlagSet("epic add")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(pos) < 2 {
		return fmt.Errorf("usage: mf epic add <EPIC_KEY> <ISSUE_KEY>…   (an issue moves if it is in another epic)")
	}
	epic := strings.ToUpper(pos[0])
	for _, key := range pos[1:] {
		res, raw, err := e.setEpic(key, epic)
		if err != nil {
			return err
		}
		if e.emitJSON(raw) {
			continue
		}
		switch {
		case res.Previous == nil:
			e.printf("%s: added to %s\n", res.Issue.IssueKey, epic)
		case strings.EqualFold(res.Previous.IssueKey, epic):
			e.printf("%s: already in %s\n", res.Issue.IssueKey, epic)
		default:
			e.printf("%s: moved from %s to %s\n", res.Issue.IssueKey, res.Previous.IssueKey, epic)
		}
	}
	return nil
}

func cmdEpicRemove(e *env, args []string) error {
	fs := newFlagSet("epic rm")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(pos) < 1 {
		return fmt.Errorf("usage: mf epic rm <ISSUE_KEY>…   (take issues out of their epic)")
	}
	for _, key := range pos {
		res, raw, err := e.setEpic(key, "")
		if err != nil {
			return err
		}
		if e.emitJSON(raw) {
			continue
		}
		if res.Previous == nil {
			e.printf("%s: not in an epic\n", res.Issue.IssueKey)
		} else {
			e.printf("%s: removed from %s\n", res.Issue.IssueKey, res.Previous.IssueKey)
		}
	}
	return nil
}

func cmdEpicProgress(e *env, args []string) error {
	fs := newFlagSet("epic progress")
	days := fs.Int("trend", 0, "also show the daily created/done trend over N days")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "epic progress <EPIC_KEY> [--trend <DAYS>]"); err != nil {
		return err
	}
	d := *days
	if d <= 0 {
		d = 30
	}
	var p model.EpicProgress
	raw, err := e.client.get("/api/v1/epics/"+pos[0]+"/progress"+query(map[string]string{"days": strconv.Itoa(d)}), &p)
	if err != nil {
		return epicRouteMissing(err)
	}
	if e.emitJSON(raw) {
		return nil
	}
	c := p.Counts
	e.printf("%s: %d/%d done (%s)\n", strings.ToUpper(pos[0]), c.Done, c.Total, percent(c.DoneRatio))
	e.printf("  status    %s\n", sortedCounts(c.StatusCounts, statusOrder))
	e.printf("  priority  %s\n", sortedCounts(p.PriorityCounts, []string{"P0", "P1", "P2"}))
	e.printf("  type      %s\n", sortedCounts(p.TypeCounts, []string{"bug", "requirement"}))
	printBlocked(e, p.Blocked)
	if *days > 0 {
		e.printf("\nTrend (last %d days):\n", *days)
		rows := make([][]string, 0, len(p.Trend))
		for _, t := range p.Trend {
			rows = append(rows, []string{t.Date, strconv.Itoa(t.Created), strconv.Itoa(t.Done)})
		}
		table(e.out, []string{"DATE", "CREATED", "DONE"}, rows)
	}
	return nil
}

func cmdEpicClose(e *env, args []string) error {
	fs := newFlagSet("epic close")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "epic close <EPIC_KEY>"); err != nil {
		return err
	}
	d, _, err := e.fetchEpic(pos[0])
	if err != nil {
		return err
	}
	if open := d.Counts.Total - d.Counts.Done; open > 0 {
		e.printf("note: %d issue(s) of %s are not done yet\n", open, d.Epic.IssueKey)
	}
	_, err = e.transition(pos[0], "done", false)
	return err
}

// --- memories as seen from an issue ------------------------------------------

// fetchVisibleMemories returns an issue's own memories and its epic's, with
// the actor's read state. ok is false against a server without epics, so the
// caller can fall back to the issue's own memories.
func (e *env) fetchVisibleMemories(issueKey string) (own, epic []model.IssueMemory, ok bool, err error) {
	var all []model.IssueMemory
	_, err = e.client.get("/api/v1/issues/"+issueKey+"/memories"+query(map[string]string{"reader": e.actor}), &all)
	if err != nil {
		if strings.Contains(err.Error(), "404 page not found") {
			return nil, nil, false, nil
		}
		return nil, nil, false, err
	}
	for _, m := range all {
		if m.Scope == model.MemoryScopeEpic {
			epic = append(epic, m)
		} else {
			own = append(own, m)
		}
	}
	return own, epic, true, nil
}

func plainMemories(ms []model.IssueMemory) []model.MemoryResponse {
	out := make([]model.MemoryResponse, len(ms))
	for i, m := range ms {
		out[i] = m.MemoryResponse
	}
	return out
}

func printMemoryRows(e *env, memories []model.MemoryResponse) {
	rows := make([][]string, 0, len(memories))
	for _, m := range memories {
		rows = append(rows, []string{"  " + m.Type, truncate(m.Title, 46), truncate(m.Content, 50), m.ID.String()})
	}
	table(e.out, nil, rows)
}

func memoryMarker(m model.IssueMemory) string {
	switch {
	case m.Updated:
		return "updated"
	case m.Unread:
		return "new"
	}
	return ""
}

// printEpicMemories is the "Epic memories" block of `mf issue show`.
func printEpicMemories(e *env, issueKey string, ms []model.IssueMemory) {
	if len(ms) == 0 {
		return
	}
	changed := 0
	for _, m := range ms {
		if m.Updated || m.Unread {
			changed++
		}
	}
	head := fmt.Sprintf("\nEpic memories (from %s", ms[0].EpicKey)
	if changed > 0 {
		head += fmt.Sprintf(", %d new or changed since %s last read them", changed, e.actor)
	}
	e.printf("%s):\n", head)
	rows := make([][]string, 0, len(ms))
	for _, m := range ms {
		marker := memoryMarker(m)
		if marker != "" {
			marker = "*" + marker
		}
		rows = append(rows, []string{"  " + m.Type, truncate(m.Title, 40), truncate(m.Content, 44), marker, m.ID.String()})
	}
	table(e.out, nil, rows)
	if changed > 0 {
		e.printf("  read them in full with: mf issue context %s\n", issueKey)
	}
}

func (e *env) markMemoriesRead(ms []model.IssueMemory) error {
	if len(ms) == 0 {
		return nil
	}
	marks := make([]model.MemoryReadMark, len(ms))
	for i, m := range ms {
		marks[i] = model.MemoryReadMark{MemoryID: m.ID, Revision: m.Revision}
	}
	_, err := e.client.post("/api/v1/memories/read", model.MarkMemoriesReadRequest{Reader: e.actor, Memories: marks}, nil)
	return err
}

// cmdIssueContext is the pick-up command: everything an issue can draw on —
// the issue, its epic, the epic's shared memories, its own memories, assets
// and the comment thread — in one read. Shared memories and comments shown
// here are marked read for the actor.
func cmdIssueContext(e *env, args []string) error {
	fs := newFlagSet("issue context")
	start := fs.Bool("start", false, "also move the issue to in_progress")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "issue context <ISSUE_KEY> [--start]"); err != nil {
		return err
	}
	key := pos[0]
	issue, _, err := e.fetchIssue(key)
	if err != nil {
		return err
	}
	assets, _, _ := e.fetchAssets(key)
	comments, _, _ := e.fetchComments(key)
	own, shared, ok, err := e.fetchVisibleMemories(key)
	if err != nil {
		return err
	}
	if !ok {
		plain, _ := e.fetchIssueMemories(issue.ID)
		for _, m := range plain {
			own = append(own, model.IssueMemory{MemoryResponse: m, Scope: model.MemoryScopeIssue})
		}
	}
	var epic *model.EpicDetail
	var epicAssets []model.IssueAsset
	if issue.Epic != nil {
		if epic, _, err = e.fetchEpic(issue.Epic.IssueKey); err != nil {
			return err
		}
		epicAssets, _, _ = e.fetchAssets(issue.Epic.IssueKey)
	}

	if e.asJSON {
		out := map[string]any{
			"issue": issue, "assets": nonNil(assets), "comments": nonNil(comments),
			"memories": nonNil(own), "epic_memories": nonNil(shared),
		}
		if epic != nil {
			out["epic"] = epic
			out["epic_assets"] = nonNil(epicAssets)
		}
		b, _ := json.Marshal(map[string]any{"data": out})
		e.printf("%s\n", b)
	} else {
		if issue.Description != nil {
			annotated := annotateAssetRefs(*issue.Description, assets)
			issue.Description = &annotated
		}
		e.printf("=== %s ===\n", issue.IssueKey)
		printIssue(e.out, issue)
		printAssets(e, assets)
		if len(assets) > 0 {
			e.printf("  pull them with: mf asset get %s --all -o ./.work/%s\n", key, key)
		}
		if epic != nil {
			c := epic.Counts
			e.printf("\n=== Epic %s: %s [%s] — %d/%d done ===\n", epic.Epic.IssueKey, epic.Epic.Title, epic.Epic.Status, c.Done, c.Total)
			if desc := strings.TrimSpace(deref(epic.Epic.Description)); desc != "" {
				e.printf("\n%s\n", desc)
			}
			printEpicSiblings(e, issue.IssueKey, epic)
			printAssets(e, epicAssets)
			if len(epicAssets) > 0 {
				e.printf("  pull them with: mf asset get %s --all -o ./.work/%s\n", epic.Epic.IssueKey, epic.Epic.IssueKey)
			}
			e.printf("\n=== Epic memories (%d) ===\n", len(shared))
			if len(shared) == 0 {
				e.printf("none yet — record decisions every issue of the epic needs with: mf memory add --epic %s …\n", epic.Epic.IssueKey)
			}
			for _, m := range shared {
				e.printf("\n")
				if marker := memoryMarker(m); marker != "" {
					e.printf("[%s] ", strings.ToUpper(marker))
				}
				printMemory(e, &m.MemoryResponse)
			}
		}
		e.printf("\n=== Memories on %s (%d) ===\n", issue.IssueKey, len(own))
		for _, m := range own {
			e.printf("\n")
			printMemory(e, &m.MemoryResponse)
		}
		if len(comments) > 0 {
			printComments(e, key, comments)
		} else {
			e.printf("\nNo comments.\n")
		}
	}

	if err := e.markMemoriesRead(shared); err != nil {
		return err
	}
	if len(comments) > 0 {
		if err := e.markCommentsRead(key); err != nil {
			return err
		}
	}
	if *start && issue.Status != "in_progress" {
		if _, err := e.transition(key, "in_progress", false); err != nil {
			return err
		}
	}
	return nil
}

// printEpicSiblings shows where this issue sits in its epic.
func printEpicSiblings(e *env, self string, d *model.EpicDetail) {
	if len(d.Children) <= 1 {
		return
	}
	e.printf("\nOther issues in the epic:\n")
	rows := [][]string{}
	for _, ch := range d.Children {
		if ch.IssueKey == self {
			continue
		}
		rows = append(rows, []string{"  " + ch.IssueKey, ch.Status, dash(ch.AssigneeID), truncate(ch.Title, 60)})
	}
	table(e.out, nil, rows)
	for _, b := range d.Blocked {
		if b.Issue.IssueKey == self {
			by := make([]string, len(b.BlockedBy))
			for i, r := range b.BlockedBy {
				by[i] = fmt.Sprintf("%s [%s]", r.IssueKey, r.Status)
			}
			e.printf("\n⚠ %s waits on %s (critical dependency not done)\n", self, strings.Join(by, ", "))
		}
	}
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// --- epic write-back -----------------------------------------------------------

// printConflicts explains a refused epic memory and how to proceed.
func printConflicts(e *env, err error, retry string) error {
	he, ok := asHTTPError(err, 409)
	if !ok {
		return err
	}
	var body struct {
		Error     string                 `json:"error"`
		Conflicts []model.MemoryConflict `json:"conflicts"`
	}
	if json.Unmarshal(he.Raw, &body) != nil || len(body.Conflicts) == 0 {
		return err
	}
	fmt.Fprintf(e.errOut, "%s\n\n", body.Error)
	for _, c := range body.Conflicts {
		fmt.Fprintf(e.errOut, "  %s  %.0f%% alike  %s\n", c.ID, c.Similarity*100, truncate(c.Title, 60))
		fmt.Fprintf(e.errOut, "      %s\n", truncate(c.Content, 90))
	}
	fmt.Fprintf(e.errOut, "\n  same decision, new wording: mf memory update <ID> --content …\n")
	fmt.Fprintf(e.errOut, "  genuinely separate:         %s\n", retry)
	return fmt.Errorf("not recorded: it seems to restate an existing epic memory")
}

func cmdMemoryPromote(e *env, args []string) error {
	fs := newFlagSet("memory promote")
	copyIt := fs.Bool("copy", false, "keep the original on the issue as well")
	force := fs.Bool("force", false, "promote even if the epic already has a similar memory")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "memory promote <MEMORY_ID> [--copy] [--force]   (lift an issue memory to its epic)"); err != nil {
		return err
	}
	var res struct {
		Memory model.MemoryResponse `json:"memory"`
		Epic   string               `json:"epic"`
	}
	raw, err := e.client.post("/api/v1/memories/"+pos[0]+"/promote", model.PromoteMemoryRequest{Copy: *copyIt, Force: *force}, &res)
	if err != nil {
		return printConflicts(e, epicRouteMissing(err), "mf memory promote "+pos[0]+" --force")
	}
	if e.emitJSON(raw) {
		return nil
	}
	verb := "moved"
	if *copyIt {
		verb = "copied"
	}
	e.printf("%s memory to epic %s: %s (%s)\n", verb, res.Epic, res.Memory.Title, res.Memory.ID)
	return nil
}

// cmdIssueMemories is `mf memory search --issue KEY`: what the issue can see,
// labeled by where each memory comes from.
func cmdIssueMemories(e *env, key string, full bool) error {
	own, shared, ok, err := e.fetchVisibleMemories(key)
	if err != nil {
		return err
	}
	if !ok {
		issue, _, err := e.fetchIssue(key)
		if err != nil {
			return err
		}
		plain, err := e.fetchIssueMemories(issue.ID)
		if err != nil {
			return err
		}
		for _, m := range plain {
			own = append(own, model.IssueMemory{MemoryResponse: m, Scope: model.MemoryScopeIssue})
		}
	}
	all := append(append([]model.IssueMemory{}, own...), shared...)
	if e.asJSON {
		b, _ := json.Marshal(map[string]any{"data": nonNil(all)})
		e.printf("%s\n", b)
		return nil
	}
	if len(all) == 0 {
		e.printf("no memories on %s or its epic\n", key)
		return nil
	}
	if full {
		for i, m := range all {
			if i > 0 {
				e.printf("\n---\n\n")
			}
			e.printf("[%s] ", scopeLabel(key, m))
			printMemory(e, &m.MemoryResponse)
		}
		return e.markMemoriesRead(shared)
	}
	rows := make([][]string, 0, len(all))
	for _, m := range all {
		rows = append(rows, []string{m.ID.String(), scopeLabel(key, m), m.Type, truncate(m.Title, 46), truncate(m.Content, 50), memoryMarker(m)})
	}
	table(e.out, []string{"ID", "FROM", "TYPE", "TITLE", "CONTENT", ""}, rows)
	return nil
}

func scopeLabel(key string, m model.IssueMemory) string {
	if m.Scope == model.MemoryScopeEpic {
		return "epic " + m.EpicKey
	}
	return strings.ToUpper(key)
}
