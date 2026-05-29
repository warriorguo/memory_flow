package synccore

import (
	"context"

	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

func exportProjects(ctx context.Context, db database.DB) ([]model.Project, error) {
	rows, err := db.Query(ctx, `SELECT id, key, name, summary, description, design_principles, git_url, cicd_url, doc_url, owner_id, status, next_issue_number, created_at, updated_at FROM projects`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Project
	for rows.Next() {
		var p model.Project
		if err := rows.Scan(&p.ID, &p.Key, &p.Name, &p.Summary, &p.Description, &p.DesignPrinciples,
			&p.GitURL, &p.CICDURL, &p.DocURL, &p.OwnerID, &p.Status, &p.NextIssueNumber,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func exportIssues(ctx context.Context, db database.DB) ([]model.Issue, error) {
	rows, err := db.Query(ctx, `SELECT id, issue_key, project_id, type, title, description, priority, status, assignee_id, creator_id, source, version, git_url, pr_url, doc_url, created_at, updated_at FROM issues`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Issue
	for rows.Next() {
		var i model.Issue
		if err := rows.Scan(&i.ID, &i.IssueKey, &i.ProjectID, &i.Type, &i.Title, &i.Description,
			&i.Priority, &i.Status, &i.AssigneeID, &i.CreatorID, &i.Source, &i.Version,
			&i.GitURL, &i.PRURL, &i.DocURL, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func exportIssueHistory(ctx context.Context, db database.DB) ([]model.IssueHistory, error) {
	rows, err := db.Query(ctx, `SELECT id, issue_id, field_name, old_value, new_value, operator_id, created_at FROM issue_history`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.IssueHistory
	for rows.Next() {
		var h model.IssueHistory
		if err := rows.Scan(&h.ID, &h.IssueID, &h.FieldName, &h.OldValue, &h.NewValue, &h.OperatorID, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func exportTags(ctx context.Context, db database.DB) ([]model.Tag, error) {
	rows, err := db.Query(ctx, `SELECT id, name, color, created_at FROM tags`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Tag
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func exportMemories(ctx context.Context, db database.DB) ([]model.Memory, error) {
	rows, err := db.Query(ctx, `SELECT id, project_id, type, title, content, source_object_type, source_object_id, creator_id, created_at, updated_at FROM memories`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Memory
	for rows.Next() {
		var m model.Memory
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Type, &m.Title, &m.Content,
			&m.SourceObjectType, &m.SourceObjectID, &m.CreatorID, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func exportTagRel(ctx context.Context, db database.DB, table, leftCol string) ([]TagRel, error) {
	rows, err := db.Query(ctx, `SELECT `+leftCol+`, tag_id FROM `+table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TagRel
	for rows.Next() {
		var r TagRel
		if err := rows.Scan(&r.LeftID, &r.TagID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func exportDependencies(ctx context.Context, db database.DB) ([]model.IssueDependency, error) {
	rows, err := db.Query(ctx, `SELECT id, source_issue_id, target_issue_id, type, severity, created_at FROM issue_dependencies`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.IssueDependency
	for rows.Next() {
		var d model.IssueDependency
		if err := rows.Scan(&d.ID, &d.SourceIssueID, &d.TargetIssueID, &d.Type, &d.Severity, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func exportUsers(ctx context.Context, db database.DB) ([]SyncUser, error) {
	rows, err := db.Query(ctx, `SELECT id, username, password_hash, display_name, role, created_at FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SyncUser
	for rows.Next() {
		var u SyncUser
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
