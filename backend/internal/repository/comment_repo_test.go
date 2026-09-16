package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

func newCommentRepo(t *testing.T) (*CommentRepo, database.DB, uuid.UUID) {
	t.Helper()
	db := newTestDB(t)
	return NewCommentRepo(db), db, seedIssue(t, db, "MF-1")
}

// stampComment forces a comment's created_at, because SQLite's CURRENT_TIMESTAMP
// has second resolution and everything here is written within the same second.
func stampComment(t *testing.T, db database.DB, id uuid.UUID, at string) {
	t.Helper()
	if _, err := db.Exec(context.Background(),
		`UPDATE issue_comments SET created_at = $1 WHERE id = $2`, at, id); err != nil {
		t.Fatalf("stamp comment: %v", err)
	}
}

func author(s string) *string { return &s }

func TestCommentCreateAndList(t *testing.T) {
	r, db, issueID := newCommentRepo(t)
	ctx := context.Background()

	second, err := r.Create(ctx, issueID, model.CreateCommentRequest{Body: "second", AuthorID: author("bob")})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	first, err := r.Create(ctx, issueID, model.CreateCommentRequest{Body: "first", AuthorID: author("alice")})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	stampComment(t, db, first.ID, "2026-08-01 09:00:00")
	stampComment(t, db, second.ID, "2026-08-01 10:00:00")

	comments, err := r.ListByIssueID(ctx, issueID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("got %d comments, want 2", len(comments))
	}
	// Oldest first — reading order, not insertion order.
	if comments[0].Body != "first" || comments[1].Body != "second" {
		t.Errorf("order = %q, %q; want first, second", comments[0].Body, comments[1].Body)
	}
	if comments[0].AuthorID == nil || *comments[0].AuthorID != "alice" {
		t.Errorf("author = %v, want alice", comments[0].AuthorID)
	}

	// A comment on another issue must not leak into this issue's thread.
	other := seedIssue(t, db, "MF-2")
	if _, err := r.Create(ctx, other, model.CreateCommentRequest{Body: "elsewhere"}); err != nil {
		t.Fatalf("create on other issue: %v", err)
	}
	if comments, _ := r.ListByIssueID(ctx, issueID); len(comments) != 2 {
		t.Errorf("got %d comments after seeding another issue, want 2", len(comments))
	}
}

func TestCommentReadReceiptsArePerReader(t *testing.T) {
	r, _, issueID := newCommentRepo(t)
	ctx := context.Background()

	// Nothing read yet.
	read, err := r.ReadIDs(ctx, issueID, "alice")
	if err != nil {
		t.Fatalf("read ids: %v", err)
	}
	if len(read) != 0 {
		t.Fatalf("read ids = %v, want none", read)
	}

	first, err := r.Create(ctx, issueID, model.CreateCommentRequest{Body: "first", AuthorID: author("bob")})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := r.MarkRead(ctx, issueID, "alice"); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	// A second comment lands within the same second as alice's receipt. This is
	// the case a "last read at" watermark cannot represent on SQLite, whose
	// CURRENT_TIMESTAMP only has second resolution: the comment must still be
	// unread.
	second, err := r.Create(ctx, issueID, model.CreateCommentRequest{Body: "second", AuthorID: author("bob")})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	read, err = r.ReadIDs(ctx, issueID, "alice")
	if err != nil {
		t.Fatalf("read ids: %v", err)
	}
	if !read[first.ID] {
		t.Errorf("first comment has no receipt for alice")
	}
	if read[second.ID] {
		t.Errorf("second comment counted as read; it arrived after alice's receipt")
	}

	// Bob has read neither: receipts do not leak between readers.
	if bobRead, _ := r.ReadIDs(ctx, issueID, "bob"); len(bobRead) != 0 {
		t.Errorf("bob's receipts = %v, want none", bobRead)
	}

	// Marking again is idempotent and picks up what has since arrived.
	if err := r.MarkRead(ctx, issueID, "alice"); err != nil {
		t.Fatalf("second mark read: %v", err)
	}
	read, _ = r.ReadIDs(ctx, issueID, "alice")
	if len(read) != 2 {
		t.Errorf("receipts = %d, want 2", len(read))
	}
}

// Receipts are scoped to the issue's own comments, so a reader who cleared one
// issue is not reported as having read another.
func TestCommentReadIDsAreScopedToTheIssue(t *testing.T) {
	r, db, issueID := newCommentRepo(t)
	ctx := context.Background()
	other := seedIssue(t, db, "MF-9")

	if _, err := r.Create(ctx, issueID, model.CreateCommentRequest{Body: "here"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := r.Create(ctx, other, model.CreateCommentRequest{Body: "there"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := r.MarkRead(ctx, issueID, "alice"); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	if read, _ := r.ReadIDs(ctx, issueID, "alice"); len(read) != 1 {
		t.Errorf("receipts on the marked issue = %d, want 1", len(read))
	}
	if read, _ := r.ReadIDs(ctx, other, "alice"); len(read) != 0 {
		t.Errorf("receipts on the other issue = %d, want 0", len(read))
	}
}

func TestCommentDelete(t *testing.T) {
	r, db, issueID := newCommentRepo(t)
	ctx := context.Background()

	comment, err := r.Create(ctx, issueID, model.CreateCommentRequest{Body: "oops"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// The wrong issue must not be able to delete another issue's comment.
	other := seedIssue(t, db, "MF-3")
	if err := r.Delete(ctx, other, comment.ID); !errors.Is(err, ErrCommentNotFound) {
		t.Errorf("cross-issue delete error = %v, want ErrCommentNotFound", err)
	}

	if err := r.MarkRead(ctx, issueID, "alice"); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	if err := r.Delete(ctx, issueID, comment.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// The receipt goes with the comment rather than lingering as an orphan row.
	if read, _ := r.ReadIDs(ctx, issueID, "alice"); len(read) != 0 {
		t.Errorf("receipts after delete = %v, want none", read)
	}
	if comments, _ := r.ListByIssueID(ctx, issueID); len(comments) != 0 {
		t.Errorf("got %d comments after delete, want 0", len(comments))
	}
	if err := r.Delete(ctx, issueID, comment.ID); !errors.Is(err, ErrCommentNotFound) {
		t.Errorf("second delete error = %v, want ErrCommentNotFound", err)
	}
}
