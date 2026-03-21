package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

type MemoryRepo struct {
	pool *pgxpool.Pool
}

func NewMemoryRepo(pool *pgxpool.Pool) *MemoryRepo {
	return &MemoryRepo{pool: pool}
}

// memorySelectCols selects memory columns plus project id/key/name via LEFT JOIN.
const memorySelectCols = `
	m.id, m.project_id, m.type, m.title, m.content,
	m.source_object_type, m.source_object_id, m.creator_id,
	m.created_at, m.updated_at,
	p.id, p.key, p.name`

// scanMemoryResponse scans a row that contains memory columns followed by optional project columns.
func scanMemoryResponse(row pgx.Row) (*model.MemoryResponse, error) {
	var mr model.MemoryResponse
	var projID *uuid.UUID
	var projKey *string
	var projName *string

	err := row.Scan(
		&mr.ID, &mr.ProjectID, &mr.Type, &mr.Title, &mr.Content,
		&mr.SourceObjectType, &mr.SourceObjectID, &mr.CreatorID,
		&mr.CreatedAt, &mr.UpdatedAt,
		&projID, &projKey, &projName,
	)
	if err != nil {
		return nil, err
	}
	if projID != nil && projKey != nil && projName != nil {
		mr.Project = &model.MemoryProjectRef{
			ID:   *projID,
			Key:  *projKey,
			Name: *projName,
		}
	}
	return &mr, nil
}

func (r *MemoryRepo) Create(ctx context.Context, req model.CreateMemoryRequest) (*model.MemoryResponse, error) {
	// Insert first, then fetch with JOIN so project info is included.
	insertQuery := `
		INSERT INTO memories (project_id, type, title, content, source_object_type, source_object_id, creator_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	var newID uuid.UUID
	err := r.pool.QueryRow(ctx, insertQuery,
		req.ProjectID, req.Type, req.Title, req.Content,
		req.SourceObjectType, req.SourceObjectID, req.CreatorID,
	).Scan(&newID)
	if err != nil {
		return nil, fmt.Errorf("create memory: %w", err)
	}
	return r.GetByID(ctx, newID)
}

func (r *MemoryRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.MemoryResponse, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM memories m
		LEFT JOIN projects p ON p.id = m.project_id
		WHERE m.id = $1`, memorySelectCols)

	row := r.pool.QueryRow(ctx, query, id)
	mr, err := scanMemoryResponse(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get memory by id: %w", err)
	}
	return mr, nil
}

func (r *MemoryRepo) List(ctx context.Context, filter model.MemoryFilter) ([]model.MemoryResponse, int, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if filter.ProjectID != nil {
		conditions = append(conditions, fmt.Sprintf("m.project_id = $%d", argIdx))
		args = append(args, *filter.ProjectID)
		argIdx++
	}
	if filter.Type != nil {
		conditions = append(conditions, fmt.Sprintf("m.type = $%d", argIdx))
		args = append(args, *filter.Type)
		argIdx++
	}
	if filter.Keyword != nil {
		conditions = append(conditions, fmt.Sprintf("(m.title ILIKE $%d OR m.content ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+escapeLike(*filter.Keyword)+"%")
		argIdx++
	}
	if filter.SourceObjectType != nil {
		conditions = append(conditions, fmt.Sprintf("m.source_object_type = $%d", argIdx))
		args = append(args, *filter.SourceObjectType)
		argIdx++
	}
	if filter.SourceObjectID != nil {
		conditions = append(conditions, fmt.Sprintf("m.source_object_id = $%d", argIdx))
		args = append(args, *filter.SourceObjectID)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	page, pageSize := normalizePagination(filter.Page, filter.PageSize)
	offset := (page - 1) * pageSize

	query := fmt.Sprintf(`
		SELECT %s, COUNT(*) OVER() AS total
		FROM memories m
		LEFT JOIN projects p ON p.id = m.project_id
		%s
		ORDER BY m.created_at DESC
		LIMIT $%d OFFSET $%d`, memorySelectCols, where, argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()

	var memories []model.MemoryResponse
	var total int
	for rows.Next() {
		var mr model.MemoryResponse
		var projID *uuid.UUID
		var projKey *string
		var projName *string

		err := rows.Scan(
			&mr.ID, &mr.ProjectID, &mr.Type, &mr.Title, &mr.Content,
			&mr.SourceObjectType, &mr.SourceObjectID, &mr.CreatorID,
			&mr.CreatedAt, &mr.UpdatedAt,
			&projID, &projKey, &projName,
			&total,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan memory: %w", err)
		}
		if projID != nil && projKey != nil && projName != nil {
			mr.Project = &model.MemoryProjectRef{
				ID:   *projID,
				Key:  *projKey,
				Name: *projName,
			}
		}
		memories = append(memories, mr)
	}
	return memories, total, rows.Err()
}

func (r *MemoryRepo) Update(ctx context.Context, id uuid.UUID, req model.UpdateMemoryRequest) (*model.MemoryResponse, error) {
	var setClauses []string
	var args []interface{}
	argIdx := 1

	if req.Type != nil {
		setClauses = append(setClauses, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, *req.Type)
		argIdx++
	}
	if req.Title != nil {
		setClauses = append(setClauses, fmt.Sprintf("title = $%d", argIdx))
		args = append(args, *req.Title)
		argIdx++
	}
	if req.Content != nil {
		setClauses = append(setClauses, fmt.Sprintf("content = $%d", argIdx))
		args = append(args, *req.Content)
		argIdx++
	}

	if len(setClauses) == 0 {
		return r.GetByID(ctx, id)
	}

	setClauses = append(setClauses, "updated_at = now()")
	args = append(args, id)

	updateQuery := fmt.Sprintf(`
		UPDATE memories SET %s WHERE id = $%d`,
		strings.Join(setClauses, ", "), argIdx)

	ct, err := r.pool.Exec(ctx, updateQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("update memory: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return nil, nil
	}
	return r.GetByID(ctx, id)
}

func (r *MemoryRepo) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM memories WHERE id = $1`
	ct, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("memory not found")
	}
	return nil
}
