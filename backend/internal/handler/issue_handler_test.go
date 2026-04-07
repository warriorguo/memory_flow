package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository/mocks"
	"github.com/warriorguo/memory_flow/backend/internal/service"
)

func newMockTxForHandler() pgx.Tx {
	return &mocks.MockTx{
		CommitFn:   func(ctx context.Context) error { return nil },
		RollbackFn: func(ctx context.Context) error { return nil },
	}
}

func newTestIssueHandler() (*IssueHandler, *mocks.MockIssueRepo, *mocks.MockProjectRepo, *mocks.MockIssueHistoryRepo, *mocks.MockTagRepo) {
	issueRepo := &mocks.MockIssueRepo{}
	projectRepo := &mocks.MockProjectRepo{}
	historyRepo := &mocks.MockIssueHistoryRepo{}
	tagRepo := &mocks.MockTagRepo{}

	issueSvc := service.NewIssueService(issueRepo, projectRepo, historyRepo)
	projectSvc := service.NewProjectService(projectRepo)
	resolver := NewIDResolver(projectSvc, issueSvc)
	h := NewIssueHandler(issueSvc, tagRepo, resolver)
	return h, issueRepo, projectRepo, historyRepo, tagRepo
}

func TestCreateIssue_Handler_Success(t *testing.T) {
	h, issueRepo, projectRepo, _, _ := newTestIssueHandler()
	projectID := uuid.New()
	mockTx := newMockTxForHandler()

	projectRepo.IncrementIssueNumberFn = func(ctx context.Context, tx pgx.Tx, id uuid.UUID) (int, string, error) {
		return 1, "PROJ", nil
	}
	issueRepo.BeginTxFn = func(ctx context.Context) (pgx.Tx, error) {
		return mockTx, nil
	}
	issueRepo.CreateFn = func(ctx context.Context, tx pgx.Tx, issueKey string, pid uuid.UUID, req model.CreateIssueRequest) (*model.Issue, error) {
		return &model.Issue{
			ID:        uuid.New(),
			IssueKey:  issueKey,
			ProjectID: pid,
			Type:      req.Type,
			Title:     req.Title,
			Priority:  "P2",
			Status:    "todo",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil
	}

	body, _ := json.Marshal(model.CreateIssueRequest{
		Type:  "bug",
		Title: "Test Bug",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/issues", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("projectId", projectID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestGetIssue_Handler_Success(t *testing.T) {
	h, issueRepo, _, _, tagRepo := newTestIssueHandler()
	issueID := uuid.New()

	issueRepo.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
		return &model.Issue{
			ID:        issueID,
			IssueKey:  "PROJ-1",
			ProjectID: uuid.New(),
			Type:      "bug",
			Title:     "Test Bug",
			Priority:  "P2",
			Status:    "todo",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil
	}

	tagRepo.GetByIssueIDFn = func(ctx context.Context, iid uuid.UUID) ([]model.Tag, error) {
		return []model.Tag{}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/issues/"+issueID.String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", issueID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestTransitionStatus_Handler_Success(t *testing.T) {
	h, issueRepo, _, historyRepo, _ := newTestIssueHandler()
	issueID := uuid.New()
	mockTx := newMockTxForHandler()

	issueRepo.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
		return &model.Issue{
			ID:        issueID,
			IssueKey:  "PROJ-1",
			ProjectID: uuid.New(),
			Type:      "bug",
			Title:     "Test",
			Priority:  "P2",
			Status:    "todo",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil
	}
	issueRepo.BeginTxFn = func(ctx context.Context) (pgx.Tx, error) {
		return mockTx, nil
	}
	issueRepo.UpdateFn = func(ctx context.Context, tx pgx.Tx, id uuid.UUID, setClauses []string, args []interface{}) (*model.Issue, error) {
		return &model.Issue{
			ID:        issueID,
			IssueKey:  "PROJ-1",
			ProjectID: uuid.New(),
			Type:      "bug",
			Title:     "Test",
			Priority:  "P2",
			Status:    "in_progress",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil
	}
	historyRepo.CreateFn = func(ctx context.Context, tx pgx.Tx, iid uuid.UUID, fieldName string, oldValue, newValue *string, operatorID *string) error {
		return nil
	}

	body, _ := json.Marshal(model.TransitionStatusRequest{Status: "in_progress"})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/issues/"+issueID.String()+"/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", issueID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.TransitionStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestTransitionStatus_Handler_InvalidTransition(t *testing.T) {
	h, issueRepo, _, _, _ := newTestIssueHandler()
	issueID := uuid.New()

	issueRepo.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
		return &model.Issue{
			ID:        issueID,
			Status:    "todo",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil
	}

	body, _ := json.Marshal(model.TransitionStatusRequest{Status: "done"})
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/issues/%s/status", issueID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", issueID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.TransitionStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d; body: %s", w.Code, w.Body.String())
	}
}
