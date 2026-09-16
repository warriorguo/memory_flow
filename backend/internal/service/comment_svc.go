package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
)

// MaxCommentBytes caps one comment. Comments are discussion, not attachments:
// anything longer belongs in an asset or a memory.
const MaxCommentBytes = 16 << 10 // 16KB

// ErrEmptyComment is returned for a comment whose body is blank.
var ErrEmptyComment = errors.New("comment body is required")

// ErrCommentTooLong is returned for a body over [MaxCommentBytes].
var ErrCommentTooLong = errors.New("comment too long")

// ErrReaderRequired is returned when an operation that is inherently per-reader
// (marking comments read) is called without one.
var ErrReaderRequired = errors.New("reader is required")

type CommentService struct {
	repo repository.CommentRepository
}

func NewCommentService(repo repository.CommentRepository) *CommentService {
	return &CommentService{repo: repo}
}

// Create appends a comment after trimming and length-checking the body.
func (s *CommentService) Create(ctx context.Context, issueID uuid.UUID, req model.CreateCommentRequest) (*model.IssueComment, error) {
	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" {
		return nil, ErrEmptyComment
	}
	if len(req.Body) > MaxCommentBytes {
		return nil, fmt.Errorf("%w: %d bytes exceeds the %d byte limit", ErrCommentTooLong, len(req.Body), MaxCommentBytes)
	}
	if req.AuthorID != nil {
		if author := strings.TrimSpace(*req.AuthorID); author == "" {
			req.AuthorID = nil
		} else {
			req.AuthorID = &author
		}
	}
	return s.repo.Create(ctx, issueID, req)
}

// List returns an issue's comments, flagging the ones reader has not seen. An
// empty reader means "nobody in particular": every comment comes back read,
// because unread is only meaningful relative to someone.
func (s *CommentService) List(ctx context.Context, issueID uuid.UUID, reader string) ([]model.IssueComment, error) {
	comments, err := s.repo.ListByIssueID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	reader = strings.TrimSpace(reader)
	if reader == "" || len(comments) == 0 {
		return comments, nil
	}

	read, err := s.repo.ReadIDs(ctx, issueID, reader)
	if err != nil {
		return nil, err
	}
	for i := range comments {
		comments[i].Unread = isUnread(comments[i], read, reader)
	}
	return comments, nil
}

// isUnread reports whether reader still has to read this comment. Your own
// comments are never unread to you — you wrote them.
func isUnread(c model.IssueComment, read map[uuid.UUID]bool, reader string) bool {
	if c.AuthorID != nil && *c.AuthorID == reader {
		return false
	}
	return !read[c.ID]
}

// Summarize is the "you have mail" line for an issue: how many comments it
// carries and how many of them reader has not seen.
func (s *CommentService) Summarize(ctx context.Context, issueID uuid.UUID, reader string) (*model.CommentSummary, error) {
	comments, err := s.List(ctx, issueID, reader)
	if err != nil {
		return nil, err
	}
	return SummarizeComments(comments), nil
}

// SummarizeComments folds an already-fetched, already-flagged comment list into
// a summary, so a caller that needs both does not fetch twice.
func SummarizeComments(comments []model.IssueComment) *model.CommentSummary {
	summary := &model.CommentSummary{Total: len(comments)}
	for _, c := range comments {
		if c.Unread {
			summary.Unread++
		}
	}
	if len(comments) > 0 {
		last := comments[len(comments)-1]
		summary.LastCommentAt = &last.CreatedAt
		summary.LastAuthor = last.AuthorID
	}
	return summary
}

// MarkRead files a read receipt for every comment on the issue and reports the
// resulting state. Marking an issue with no comments is a no-op, not an error —
// there was simply nothing to read.
func (s *CommentService) MarkRead(ctx context.Context, issueID uuid.UUID, reader string) (*model.CommentSummary, error) {
	reader = strings.TrimSpace(reader)
	if reader == "" {
		return nil, ErrReaderRequired
	}
	if err := s.repo.MarkRead(ctx, issueID, reader); err != nil {
		return nil, err
	}
	return s.Summarize(ctx, issueID, reader)
}

func (s *CommentService) Delete(ctx context.Context, issueID, commentID uuid.UUID) error {
	return s.repo.Delete(ctx, issueID, commentID)
}
