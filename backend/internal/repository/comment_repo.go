package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

const commentColumns = `id, issue_id, author_id, body, created_at, updated_at`

// ErrCommentNotFound is returned when the issue has no comment under that id.
var ErrCommentNotFound = fmt.Errorf("comment not found")

type CommentRepo struct {
	db database.DB
}

func NewCommentRepo(db database.DB) *CommentRepo {
	return &CommentRepo{db: db}
}

func scanComment(row database.Row) (*model.IssueComment, error) {
	var c model.IssueComment
	if err := row.Scan(&c.ID, &c.IssueID, &c.AuthorID, &c.Body, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

// Create appends a comment to an issue.
func (r *CommentRepo) Create(ctx context.Context, issueID uuid.UUID, req model.CreateCommentRequest) (*model.IssueComment, error) {
	query := fmt.Sprintf(`
		INSERT INTO issue_comments (id, issue_id, author_id, body)
		VALUES ($1, $2, $3, $4)
		RETURNING %s`, commentColumns)

	comment, err := scanComment(r.db.QueryRow(ctx, query, uuid.New(), issueID, req.AuthorID, req.Body))
	if err != nil {
		return nil, fmt.Errorf("create comment: %w", err)
	}
	return comment, nil
}

// ListByIssueID returns an issue's comments oldest first — reading order. The id
// breaks ties, which keeps the order stable across queries; two comments written
// within the same clock tick (a millisecond on SQLite) sort arbitrarily but
// consistently. Read state does not depend on this order — see [CommentRepo.MarkRead].
func (r *CommentRepo) ListByIssueID(ctx context.Context, issueID uuid.UUID) ([]model.IssueComment, error) {
	query := fmt.Sprintf(`SELECT %s FROM issue_comments WHERE issue_id = $1 ORDER BY created_at, id`, commentColumns)
	rows, err := r.db.Query(ctx, query, issueID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	var comments []model.IssueComment
	for rows.Next() {
		var c model.IssueComment
		if err := rows.Scan(&c.ID, &c.IssueID, &c.AuthorID, &c.Body, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comments: %w", err)
	}
	return comments, nil
}

// Delete removes one comment from an issue. The issue id is part of the
// predicate so a comment id from another issue cannot be deleted through it.
func (r *CommentRepo) Delete(ctx context.Context, issueID, commentID uuid.UUID) error {
	res, err := r.db.Exec(ctx, `DELETE FROM issue_comments WHERE id = $1 AND issue_id = $2`, commentID, issueID)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrCommentNotFound
	}
	return nil
}

// ReadIDs returns the ids of this issue's comments that reader has already
// seen. Callers diff it against the thread rather than asking per comment.
func (r *CommentRepo) ReadIDs(ctx context.Context, issueID uuid.UUID, reader string) (map[uuid.UUID]bool, error) {
	rows, err := r.db.Query(ctx, `
		SELECT cr.comment_id
		FROM issue_comment_reads cr
		JOIN issue_comments c ON c.id = cr.comment_id
		WHERE c.issue_id = $1 AND cr.reader = $2`, issueID, reader)
	if err != nil {
		return nil, fmt.Errorf("list comment read receipts: %w", err)
	}
	defer rows.Close()

	read := map[uuid.UUID]bool{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan comment read receipt: %w", err)
		}
		read[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comment read receipts: %w", err)
	}
	return read, nil
}

// MarkRead files a receipt for every comment currently on the issue. Comments
// that arrive afterwards stay unread, and an issue with no comments is a no-op
// rather than an error.
func (r *CommentRepo) MarkRead(ctx context.Context, issueID uuid.UUID, reader string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO issue_comment_reads (comment_id, reader)
		SELECT id, $2 FROM issue_comments WHERE issue_id = $1
		ON CONFLICT DO NOTHING`, issueID, reader)
	if err != nil {
		return fmt.Errorf("mark comments read: %w", err)
	}
	return nil
}
