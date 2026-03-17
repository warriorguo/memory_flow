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

type IssueRepo struct {
	pool *pgxpool.Pool
}

func NewIssueRepo(pool *pgxpool.Pool) *IssueRepo {
	return &IssueRepo{pool: pool}
}

var issueColumns = `id, issue_key, project_id, type, title, description, priority, status, assignee_id, creator_id, source, version, git_url, pr_url, doc_url, created_at, updated_at`

func scanIssue(row pgx.Row) (*model.Issue, error) {
	var i model.Issue
	err := row.Scan(
		&i.ID, &i.IssueKey, &i.ProjectID, &i.Type, &i.Title, &i.Description,
		&i.Priority, &i.Status, &i.AssigneeID, &i.CreatorID, &i.Source,
		&i.Version, &i.GitURL, &i.PRURL, &i.DocURL, &i.CreatedAt, &i.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (r *IssueRepo) Create(ctx context.Context, tx pgx.Tx, issueKey string, projectID uuid.UUID, req model.CreateIssueRequest) (*model.Issue, error) {
	priority := "P2"
	if req.Priority != nil {
		priority = *req.Priority
	}

	query := fmt.Sprintf(`
		INSERT INTO issues (issue_key, project_id, type, title, description, priority, assignee_id, creator_id, source, version, git_url, pr_url, doc_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING %s`, issueColumns)

	row := tx.QueryRow(ctx, query,
		issueKey, projectID, req.Type, req.Title, req.Description, priority,
		req.AssigneeID, req.CreatorID, req.Source, req.Version,
		req.GitURL, req.PRURL, req.DocURL,
	)

	issue, err := scanIssue(row)
	if err != nil {
		return nil, fmt.Errorf("create issue: %w", err)
	}
	return issue, nil
}

func (r *IssueRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
	query := fmt.Sprintf(`SELECT %s FROM issues WHERE id = $1`, issueColumns)
	row := r.pool.QueryRow(ctx, query, id)
	issue, err := scanIssue(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get issue by id: %w", err)
	}
	return issue, nil
}

func (r *IssueRepo) List(ctx context.Context, filter model.IssueFilter) ([]model.Issue, int, error) {
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
	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.Priority != nil {
		conditions = append(conditions, fmt.Sprintf("priority = $%d", argIdx))
		args = append(args, *filter.Priority)
		argIdx++
	}
	if filter.AssigneeID != nil {
		conditions = append(conditions, fmt.Sprintf("assignee_id = $%d", argIdx))
		args = append(args, *filter.AssigneeID)
		argIdx++
	}
	if filter.CreatorID != nil {
		conditions = append(conditions, fmt.Sprintf("creator_id = $%d", argIdx))
		args = append(args, *filter.CreatorID)
		argIdx++
	}
	if filter.Keyword != nil {
		conditions = append(conditions, fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+escapeLike(*filter.Keyword)+"%")
		argIdx++
	}
	if filter.TagID != nil {
		conditions = append(conditions, fmt.Sprintf("id IN (SELECT issue_id FROM issue_tag_rel WHERE tag_id = $%d)", argIdx))
		args = append(args, *filter.TagID)
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
		FROM issues %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, issueColumns, where, argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()

	var issues []model.Issue
	var total int
	for rows.Next() {
		var i model.Issue
		err := rows.Scan(
			&i.ID, &i.IssueKey, &i.ProjectID, &i.Type, &i.Title, &i.Description,
			&i.Priority, &i.Status, &i.AssigneeID, &i.CreatorID, &i.Source,
			&i.Version, &i.GitURL, &i.PRURL, &i.DocURL, &i.CreatedAt, &i.UpdatedAt,
			&total,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan issue: %w", err)
		}
		issues = append(issues, i)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate issues: %w", err)
	}

	return issues, total, nil
}

func (r *IssueRepo) Update(ctx context.Context, tx pgx.Tx, id uuid.UUID, setClauses []string, args []interface{}) (*model.Issue, error) {
	setClauses = append(setClauses, "updated_at = now()")
	argIdx := len(args) + 1
	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE issues SET %s WHERE id = $%d
		RETURNING %s`,
		strings.Join(setClauses, ", "), argIdx, issueColumns)

	row := tx.QueryRow(ctx, query, args...)
	issue, err := scanIssue(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("update issue: %w", err)
	}
	return issue, nil
}

func (r *IssueRepo) CountByStatus(ctx context.Context, projectID uuid.UUID) (map[string]int, error) {
	query := `SELECT status, COUNT(*) as count FROM issues WHERE project_id = $1 GROUP BY status ORDER BY status`
	rows, err := r.pool.Query(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("count by status: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan status count: %w", err)
		}
		counts[status] = count
	}
	return counts, rows.Err()
}

func (r *IssueRepo) CountByPriority(ctx context.Context, projectID uuid.UUID) (map[string]int, error) {
	query := `SELECT priority, COUNT(*) as count FROM issues WHERE project_id = $1 GROUP BY priority ORDER BY priority`
	rows, err := r.pool.Query(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("count by priority: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var priority string
		var count int
		if err := rows.Scan(&priority, &count); err != nil {
			return nil, fmt.Errorf("scan priority count: %w", err)
		}
		counts[priority] = count
	}
	return counts, rows.Err()
}

func (r *IssueRepo) CountByType(ctx context.Context, projectID uuid.UUID) (map[string]int, error) {
	query := `SELECT type, COUNT(*) as count FROM issues WHERE project_id = $1 GROUP BY type ORDER BY type`
	rows, err := r.pool.Query(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("count by type: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var typ string
		var count int
		if err := rows.Scan(&typ, &count); err != nil {
			return nil, fmt.Errorf("scan type count: %w", err)
		}
		counts[typ] = count
	}
	return counts, rows.Err()
}

func (r *IssueRepo) GetTrend(ctx context.Context, projectID uuid.UUID, days int) ([]model.TrendPoint, error) {
	// Query created issues per day
	createdQuery := `
		SELECT DATE(created_at)::text AS date, COUNT(*) AS count
		FROM issues
		WHERE project_id = $1 AND created_at >= now() - interval '1 day' * $2
		GROUP BY DATE(created_at)`

	createdRows, err := r.pool.Query(ctx, createdQuery, projectID, days)
	if err != nil {
		return nil, fmt.Errorf("get trend created: %w", err)
	}
	defer createdRows.Close()

	createdMap := make(map[string]int)
	for createdRows.Next() {
		var date string
		var count int
		if err := createdRows.Scan(&date, &count); err != nil {
			return nil, fmt.Errorf("scan created trend: %w", err)
		}
		createdMap[date] = count
	}
	if err := createdRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate created trend: %w", err)
	}

	// Query done issues per day (from issue_history)
	doneQuery := `
		SELECT DATE(ih.created_at)::text AS date, COUNT(*) AS count
		FROM issue_history ih
		JOIN issues i ON i.id = ih.issue_id
		WHERE i.project_id = $1
		  AND ih.field_name = 'status'
		  AND ih.new_value = 'done'
		  AND ih.created_at >= now() - interval '1 day' * $2
		GROUP BY DATE(ih.created_at)`

	doneRows, err := r.pool.Query(ctx, doneQuery, projectID, days)
	if err != nil {
		return nil, fmt.Errorf("get trend done: %w", err)
	}
	defer doneRows.Close()

	doneMap := make(map[string]int)
	for doneRows.Next() {
		var date string
		var count int
		if err := doneRows.Scan(&date, &count); err != nil {
			return nil, fmt.Errorf("scan done trend: %w", err)
		}
		doneMap[date] = count
	}
	if err := doneRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate done trend: %w", err)
	}

	// Merge into TrendPoint structs using generate_series for all days
	seriesQuery := `
		SELECT d::date::text AS date
		FROM generate_series(now() - ($1 || ' days')::interval, now(), '1 day') AS d
		ORDER BY d::date`

	seriesRows, err := r.pool.Query(ctx, seriesQuery, fmt.Sprintf("%d", days))
	if err != nil {
		return nil, fmt.Errorf("get trend series: %w", err)
	}
	defer seriesRows.Close()

	var points []model.TrendPoint
	for seriesRows.Next() {
		var date string
		if err := seriesRows.Scan(&date); err != nil {
			return nil, fmt.Errorf("scan trend date: %w", err)
		}
		points = append(points, model.TrendPoint{
			Date:    date,
			Created: createdMap[date],
			Done:    doneMap[date],
		})
	}
	return points, seriesRows.Err()
}

func (r *IssueRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

// Pool returns the underlying connection pool for use in transactions.
func (r *IssueRepo) Pool() *pgxpool.Pool {
	return r.pool
}
