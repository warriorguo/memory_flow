package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository/mocks"
)

func newTestProjectService(repo *mocks.MockProjectRepo) *ProjectService {
	return NewProjectService(repo)
}

func TestCreateProject_Success(t *testing.T) {
	repo := &mocks.MockProjectRepo{
		CreateFn: func(ctx context.Context, req model.CreateProjectRequest) (*model.Project, error) {
			return &model.Project{
				ID:        uuid.New(),
				Key:       req.Key,
				Name:      req.Name,
				Status:    "active",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestProjectService(repo)
	project, err := svc.Create(context.Background(), model.CreateProjectRequest{
		Key:  "PROJ",
		Name: "Test Project",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if project.Key != "PROJ" {
		t.Errorf("expected key PROJ, got %s", project.Key)
	}
	if project.Name != "Test Project" {
		t.Errorf("expected name Test Project, got %s", project.Name)
	}
}

func TestCreateProject_MissingKey(t *testing.T) {
	repo := &mocks.MockProjectRepo{}
	svc := newTestProjectService(repo)

	_, err := svc.Create(context.Background(), model.CreateProjectRequest{
		Name: "Test Project",
	})
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	if err.Error() != "project key is required" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateProject_MissingName(t *testing.T) {
	repo := &mocks.MockProjectRepo{}
	svc := newTestProjectService(repo)

	_, err := svc.Create(context.Background(), model.CreateProjectRequest{
		Key: "PROJ",
	})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if err.Error() != "project name is required" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateProject_InvalidKeyFormat(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"lowercase", "proj"},
		{"starts with number", "1PROJ"},
		{"too short", "P"},
		{"contains special chars", "PR-OJ"},
		{"too long", "ABCDEFGHIJK"},
	}

	repo := &mocks.MockProjectRepo{}
	svc := newTestProjectService(repo)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), model.CreateProjectRequest{
				Key:  tc.key,
				Name: "Test",
			})
			if err == nil {
				t.Fatalf("expected error for key %q", tc.key)
			}
		})
	}
}

func TestUpdateProject_Success(t *testing.T) {
	id := uuid.New()
	newName := "Updated Name"
	repo := &mocks.MockProjectRepo{
		UpdateFn: func(ctx context.Context, uid uuid.UUID, req model.UpdateProjectRequest) (*model.Project, error) {
			return &model.Project{
				ID:        uid,
				Key:       "PROJ",
				Name:      *req.Name,
				Status:    "active",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestProjectService(repo)
	project, err := svc.Update(context.Background(), id, model.UpdateProjectRequest{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if project.Name != "Updated Name" {
		t.Errorf("expected name Updated Name, got %s", project.Name)
	}
}

func TestUpdateProject_InvalidStatus(t *testing.T) {
	repo := &mocks.MockProjectRepo{}
	svc := newTestProjectService(repo)

	badStatus := "invalid_status"
	_, err := svc.Update(context.Background(), uuid.New(), model.UpdateProjectRequest{
		Status: &badStatus,
	})
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestArchiveProject_Success(t *testing.T) {
	id := uuid.New()
	repo := &mocks.MockProjectRepo{
		ArchiveFn: func(ctx context.Context, uid uuid.UUID) error {
			if uid != id {
				t.Errorf("expected id %v, got %v", id, uid)
			}
			return nil
		},
	}

	svc := newTestProjectService(repo)
	err := svc.Archive(context.Background(), id)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestListProjects_Success(t *testing.T) {
	repo := &mocks.MockProjectRepo{
		ListFn: func(ctx context.Context, filter model.ProjectFilter) ([]model.Project, int, error) {
			return []model.Project{
				{ID: uuid.New(), Key: "P1", Name: "Project 1", Status: "active"},
				{ID: uuid.New(), Key: "P2", Name: "Project 2", Status: "active"},
			}, 2, nil
		},
	}

	svc := newTestProjectService(repo)
	projects, total, err := svc.List(context.Background(), model.ProjectFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(projects))
	}
}
