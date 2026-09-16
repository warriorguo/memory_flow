package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository/mocks"
)

func strp(s string) *string { return &s }

// thread builds three comments an hour apart, the middle one written by "alice".
func thread(base time.Time) []model.IssueComment {
	return []model.IssueComment{
		{ID: uuid.New(), Body: "first", AuthorID: strp("bob"), CreatedAt: base},
		{ID: uuid.New(), Body: "second", AuthorID: strp("alice"), CreatedAt: base.Add(time.Hour)},
		{ID: uuid.New(), Body: "third", AuthorID: strp("bob"), CreatedAt: base.Add(2 * time.Hour)},
	}
}

func TestListFlagsUnreceiptedComments(t *testing.T) {
	base := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	issueID := uuid.New()
	posted := thread(base)
	repo := &mocks.MockCommentRepo{
		ListByIssueIDFn: func(context.Context, uuid.UUID) ([]model.IssueComment, error) {
			return posted, nil
		},
		ReadIDsFn: func(context.Context, uuid.UUID, string) (map[uuid.UUID]bool, error) {
			// alice has a receipt for the first comment only.
			return map[uuid.UUID]bool{posted[0].ID: true}, nil
		},
	}
	svc := NewCommentService(repo)

	comments, err := svc.List(context.Background(), issueID, "alice")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// #1 has a receipt, #2 is alice's own, #3 is genuinely new.
	want := []bool{false, false, true}
	for i, w := range want {
		if comments[i].Unread != w {
			t.Errorf("comment %d unread = %v, want %v", i, comments[i].Unread, w)
		}
	}

	summary := SummarizeComments(comments)
	if summary.Total != 3 || summary.Unread != 1 {
		t.Errorf("summary = %d total / %d unread, want 3/1", summary.Total, summary.Unread)
	}
	if summary.LastAuthor == nil || *summary.LastAuthor != "bob" {
		t.Errorf("last author = %v, want bob", summary.LastAuthor)
	}
}

func TestListWithNoReceiptsIsAllUnreadExceptYourOwn(t *testing.T) {
	base := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	repo := &mocks.MockCommentRepo{
		ListByIssueIDFn: func(context.Context, uuid.UUID) ([]model.IssueComment, error) {
			return thread(base), nil
		},
		// No receipts: this reader has never opened the issue.
	}
	svc := NewCommentService(repo)

	comments, err := svc.List(context.Background(), uuid.New(), "alice")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got := SummarizeComments(comments).Unread; got != 2 {
		t.Errorf("unread = %d, want 2 (both of bob's, not alice's own)", got)
	}
}

func TestListWithoutReaderReportsNothingUnread(t *testing.T) {
	base := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	repo := &mocks.MockCommentRepo{
		ListByIssueIDFn: func(context.Context, uuid.UUID) ([]model.IssueComment, error) {
			return thread(base), nil
		},
		ReadIDsFn: func(context.Context, uuid.UUID, string) (map[uuid.UUID]bool, error) {
			t.Error("read receipts should not be consulted without a reader")
			return nil, nil
		},
	}
	svc := NewCommentService(repo)

	comments, err := svc.List(context.Background(), uuid.New(), "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got := SummarizeComments(comments).Unread; got != 0 {
		t.Errorf("unread = %d, want 0 — unread is only meaningful for someone", got)
	}
}

func TestCreateRejectsBlankAndOversizedBodies(t *testing.T) {
	repo := &mocks.MockCommentRepo{
		CreateFn: func(_ context.Context, _ uuid.UUID, req model.CreateCommentRequest) (*model.IssueComment, error) {
			return &model.IssueComment{Body: req.Body, AuthorID: req.AuthorID}, nil
		},
	}
	svc := NewCommentService(repo)
	ctx := context.Background()

	if _, err := svc.Create(ctx, uuid.New(), model.CreateCommentRequest{Body: "   \n"}); !errors.Is(err, ErrEmptyComment) {
		t.Errorf("blank body error = %v, want ErrEmptyComment", err)
	}

	long := model.CreateCommentRequest{Body: strings.Repeat("x", MaxCommentBytes+1)}
	if _, err := svc.Create(ctx, uuid.New(), long); !errors.Is(err, ErrCommentTooLong) {
		t.Errorf("oversized body error = %v, want ErrCommentTooLong", err)
	}

	comment, err := svc.Create(ctx, uuid.New(), model.CreateCommentRequest{Body: "  hello  ", AuthorID: strp(" ")})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if comment.Body != "hello" {
		t.Errorf("body = %q, want %q", comment.Body, "hello")
	}
	if comment.AuthorID != nil {
		t.Errorf("blank author = %v, want nil", *comment.AuthorID)
	}
}

func TestMarkReadRequiresAReader(t *testing.T) {
	marked := 0
	repo := &mocks.MockCommentRepo{
		ListByIssueIDFn: func(context.Context, uuid.UUID) ([]model.IssueComment, error) {
			return nil, nil
		},
		MarkReadFn: func(context.Context, uuid.UUID, string) error {
			marked++
			return nil
		},
	}
	svc := NewCommentService(repo)

	if _, err := svc.MarkRead(context.Background(), uuid.New(), "  "); !errors.Is(err, ErrReaderRequired) {
		t.Errorf("empty reader error = %v, want ErrReaderRequired", err)
	}
	if marked != 0 {
		t.Errorf("marked %d times without a reader, want 0", marked)
	}

	summary, err := svc.MarkRead(context.Background(), uuid.New(), "alice")
	if err != nil {
		t.Fatalf("mark read: %v", err)
	}
	if summary.Total != 0 || summary.Unread != 0 {
		t.Errorf("summary = %+v, want an empty thread", summary)
	}
}
