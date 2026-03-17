package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

type TagRepo struct {
	pool *pgxpool.Pool
}

func NewTagRepo(pool *pgxpool.Pool) *TagRepo {
	return &TagRepo{pool: pool}
}

func (r *TagRepo) Create(ctx context.Context, req model.CreateTagRequest) (*model.Tag, error) {
	query := `
		INSERT INTO tags (name, color)
		VALUES ($1, $2)
		RETURNING id, name, color, created_at`

	row := r.pool.QueryRow(ctx, query, req.Name, req.Color)

	var t model.Tag
	err := row.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create tag: %w", err)
	}
	return &t, nil
}

func (r *TagRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Tag, error) {
	query := `SELECT id, name, color, created_at FROM tags WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)

	var t model.Tag
	err := row.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get tag by id: %w", err)
	}
	return &t, nil
}

func (r *TagRepo) List(ctx context.Context) ([]model.Tag, error) {
	query := `SELECT id, name, color, created_at FROM tags ORDER BY name`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	var tags []model.Tag
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *TagRepo) AddToIssue(ctx context.Context, issueID, tagID uuid.UUID) error {
	query := `INSERT INTO issue_tag_rel (issue_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := r.pool.Exec(ctx, query, issueID, tagID)
	if err != nil {
		return fmt.Errorf("add tag to issue: %w", err)
	}
	return nil
}

func (r *TagRepo) RemoveFromIssue(ctx context.Context, issueID, tagID uuid.UUID) error {
	query := `DELETE FROM issue_tag_rel WHERE issue_id = $1 AND tag_id = $2`
	ct, err := r.pool.Exec(ctx, query, issueID, tagID)
	if err != nil {
		return fmt.Errorf("remove tag from issue: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("tag relation not found")
	}
	return nil
}

func (r *TagRepo) GetByIssueID(ctx context.Context, issueID uuid.UUID) ([]model.Tag, error) {
	query := `
		SELECT t.id, t.name, t.color, t.created_at
		FROM tags t
		JOIN issue_tag_rel itr ON t.id = itr.tag_id
		WHERE itr.issue_id = $1
		ORDER BY t.name`

	rows, err := r.pool.Query(ctx, query, issueID)
	if err != nil {
		return nil, fmt.Errorf("get tags by issue id: %w", err)
	}
	defer rows.Close()

	var tags []model.Tag
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *TagRepo) AddToMemory(ctx context.Context, memoryID, tagID uuid.UUID) error {
	query := `INSERT INTO memory_tag_rel (memory_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := r.pool.Exec(ctx, query, memoryID, tagID)
	if err != nil {
		return fmt.Errorf("add tag to memory: %w", err)
	}
	return nil
}

func (r *TagRepo) RemoveFromMemory(ctx context.Context, memoryID, tagID uuid.UUID) error {
	query := `DELETE FROM memory_tag_rel WHERE memory_id = $1 AND tag_id = $2`
	ct, err := r.pool.Exec(ctx, query, memoryID, tagID)
	if err != nil {
		return fmt.Errorf("remove tag from memory: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("tag relation not found")
	}
	return nil
}

func (r *TagRepo) GetByMemoryID(ctx context.Context, memoryID uuid.UUID) ([]model.Tag, error) {
	query := `
		SELECT t.id, t.name, t.color, t.created_at
		FROM tags t
		JOIN memory_tag_rel mtr ON t.id = mtr.tag_id
		WHERE mtr.memory_id = $1
		ORDER BY t.name`

	rows, err := r.pool.Query(ctx, query, memoryID)
	if err != nil {
		return nil, fmt.Errorf("get tags by memory id: %w", err)
	}
	defer rows.Close()

	var tags []model.Tag
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}
