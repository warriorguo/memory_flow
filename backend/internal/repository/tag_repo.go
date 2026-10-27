package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

// ErrTagNameTaken is returned when renaming a tag onto another tag's name.
var ErrTagNameTaken = errors.New("a tag with that name already exists")

type TagRepo struct {
	db database.DB
}

func NewTagRepo(db database.DB) *TagRepo {
	return &TagRepo{db: db}
}

// Create inserts a tag. A tag with no color gets one from [model.TagPalette]
// rather than staying blank, so every label in a list has something to show.
func (r *TagRepo) Create(ctx context.Context, req model.CreateTagRequest) (*model.Tag, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("tag name is required")
	}
	var color string
	if req.Color != nil && strings.TrimSpace(*req.Color) != "" {
		c, err := model.NormalizeTagColor(*req.Color)
		if err != nil {
			return nil, err
		}
		color = c
	} else {
		existing, err := r.List(ctx)
		if err != nil {
			return nil, err
		}
		color = model.PickTagColor(name, existing)
	}

	query := `
		INSERT INTO tags (id, name, color)
		VALUES ($1, $2, $3)
		RETURNING id, name, color, created_at`

	row := r.db.QueryRow(ctx, query, uuid.New(), name, color)

	var t model.Tag
	err := row.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create tag: %w", err)
	}
	return &t, nil
}

// GetByName finds a tag by name, ignoring case — the CLI has always matched
// names that way, and two tags differing only in case would be a trap.
func (r *TagRepo) GetByName(ctx context.Context, name string) (*model.Tag, error) {
	query := `SELECT id, name, color, created_at FROM tags WHERE LOWER(name) = LOWER($1) ORDER BY name LIMIT 1`
	row := r.db.QueryRow(ctx, query, strings.TrimSpace(name))

	var t model.Tag
	if err := row.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedAt); err != nil {
		if err == database.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get tag by name: %w", err)
	}
	return &t, nil
}

// Update renames and/or recolors a tag. It returns ErrTagNameTaken when the
// new name belongs to another tag, and (nil, nil) when the tag does not exist.
func (r *TagRepo) Update(ctx context.Context, id uuid.UUID, req model.UpdateTagRequest) (*model.Tag, error) {
	current, err := r.GetByID(ctx, id)
	if err != nil || current == nil {
		return nil, err
	}
	name, color := current.Name, deref(current.Color)
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("tag name cannot be empty")
		}
		other, err := r.GetByName(ctx, name)
		if err != nil {
			return nil, err
		}
		if other != nil && other.ID != id {
			return nil, ErrTagNameTaken
		}
	}
	if req.Color != nil {
		c, err := model.NormalizeTagColor(*req.Color)
		if err != nil {
			return nil, err
		}
		color = c
	}

	row := r.db.QueryRow(ctx, `
		UPDATE tags SET name = $1, color = $2 WHERE id = $3
		RETURNING id, name, color, created_at`, name, color, id)
	var t model.Tag
	if err := row.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedAt); err != nil {
		return nil, fmt.Errorf("update tag: %w", err)
	}
	return &t, nil
}

// GetByIssueIDs loads the tags of many issues in one query, so a list page
// costs one round trip for its tags instead of one per row.
func (r *TagRepo) GetByIssueIDs(ctx context.Context, issueIDs []uuid.UUID) (map[uuid.UUID][]model.Tag, error) {
	out := make(map[uuid.UUID][]model.Tag, len(issueIDs))
	if len(issueIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(issueIDs))
	args := make([]interface{}, len(issueIDs))
	for i, id := range issueIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	query := `
		SELECT itr.issue_id, t.id, t.name, t.color, t.created_at
		FROM tags t
		JOIN issue_tag_rel itr ON t.id = itr.tag_id
		WHERE itr.issue_id IN (` + strings.Join(placeholders, ", ") + `)
		ORDER BY t.name`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get tags by issue ids: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var issueID uuid.UUID
		var t model.Tag
		if err := rows.Scan(&issueID, &t.ID, &t.Name, &t.Color, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		out[issueID] = append(out[issueID], t)
	}
	return out, rows.Err()
}

func (r *TagRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Tag, error) {
	query := `SELECT id, name, color, created_at FROM tags WHERE id = $1`
	row := r.db.QueryRow(ctx, query, id)

	var t model.Tag
	err := row.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedAt)
	if err != nil {
		if err == database.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get tag by id: %w", err)
	}
	return &t, nil
}

func (r *TagRepo) List(ctx context.Context) ([]model.Tag, error) {
	query := `SELECT id, name, color, created_at FROM tags ORDER BY name`
	rows, err := r.db.Query(ctx, query)
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
	_, err := r.db.Exec(ctx, query, issueID, tagID)
	if err != nil {
		return fmt.Errorf("add tag to issue: %w", err)
	}
	return nil
}

func (r *TagRepo) RemoveFromIssue(ctx context.Context, issueID, tagID uuid.UUID) error {
	query := `DELETE FROM issue_tag_rel WHERE issue_id = $1 AND tag_id = $2`
	ct, err := r.db.Exec(ctx, query, issueID, tagID)
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

	rows, err := r.db.Query(ctx, query, issueID)
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
	_, err := r.db.Exec(ctx, query, memoryID, tagID)
	if err != nil {
		return fmt.Errorf("add tag to memory: %w", err)
	}
	return nil
}

func (r *TagRepo) RemoveFromMemory(ctx context.Context, memoryID, tagID uuid.UUID) error {
	query := `DELETE FROM memory_tag_rel WHERE memory_id = $1 AND tag_id = $2`
	ct, err := r.db.Exec(ctx, query, memoryID, tagID)
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

	rows, err := r.db.Query(ctx, query, memoryID)
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

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
