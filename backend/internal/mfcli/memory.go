package mfcli

import (
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

func cmdMemoryAdd(e *env, args []string) error {
	fs := newFlagSet("memory add")
	memType := fs.String("type", "recall", "recall (reusable context) or write (produced artifact)")
	title := fs.String("title", "", "memory title (required)")
	content := fs.String("content", "", "memory content")
	contentFile := fs.String("content-file", "", "read content from file ('-' for stdin)")
	project := fs.String("project", "", "project key to attach the memory to")
	issue := fs.String("issue", "", "issue key this memory came out of")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	// `mf memory add "Title" --content …` is accepted as shorthand for --title.
	if len(pos) > 0 && *title == "" {
		*title = pos[0]
		pos = pos[1:]
	}
	if err := need(pos, 0, `memory add --title "…" --content "…" [--project KEY] [--type recall|write]`); err != nil {
		return err
	}
	if *title == "" {
		return fmt.Errorf("--title is required")
	}

	body, err := readContent(*content, *contentFile)
	if err != nil {
		return err
	}
	if body == "" {
		return fmt.Errorf("--content or --content-file is required")
	}

	req := model.CreateMemoryRequest{Type: *memType, Title: *title, Content: body}

	// An issue reference implies its project, so --issue alone is enough.
	if *issue != "" {
		target, _, err := e.fetchIssue(*issue)
		if err != nil {
			return err
		}
		sourceType := target.Type
		req.SourceObjectType = &sourceType
		req.SourceObjectID = &target.ID
		if *project == "" {
			req.ProjectID = &target.ProjectID
		}
	}
	if *project != "" {
		var p model.Project
		if _, err := e.client.get("/api/v1/projects/"+*project, &p); err != nil {
			return err
		}
		req.ProjectID = &p.ID
		if req.SourceObjectType == nil {
			sourceType := "project"
			req.SourceObjectType = &sourceType
			req.SourceObjectID = &p.ID
		}
	}

	var memory model.MemoryResponse
	raw, err := e.client.post("/api/v1/memories", req, &memory)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	e.printf("recorded %s memory %s: %s\n", memory.Type, memory.ID, memory.Title)
	return nil
}

func cmdMemoryList(e *env, args []string) error {
	fs := newFlagSet("memory search")
	memType := fs.String("type", "", "recall or write")
	keyword := fs.String("keyword", "", "search titles and content")
	project := fs.String("project", "", "project key")
	limit := fs.Int("limit", 20, "max memories to show")
	page := fs.Int("page", 1, "page number")
	full := fs.Bool("full", false, "print full content instead of a table")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	// `mf memory search "some words"` is accepted as shorthand for --keyword.
	if len(pos) > 0 && *keyword == "" {
		*keyword = pos[0]
		pos = pos[1:]
	}
	if err := need(pos, 0, `memory search "<KEYWORD>" [--project KEY] [--type recall|write]`); err != nil {
		return err
	}

	projectID := ""
	if *project != "" {
		var p model.Project
		if _, err := e.client.get("/api/v1/projects/"+*project, &p); err != nil {
			return err
		}
		projectID = p.ID.String()
	}

	var memories []model.MemoryResponse
	raw, err := e.client.get("/api/v1/memories"+query(map[string]string{
		"project_id": projectID,
		"type":       *memType,
		"keyword":    *keyword,
		"page":       strconv.Itoa(*page),
		"page_size":  strconv.Itoa(*limit),
	}), &memories)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	if len(memories) == 0 {
		e.printf("no memories match\n")
		return nil
	}

	if *full {
		for i, m := range memories {
			if i > 0 {
				e.printf("\n---\n\n")
			}
			printMemory(e, &m)
		}
		return nil
	}

	rows := make([][]string, 0, len(memories))
	for _, m := range memories {
		projectKey := "-"
		if m.Project != nil {
			projectKey = m.Project.Key
		}
		rows = append(rows, []string{
			m.ID.String(), projectKey, m.Type, truncate(m.Title, 50), truncate(m.Content, 60),
		})
	}
	table(e.out, []string{"ID", "PROJECT", "TYPE", "TITLE", "CONTENT"}, rows)
	e.printf("\n%d shown of %d matching\n", len(memories), listTotal(raw))
	return nil
}

func cmdMemoryShow(e *env, args []string) error {
	fs := newFlagSet("memory show")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "memory show <MEMORY_ID>"); err != nil {
		return err
	}

	var m model.MemoryResponse
	raw, err := e.client.get("/api/v1/memories/"+pos[0], &m)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	printMemory(e, &m)
	return nil
}

func cmdMemoryUpdate(e *env, args []string) error {
	fs := newFlagSet("memory update")
	memType := fs.String("type", "", "recall or write")
	title := fs.String("title", "", "memory title")
	content := fs.String("content", "", "memory content")
	contentFile := fs.String("content-file", "", "read content from file ('-' for stdin)")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, `memory update <MEMORY_ID> [--title "…"] [--content "…"]`); err != nil {
		return err
	}

	req := model.UpdateMemoryRequest{
		Type:  strPtr(fs, "type", memType),
		Title: strPtr(fs, "title", title),
	}
	if flagWasSet(fs, "content") || flagWasSet(fs, "content-file") {
		body, err := readContent(*content, *contentFile)
		if err != nil {
			return err
		}
		req.Content = &body
	}
	if req == (model.UpdateMemoryRequest{}) {
		return fmt.Errorf("nothing to update: pass --title, --content, --content-file or --type")
	}

	var m model.MemoryResponse
	raw, err := e.client.put("/api/v1/memories/"+pos[0], req, &m)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	e.printf("updated memory %s\n", m.ID)
	return nil
}

func cmdMemoryRemove(e *env, args []string) error {
	fs := newFlagSet("memory rm")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "memory rm <MEMORY_ID>"); err != nil {
		return err
	}
	if _, err := uuid.Parse(pos[0]); err != nil {
		return fmt.Errorf("%q is not a memory id (memories are identified by UUID — find one with `mf memory search`)", pos[0])
	}
	raw, err := e.client.del("/api/v1/memories/" + pos[0])
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	e.printf("deleted memory %s\n", pos[0])
	return nil
}

func printMemory(e *env, m *model.MemoryResponse) {
	e.printf("%s  [%s]\n", m.Title, m.Type)
	e.printf("  id       %s\n", m.ID)
	if m.Project != nil {
		e.printf("  project  %s (%s)\n", m.Project.Key, m.Project.Name)
	}
	if m.SourceObjectType != nil {
		e.printf("  source   %s %s\n", *m.SourceObjectType, uuidOrDash(m.SourceObjectID))
	}
	e.printf("  created  %s\n", shortTime(m.CreatedAt))
	e.printf("\n%s\n", m.Content)
}

func uuidOrDash(id *uuid.UUID) string {
	if id == nil {
		return "-"
	}
	return id.String()
}
