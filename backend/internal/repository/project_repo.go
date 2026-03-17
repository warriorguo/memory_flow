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

type ProjectRepo struct {
	pool *pgxpool.Pool
}

func NewProjectRepo(pool *pgxpool.Pool) *ProjectRepo {
	return &ProjectRepo{pool: pool}
}

func (r *ProjectRepo) Create(ctx context.Context, req model.CreateProjectRequest) (*model.Project, error) {
	query := `
		INSERT INTO projects (key, name, summary, description, design_principles, git_url, cicd_url, doc_url, owner_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, key, name, summary, description, design_principles, git_url, cicd_url, doc_url, owner_id, status, next_issue_number, created_at, updated_at`

	row := r.pool.QueryRow(ctx, query,
		req.Key, req.Name, req.Summary, req.Description, req.DesignPrinciples,
		req.GitURL, req.CICDURL, req.DocURL, req.OwnerID,
	)

	var p model.Project
	err := row.Scan(
		&p.ID, &p.Key, &p.Name, &p.Summary, &p.Description, &p.DesignPrinciples,
		&p.GitURL, &p.CICDURL, &p.DocURL, &p.OwnerID, &p.Status, &p.NextIssueNumber,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return &p, nil
}

func (r *ProjectRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	query := `
		SELECT id, key, name, summary, description, design_principles, git_url, cicd_url, doc_url, owner_id, status, next_issue_number, created_at, updated_at
		FROM projects WHERE id = $1`

	row := r.pool.QueryRow(ctx, query, id)

	var p model.Project
	err := row.Scan(
		&p.ID, &p.Key, &p.Name, &p.Summary, &p.Description, &p.DesignPrinciples,
		&p.GitURL, &p.CICDURL, &p.DocURL, &p.OwnerID, &p.Status, &p.NextIssueNumber,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get project by id: %w", err)
	}
	return &p, nil
}

func (r *ProjectRepo) List(ctx context.Context, filter model.ProjectFilter) ([]model.Project, int, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if filter.Name != nil {
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", argIdx))
		args = append(args, "%"+escapeLike(*filter.Name)+"%")
		argIdx++
	}
	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.OwnerID != nil {
		conditions = append(conditions, fmt.Sprintf("owner_id = $%d", argIdx))
		args = append(args, *filter.OwnerID)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	page, pageSize := normalizePagination(filter.Page, filter.PageSize)
	offset := (page - 1) * pageSize

	query := fmt.Sprintf(`
		SELECT id, key, name, summary, description, design_principles, git_url, cicd_url, doc_url, owner_id, status, next_issue_number, created_at, updated_at,
		       COUNT(*) OVER() AS total
		FROM projects %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []model.Project
	var total int
	for rows.Next() {
		var p model.Project
		err := rows.Scan(
			&p.ID, &p.Key, &p.Name, &p.Summary, &p.Description, &p.DesignPrinciples,
			&p.GitURL, &p.CICDURL, &p.DocURL, &p.OwnerID, &p.Status, &p.NextIssueNumber,
			&p.CreatedAt, &p.UpdatedAt, &total,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate projects: %w", err)
	}

	return projects, total, nil
}

func (r *ProjectRepo) Update(ctx context.Context, id uuid.UUID, req model.UpdateProjectRequest) (*model.Project, error) {
	var setClauses []string
	var args []interface{}
	argIdx := 1

	if req.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *req.Name)
		argIdx++
	}
	if req.Summary != nil {
		setClauses = append(setClauses, fmt.Sprintf("summary = $%d", argIdx))
		args = append(args, *req.Summary)
		argIdx++
	}
	if req.Description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *req.Description)
		argIdx++
	}
	if req.DesignPrinciples != nil {
		setClauses = append(setClauses, fmt.Sprintf("design_principles = $%d", argIdx))
		args = append(args, *req.DesignPrinciples)
		argIdx++
	}
	if req.GitURL != nil {
		setClauses = append(setClauses, fmt.Sprintf("git_url = $%d", argIdx))
		args = append(args, *req.GitURL)
		argIdx++
	}
	if req.CICDURL != nil {
		setClauses = append(setClauses, fmt.Sprintf("cicd_url = $%d", argIdx))
		args = append(args, *req.CICDURL)
		argIdx++
	}
	if req.DocURL != nil {
		setClauses = append(setClauses, fmt.Sprintf("doc_url = $%d", argIdx))
		args = append(args, *req.DocURL)
		argIdx++
	}
	if req.OwnerID != nil {
		setClauses = append(setClauses, fmt.Sprintf("owner_id = $%d", argIdx))
		args = append(args, *req.OwnerID)
		argIdx++
	}
	if req.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *req.Status)
		argIdx++
	}

	if len(setClauses) == 0 {
		return r.GetByID(ctx, id)
	}

	setClauses = append(setClauses, "updated_at = now()")
	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE projects SET %s WHERE id = $%d
		RETURNING id, key, name, summary, description, design_principles, git_url, cicd_url, doc_url, owner_id, status, next_issue_number, created_at, updated_at`,
		strings.Join(setClauses, ", "), argIdx)

	row := r.pool.QueryRow(ctx, query, args...)
	var p model.Project
	err := row.Scan(
		&p.ID, &p.Key, &p.Name, &p.Summary, &p.Description, &p.DesignPrinciples,
		&p.GitURL, &p.CICDURL, &p.DocURL, &p.OwnerID, &p.Status, &p.NextIssueNumber,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("update project: %w", err)
	}
	return &p, nil
}

func (r *ProjectRepo) Archive(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE projects SET status = 'archived', updated_at = now() WHERE id = $1`
	ct, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("archive project: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("project not found")
	}
	return nil
}

func (r *ProjectRepo) IncrementIssueNumber(ctx context.Context, tx pgx.Tx, id uuid.UUID) (int, string, error) {
	query := `UPDATE projects SET next_issue_number = next_issue_number + 1, updated_at = now() WHERE id = $1 RETURNING next_issue_number, key`
	row := tx.QueryRow(ctx, query, id)
	var num int
	var key string
	if err := row.Scan(&num, &key); err != nil {
		return 0, "", fmt.Errorf("increment issue number: %w", err)
	}
	return num, key, nil
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

func normalizePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
