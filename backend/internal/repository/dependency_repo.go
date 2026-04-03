package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

type DependencyRepo struct {
	pool *pgxpool.Pool
}

func NewDependencyRepo(pool *pgxpool.Pool) *DependencyRepo {
	return &DependencyRepo{pool: pool}
}

var depColumns = `id, source_issue_id, target_issue_id, type, severity, created_at`

func scanDep(row pgx.Row) (*model.IssueDependency, error) {
	var d model.IssueDependency
	err := row.Scan(&d.ID, &d.SourceIssueID, &d.TargetIssueID, &d.Type, &d.Severity, &d.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DependencyRepo) Create(ctx context.Context, dep model.IssueDependency) (*model.IssueDependency, error) {
	query := fmt.Sprintf(`
		INSERT INTO issue_dependencies (source_issue_id, target_issue_id, type, severity)
		VALUES ($1, $2, $3, $4)
		RETURNING %s`, depColumns)

	row := r.pool.QueryRow(ctx, query, dep.SourceIssueID, dep.TargetIssueID, dep.Type, dep.Severity)
	result, err := scanDep(row)
	if err != nil {
		return nil, fmt.Errorf("create dependency: %w", err)
	}
	return result, nil
}

func (r *DependencyRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM issue_dependencies WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete dependency: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("dependency not found")
	}
	return nil
}

func (r *DependencyRepo) ListByIssueID(ctx context.Context, issueID uuid.UUID) ([]model.IssueDependency, error) {
	query := fmt.Sprintf(`SELECT %s FROM issue_dependencies WHERE source_issue_id = $1 OR target_issue_id = $1 ORDER BY created_at`, depColumns)
	rows, err := r.pool.Query(ctx, query, issueID)
	if err != nil {
		return nil, fmt.Errorf("list dependencies: %w", err)
	}
	defer rows.Close()

	var deps []model.IssueDependency
	for rows.Next() {
		var d model.IssueDependency
		if err := rows.Scan(&d.ID, &d.SourceIssueID, &d.TargetIssueID, &d.Type, &d.Severity, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}
		deps = append(deps, d)
	}
	return deps, rows.Err()
}

func (r *DependencyRepo) GetDependsOn(ctx context.Context, issueID uuid.UUID) ([]model.IssueDependency, error) {
	query := fmt.Sprintf(`SELECT %s FROM issue_dependencies WHERE source_issue_id = $1 AND type = 'depends_on' ORDER BY created_at`, depColumns)
	return r.queryDeps(ctx, query, issueID)
}

func (r *DependencyRepo) GetBlocks(ctx context.Context, issueID uuid.UUID) ([]model.IssueDependency, error) {
	query := fmt.Sprintf(`SELECT %s FROM issue_dependencies WHERE source_issue_id = $1 AND type = 'blocks' ORDER BY created_at`, depColumns)
	return r.queryDeps(ctx, query, issueID)
}

func (r *DependencyRepo) queryDeps(ctx context.Context, query string, issueID uuid.UUID) ([]model.IssueDependency, error) {
	rows, err := r.pool.Query(ctx, query, issueID)
	if err != nil {
		return nil, fmt.Errorf("query dependencies: %w", err)
	}
	defer rows.Close()

	var deps []model.IssueDependency
	for rows.Next() {
		var d model.IssueDependency
		if err := rows.Scan(&d.ID, &d.SourceIssueID, &d.TargetIssueID, &d.Type, &d.Severity, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}
		deps = append(deps, d)
	}
	return deps, rows.Err()
}

// HasPath uses a recursive CTE to check if there is a directed path from sourceID to targetID
// via depends_on edges (source_issue_id depends on target_issue_id).
// This is used for cycle detection: before creating A depends_on B, check if B already
// has a path to A (which would create a cycle).
func (r *DependencyRepo) HasPath(ctx context.Context, sourceID, targetID uuid.UUID) (bool, error) {
	query := `
		WITH RECURSIVE dep_chain AS (
			SELECT target_issue_id
			FROM issue_dependencies
			WHERE source_issue_id = $1 AND type = 'depends_on'
			UNION
			SELECT d.target_issue_id
			FROM issue_dependencies d
			JOIN dep_chain c ON d.source_issue_id = c.target_issue_id
			WHERE d.type = 'depends_on'
		)
		SELECT EXISTS (SELECT 1 FROM dep_chain WHERE target_issue_id = $2)`

	var exists bool
	err := r.pool.QueryRow(ctx, query, sourceID, targetID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check dependency path: %w", err)
	}
	return exists, nil
}
