package service

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
)

var validProjectStatuses = map[string]bool{"active": true, "paused": true, "archived": true}
var projectKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)

type ProjectService struct {
	repo repository.ProjectRepository
}

func NewProjectService(repo repository.ProjectRepository) *ProjectService {
	return &ProjectService{repo: repo}
}

func (s *ProjectService) Create(ctx context.Context, req model.CreateProjectRequest) (*model.Project, error) {
	if req.Key == "" {
		return nil, fmt.Errorf("project key is required")
	}
	if !projectKeyPattern.MatchString(req.Key) {
		return nil, fmt.Errorf("invalid project key: must be uppercase alphanumeric, 2-10 chars, starting with a letter")
	}
	if req.Name == "" {
		return nil, fmt.Errorf("project name is required")
	}
	return s.repo.Create(ctx, req)
}

func (s *ProjectService) GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ProjectService) List(ctx context.Context, filter model.ProjectFilter) ([]model.Project, int, error) {
	return s.repo.List(ctx, filter)
}

func (s *ProjectService) Update(ctx context.Context, id uuid.UUID, req model.UpdateProjectRequest) (*model.Project, error) {
	if req.Status != nil && !validProjectStatuses[*req.Status] {
		return nil, fmt.Errorf("invalid status: must be 'active', 'paused', or 'archived'")
	}
	return s.repo.Update(ctx, id, req)
}

func (s *ProjectService) Archive(ctx context.Context, id uuid.UUID) error {
	return s.repo.Archive(ctx, id)
}
