package mfcli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

// commentPath builds the endpoint for an issue's comments.
func commentPath(issueKey, suffix string) string {
	base := "/api/v1/issues/" + url.PathEscape(issueKey) + "/comments"
	if suffix == "" {
		return base
	}
	return base + "/" + suffix
}

// cmdCommentAdd posts a comment on an issue.
func cmdCommentAdd(e *env, args []string) error {
	fs := newFlagSet("comment add")
	body := fs.String("body", "", "comment text")
	bodyFile := fs.String("body-file", "", "read the comment from file ('-' for stdin)")
	author := fs.String("author", "", "who is commenting (default: the current actor)")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	// `mf comment add MF-1 "text"` is accepted as shorthand for --body.
	if len(pos) > 1 && *body == "" {
		*body = pos[1]
		pos = pos[:1]
	}
	if err := need(pos, 1, `comment add <ISSUE_KEY> "…" [--body-file FILE] [--author ID]`); err != nil {
		return err
	}

	text, err := readContent(*body, *bodyFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("nothing to say: pass the comment text, or --body-file - to read stdin")
	}

	req := model.CreateCommentRequest{Body: text}
	who := *author
	if who == "" {
		who = e.actor
	}
	setIfNonEmpty(&req.AuthorID, who)

	var comment model.IssueComment
	raw, err := e.client.post(commentPath(pos[0], ""), req, &comment)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	e.printf("commented on %s as %s (%s)\n", pos[0], dash(comment.AuthorID), comment.ID)
	return nil
}

// cmdCommentList prints an issue's comments and, unless told otherwise, marks
// them read for the current actor — reading is what clears the unread flag.
func cmdCommentList(e *env, args []string) error {
	fs := newFlagSet("comment list")
	unreadOnly := fs.Bool("unread", false, "show only the comments you have not read")
	noMark := fs.Bool("no-mark", false, "do not mark the comments as read")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "comment list <ISSUE_KEY> [--unread] [--no-mark]"); err != nil {
		return err
	}
	key := pos[0]

	comments, raw, err := e.fetchComments(key)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		// --json is a read too, unless the caller opted out.
		if !*noMark {
			_ = e.markCommentsRead(key)
		}
		return nil
	}

	shown := comments
	if *unreadOnly {
		shown = shown[:0:0]
		for _, c := range comments {
			if c.Unread {
				shown = append(shown, c)
			}
		}
	}

	if len(shown) == 0 {
		if *unreadOnly && len(comments) > 0 {
			e.printf("no unread comments on %s (%d total)\n", key, len(comments))
		} else {
			e.printf("no comments on %s\n", key)
		}
		return nil
	}

	printComments(e, key, shown)
	if !*noMark {
		if err := e.markCommentsRead(key); err != nil {
			return err
		}
	}
	return nil
}

func cmdCommentRemove(e *env, args []string) error {
	fs := newFlagSet("comment rm")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 2, "comment rm <ISSUE_KEY> <COMMENT_ID> [--yes]"); err != nil {
		return err
	}
	issueKey, commentID := pos[0], pos[1]
	if _, err := uuid.Parse(commentID); err != nil {
		return fmt.Errorf("%q is not a comment id (comments are identified by UUID — find one with `mf comment list %s`)", commentID, issueKey)
	}

	if !*yes {
		ok, err := confirm(e, fmt.Sprintf("delete comment %s from %s?", commentID, issueKey))
		if err != nil {
			return err
		}
		if !ok {
			e.printf("cancelled\n")
			return nil
		}
	}

	raw, err := e.client.del(commentPath(issueKey, commentID))
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	e.printf("deleted comment %s from %s\n", commentID, issueKey)
	return nil
}

// --- shared comment helpers ------------------------------------------------

// fetchComments loads an issue's comments with unread flags for the current
// actor.
func (e *env) fetchComments(issueKey string) ([]model.IssueComment, []byte, error) {
	var comments []model.IssueComment
	raw, err := e.client.get(commentPath(issueKey, "")+query(map[string]string{"reader": e.actor}), &comments)
	if err != nil {
		return nil, nil, err
	}
	return comments, raw, nil
}

func (e *env) markCommentsRead(issueKey string) error {
	_, err := e.client.post(commentPath(issueKey, "read"), model.MarkCommentsReadRequest{Reader: e.actor}, nil)
	return err
}

// printComments renders a comment thread, newest last, with unread ones marked.
func printComments(e *env, issueKey string, comments []model.IssueComment) {
	e.printf("\nComments:\n")
	for i := range comments {
		if i > 0 {
			e.printf("\n")
		}
		printComment(e, &comments[i])
	}
	e.printf("\n%d comment(s) on %s\n", len(comments), issueKey)
}

func printComment(e *env, c *model.IssueComment) {
	marker := " "
	if c.Unread {
		marker = "*"
	}
	e.printf("%s %s  %s\n", marker, shortTime(c.CreatedAt), dash(c.AuthorID))
	e.printf("  %s\n", strings.ReplaceAll(strings.TrimSpace(c.Body), "\n", "\n  "))
	e.printf("  id %s\n", c.ID)
}

// printCommentHint is the line `mf issue show` prints so a reader learns there
// is discussion waiting without having to ask for it.
func printCommentHint(e *env, issueKey string, comments []model.IssueComment) {
	if len(comments) == 0 {
		return
	}
	unread := 0
	for _, c := range comments {
		if c.Unread {
			unread++
		}
	}
	last := comments[len(comments)-1]
	if unread == 0 {
		e.printf("\nComments: %d, none unread (latest %s by %s)\n",
			len(comments), shortTime(last.CreatedAt), dash(last.AuthorID))
		return
	}
	e.printf("\nComments: %d, %d unread for %s — read them with: mf comment list %s\n",
		len(comments), unread, e.actor, issueKey)
	// Lead with the newest unread one, so the hint is informative on its own.
	for i := len(comments) - 1; i >= 0; i-- {
		if comments[i].Unread {
			e.printf("  latest: [%s] %s\n", dash(comments[i].AuthorID), truncate(comments[i].Body, 72))
			break
		}
	}
}

// withComments splices the comment list into an issue's JSON envelope, so
// `mf issue show --json` describes the issue and its discussion in one object.
func withComments(raw []byte, comments []model.IssueComment) []byte {
	if comments == nil {
		comments = []model.IssueComment{}
	}
	return spliceIssueData(raw, "comments", comments)
}

// spliceIssueData adds one field to the `data` object of an API envelope,
// leaving the envelope untouched when it is not shaped the way we expect.
func spliceIssueData(raw []byte, field string, value any) []byte {
	var env map[string]json.RawMessage
	if err := json.Unmarshal(raw, &env); err != nil {
		return raw
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(env["data"], &data); err != nil {
		return raw
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return raw
	}
	data[field] = encoded
	if env["data"], err = json.Marshal(data); err != nil {
		return raw
	}
	merged, err := json.Marshal(env)
	if err != nil {
		return raw
	}
	return merged
}
