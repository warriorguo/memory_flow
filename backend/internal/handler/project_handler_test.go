package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository/mocks"
	"github.com/warriorguo/memory_flow/backend/internal/service"
)

func newTestProjectHandler() (*ProjectHandler, *mocks.MockProjectRepo) {
	repo := &mocks.MockProjectRepo{}
	svc := service.NewProjectService(repo)
	issueRepo := &mocks.MockIssueRepo{}
	historyRepo := &mocks.MockIssueHistoryRepo{}
	issueSvc := service.NewIssueService(issueRepo, repo, historyRepo)
	resolver := NewIDResolver(svc, issueSvc)
	h := NewProjectHandler(svc, resolver)
	return h, repo
}

func TestCreateProject_Handler_Success(t *testing.T) {
	h, repo := newTestProjectHandler()

	repo.CreateFn = func(ctx context.Context, req model.CreateProjectRequest) (*model.Project, error) {
		return &model.Project{
			ID:        uuid.New(),
			Key:       req.Key,
			Name:      req.Name,
			Status:    "active",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil
	}

	body, _ := json.Marshal(model.CreateProjectRequest{
		Key:  "PROJ",
		Name: "Test Project",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var resp dataResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data == nil {
		t.Error("expected non-nil data")
	}
}

func TestCreateProject_Handler_BadRequest(t *testing.T) {
	h, _ := newTestProjectHandler()

	// Send invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestGetProject_Handler_Success(t *testing.T) {
	h, repo := newTestProjectHandler()

	projectID := uuid.New()
	repo.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{
			ID:        projectID,
			Key:       "PROJ",
			Name:      "Test Project",
			Status:    "active",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String(), nil)
	// Set chi URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", projectID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestGetProject_Handler_NotFound(t *testing.T) {
	h, repo := newTestProjectHandler()

	repo.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return nil, nil
	}

	projectID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", projectID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestListProjects_Handler_Success(t *testing.T) {
	h, repo := newTestProjectHandler()

	repo.ListFn = func(ctx context.Context, filter model.ProjectFilter) ([]model.Project, int, error) {
		return []model.Project{
			{ID: uuid.New(), Key: "P1", Name: "Project 1", Status: "active"},
		}, 1, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects?page=1&page_size=20", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp listResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("expected total 1, got %d", resp.Total)
	}
}

func TestListProjects_Handler_NameFilter(t *testing.T) {
	h, repo := newTestProjectHandler()

	var capturedFilter model.ProjectFilter
	repo.ListFn = func(ctx context.Context, filter model.ProjectFilter) ([]model.Project, int, error) {
		capturedFilter = filter
		return []model.Project{
			{ID: uuid.New(), Key: "ORT", Name: "OZX Room Template", Status: "active"},
		}, 1, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects?name=ORT", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if capturedFilter.Name == nil {
		t.Fatal("expected name filter to be set, got nil")
	}
	if *capturedFilter.Name != "ORT" {
		t.Errorf("expected name filter 'ORT', got %q", *capturedFilter.Name)
	}

	var resp listResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("expected total 1, got %d", resp.Total)
	}
}
