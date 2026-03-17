package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

type IssueHistoryRepo struct {
	pool *pgxpool.Pool
}

func NewIssueHistoryRepo(pool *pgxpool.Pool) *IssueHistoryRepo {
	return &IssueHistoryRepo{pool: pool}
}

func (r *IssueHistoryRepo) Create(ctx context.Context, tx pgx.Tx, issueID uuid.UUID, fieldName string, oldValue, newValue *string, operatorID *string) error {
	query := `
		INSERT INTO issue_history (issue_id, field_name, old_value, new_value, operator_id)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := tx.Exec(ctx, query, issueID, fieldName, oldValue, newValue, operatorID)
	if err != nil {
		return fmt.Errorf("create issue history: %w", err)
	}
	return nil
}

func (r *IssueHistoryRepo) ListByIssueID(ctx context.Context, issueID uuid.UUID) ([]model.IssueHistory, error) {
	query := `
		SELECT id, issue_id, field_name, old_value, new_value, operator_id, created_at
		FROM issue_history
		WHERE issue_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, issueID)
	if err != nil {
		return nil, fmt.Errorf("list issue history: %w", err)
	}
	defer rows.Close()

	var history []model.IssueHistory
	for rows.Next() {
		var h model.IssueHistory
		if err := rows.Scan(&h.ID, &h.IssueID, &h.FieldName, &h.OldValue, &h.NewValue, &h.OperatorID, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan issue history: %w", err)
		}
		history = append(history, h)
	}
	return history, rows.Err()
}
