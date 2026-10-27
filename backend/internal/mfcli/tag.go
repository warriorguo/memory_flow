package mfcli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

func cmdTagList(e *env, args []string) error {
	fs := newFlagSet("tags")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}

	tags, raw, err := e.fetchTags()
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	if len(tags) == 0 {
		e.printf("no tags defined\n")
		return nil
	}
	rows := make([][]string, 0, len(tags))
	for _, t := range tags {
		name := t.Name
		if e.color && t.Color != nil {
			name = colorLabel(*t.Color, " "+t.Name+" ")
		}
		rows = append(rows, []string{name, dash(t.Color), t.ID.String()})
	}
	header := []string{"NAME", "COLOR", "ID"}
	if e.color {
		ansiTable(e.out, header, rows)
	} else {
		table(e.out, header, rows)
	}
	return nil
}

// cmdTagUpdate renames and/or recolors a tag, addressed by name or id.
func cmdTagUpdate(e *env, args []string) error {
	fs := newFlagSet("tag update")
	color := fs.String("color", "", "new hex color, e.g. #1890ff")
	name := fs.String("name", "", "new name")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "tag update <NAME> [--color '#1890ff'] [--name NEW_NAME]"); err != nil {
		return err
	}
	if *color == "" && *name == "" {
		return fmt.Errorf("nothing to update: pass --color and/or --name")
	}

	var req model.UpdateTagRequest
	setIfNonEmpty(&req.Color, *color)
	setIfNonEmpty(&req.Name, *name)

	var tag model.Tag
	raw, err := e.client.patch("/api/v1/tags/"+url.PathEscape(pos[0]), req, &tag)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	e.printf("updated tag %s%s (%s)\n", tag.Name, e.tagLabels([]model.Tag{tag}), dash(tag.Color))
	return nil
}

func cmdTagCreate(e *env, args []string) error {
	fs := newFlagSet("tag create")
	color := fs.String("color", "", "hex color, e.g. #1890ff")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "tag create <NAME> [--color '#1890ff']"); err != nil {
		return err
	}

	req := model.CreateTagRequest{Name: pos[0]}
	setIfNonEmpty(&req.Color, *color)

	var tag model.Tag
	raw, err := e.client.post("/api/v1/tags", req, &tag)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	e.printf("created tag %s (%s, %s)\n", tag.Name, dash(tag.Color), tag.ID)
	return nil
}

// cmdIssueTag attaches tags by name, creating any that do not exist yet so a
// caller never has to look up (or invent) tag UUIDs.
func cmdIssueTag(e *env, args []string) error {
	fs := newFlagSet("issue tag")
	color := fs.String("color", "", "color for tags this command has to create")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(pos) < 2 {
		return fmt.Errorf("usage: mf issue tag <ISSUE_KEY> <TAG_NAME>…")
	}

	for _, name := range pos[1:] {
		tag, err := e.findOrCreateTag(name, *color)
		if err != nil {
			return err
		}
		if _, err := e.client.post("/api/v1/issues/"+pos[0]+"/tags", model.AddTagRequest{TagID: tag.ID}, nil); err != nil {
			return err
		}
	}
	if !e.asJSON {
		e.printf("tagged %s: %s\n", pos[0], strings.Join(pos[1:], ", "))
	}
	return nil
}

func cmdIssueUntag(e *env, args []string) error {
	fs := newFlagSet("issue untag")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 2, "issue untag <ISSUE_KEY> <TAG_NAME>"); err != nil {
		return err
	}

	tagID := pos[1]
	if _, err := uuid.Parse(tagID); err != nil {
		tag, err := e.findTag(pos[1])
		if err != nil {
			return err
		}
		if tag == nil {
			return fmt.Errorf("no tag named %q", pos[1])
		}
		tagID = tag.ID.String()
	}

	if _, err := e.client.del("/api/v1/issues/" + pos[0] + "/tags/" + tagID); err != nil {
		return err
	}
	if !e.asJSON {
		e.printf("removed tag %s from %s\n", pos[1], pos[0])
	}
	return nil
}

func (e *env) fetchTags() ([]model.Tag, []byte, error) {
	var tags []model.Tag
	raw, err := e.client.get("/api/v1/tags", &tags)
	if err != nil {
		return nil, nil, err
	}
	return tags, raw, nil
}

func (e *env) findTag(name string) (*model.Tag, error) {
	tags, _, err := e.fetchTags()
	if err != nil {
		return nil, err
	}
	for i := range tags {
		if strings.EqualFold(tags[i].Name, name) {
			return &tags[i], nil
		}
	}
	return nil, nil
}

func (e *env) findOrCreateTag(name, color string) (*model.Tag, error) {
	tag, err := e.findTag(name)
	if err != nil {
		return nil, err
	}
	if tag != nil {
		return tag, nil
	}

	req := model.CreateTagRequest{Name: name}
	setIfNonEmpty(&req.Color, color)
	var created model.Tag
	if _, err := e.client.post("/api/v1/tags", req, &created); err != nil {
		return nil, fmt.Errorf("create tag %q: %w", name, err)
	}
	return &created, nil
}
