package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/service"
)

// IDResolver resolves URL path parameters that can be either a UUID or a
// human-readable key (project key like "OIA", issue key like "OIA-1") into
// the canonical UUID. Existing UUID-based URLs continue to work.
type IDResolver struct {
	projectSvc *service.ProjectService
	issueSvc   *service.IssueService
}

func NewIDResolver(projectSvc *service.ProjectService, issueSvc *service.IssueService) *IDResolver {
	return &IDResolver{projectSvc: projectSvc, issueSvc: issueSvc}
}

// ResolveProjectID accepts a UUID string or a project key and returns the project UUID.
func (r *IDResolver) ResolveProjectID(ctx context.Context, param string) (uuid.UUID, error) {
	// Try UUID first
	id, err := uuid.Parse(param)
	if err == nil {
		return id, nil
	}

	// Treat as project key (uppercase alphanumeric)
	project, err := r.projectSvc.GetByKey(ctx, strings.ToUpper(param))
	if err != nil {
		return uuid.Nil, fmt.Errorf("resolve project: %w", err)
	}
	if project == nil {
		return uuid.Nil, fmt.Errorf("project not found: %s", param)
	}
	return project.ID, nil
}

// ResolveIssueID accepts a UUID string or an issue key (e.g. "MF-1") and returns the issue UUID.
func (r *IDResolver) ResolveIssueID(ctx context.Context, param string) (uuid.UUID, error) {
	// Try UUID first
	id, err := uuid.Parse(param)
	if err == nil {
		return id, nil
	}

	// Treat as issue key (e.g. "MF-1")
	issue, err := r.issueSvc.GetByKey(ctx, strings.ToUpper(param))
	if err != nil {
		return uuid.Nil, fmt.Errorf("resolve issue: %w", err)
	}
	if issue == nil {
		return uuid.Nil, fmt.Errorf("issue not found: %s", param)
	}
	return issue.ID, nil
}
