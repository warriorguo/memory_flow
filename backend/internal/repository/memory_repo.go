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

var memoryColumns = `id, project_id, type, title, content, source_object_type, source_object_id, creator_id, created_at, updated_at`

func scanMemory(row pgx.Row) (*model.Memory, error) {
	var m model.Memory
	err := row.Scan(
		&m.ID, &m.ProjectID, &m.Type, &m.Title, &m.Content,
		&m.SourceObjectType, &m.SourceObjectID, &m.CreatorID,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *MemoryRepo) Create(ctx context.Context, req model.CreateMemoryRequest) (*model.Memory, error) {
	query := fmt.Sprintf(`
		INSERT INTO memories (project_id, type, title, content, source_object_type, source_object_id, creator_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING %s`, memoryColumns)

	row := r.pool.QueryRow(ctx, query,
		req.ProjectID, req.Type, req.Title, req.Content,
		req.SourceObjectType, req.SourceObjectID, req.CreatorID,
	)

	memory, err := scanMemory(row)
	if err != nil {
		return nil, fmt.Errorf("create memory: %w", err)
	}
	return memory, nil
}

func (r *MemoryRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Memory, error) {
	query := fmt.Sprintf(`SELECT %s FROM memories WHERE id = $1`, memoryColumns)
	row := r.pool.QueryRow(ctx, query, id)
	memory, err := scanMemory(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get memory by id: %w", err)
	}
	return memory, nil
}

func (r *MemoryRepo) List(ctx context.Context, filter model.MemoryFilter) ([]model.Memory, int, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if filter.ProjectID != nil {
		conditions = append(conditions, fmt.Sprintf("project_id = $%d", argIdx))
		args = append(args, *filter.ProjectID)
		argIdx++
	}
	if filter.Type != nil {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, *filter.Type)
		argIdx++
	}
	if filter.Keyword != nil {
		conditions = append(conditions, fmt.Sprintf("(title ILIKE $%d OR content ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+escapeLike(*filter.Keyword)+"%")
		argIdx++
	}
	if filter.SourceObjectType != nil {
		conditions = append(conditions, fmt.Sprintf("source_object_type = $%d", argIdx))
		args = append(args, *filter.SourceObjectType)
		argIdx++
	}
	if filter.SourceObjectID != nil {
		conditions = append(conditions, fmt.Sprintf("source_object_id = $%d", argIdx))
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
		FROM memories %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, memoryColumns, where, argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()

	var memories []model.Memory
	var total int
	for rows.Next() {
		var m model.Memory
		err := rows.Scan(
			&m.ID, &m.ProjectID, &m.Type, &m.Title, &m.Content,
			&m.SourceObjectType, &m.SourceObjectID, &m.CreatorID,
			&m.CreatedAt, &m.UpdatedAt, &total,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan memory: %w", err)
		}
		memories = append(memories, m)
	}
	return memories, total, rows.Err()
}

func (r *MemoryRepo) Update(ctx context.Context, id uuid.UUID, req model.UpdateMemoryRequest) (*model.Memory, error) {
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

	query := fmt.Sprintf(`
		UPDATE memories SET %s WHERE id = $%d
		RETURNING %s`,
		strings.Join(setClauses, ", "), argIdx, memoryColumns)

	row := r.pool.QueryRow(ctx, query, args...)
	memory, err := scanMemory(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("update memory: %w", err)
	}
	return memory, nil
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
