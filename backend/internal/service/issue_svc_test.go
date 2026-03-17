package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository/mocks"
)

func newMockTx() pgx.Tx {
	return &mocks.MockTx{
		CommitFn:   func(ctx context.Context) error { return nil },
		RollbackFn: func(ctx context.Context) error { return nil },
	}
}

func newTestIssueService(issueRepo *mocks.MockIssueRepo, projectRepo *mocks.MockProjectRepo, historyRepo *mocks.MockIssueHistoryRepo) *IssueService {
	return NewIssueService(issueRepo, projectRepo, historyRepo)
}

func TestCreateIssue_Success(t *testing.T) {
	projectID := uuid.New()
	mockTx := newMockTx()

	projectRepo := &mocks.MockProjectRepo{
		IncrementIssueNumberFn: func(ctx context.Context, tx pgx.Tx, id uuid.UUID) (int, string, error) {
			return 1, "PROJ", nil
		},
	}

	issueRepo := &mocks.MockIssueRepo{
		BeginTxFn: func(ctx context.Context) (pgx.Tx, error) {
			return mockTx, nil
		},
		CreateFn: func(ctx context.Context, tx pgx.Tx, issueKey string, pid uuid.UUID, req model.CreateIssueRequest) (*model.Issue, error) {
			if issueKey != "PROJ-1" {
				t.Errorf("expected issue key PROJ-1, got %s", issueKey)
			}
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
		},
	}

	historyRepo := &mocks.MockIssueHistoryRepo{}

	svc := newTestIssueService(issueRepo, projectRepo, historyRepo)
	issue, err := svc.Create(context.Background(), projectID, model.CreateIssueRequest{
		Type:  "bug",
		Title: "Test Bug",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if issue.IssueKey != "PROJ-1" {
		t.Errorf("expected issue key PROJ-1, got %s", issue.IssueKey)
	}
	if issue.Type != "bug" {
		t.Errorf("expected type bug, got %s", issue.Type)
	}
}

func TestCreateIssue_MissingTitle(t *testing.T) {
	svc := newTestIssueService(&mocks.MockIssueRepo{}, &mocks.MockProjectRepo{}, &mocks.MockIssueHistoryRepo{})

	_, err := svc.Create(context.Background(), uuid.New(), model.CreateIssueRequest{
		Type: "bug",
	})
	if err == nil {
		t.Fatal("expected error for missing title")
	}
	if !strings.Contains(err.Error(), "title is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateIssue_InvalidType(t *testing.T) {
	svc := newTestIssueService(&mocks.MockIssueRepo{}, &mocks.MockProjectRepo{}, &mocks.MockIssueHistoryRepo{})

	_, err := svc.Create(context.Background(), uuid.New(), model.CreateIssueRequest{
		Type:  "feature",
		Title: "Test",
	})
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "invalid issue type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateIssue_InvalidPriority(t *testing.T) {
	svc := newTestIssueService(&mocks.MockIssueRepo{}, &mocks.MockProjectRepo{}, &mocks.MockIssueHistoryRepo{})

	badPriority := "P5"
	_, err := svc.Create(context.Background(), uuid.New(), model.CreateIssueRequest{
		Type:     "bug",
		Title:    "Test",
		Priority: &badPriority,
	})
	if err == nil {
		t.Fatal("expected error for invalid priority")
	}
	if !strings.Contains(err.Error(), "invalid priority") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpdateIssue_Success(t *testing.T) {
	issueID := uuid.New()
	mockTx := newMockTx()
	existingIssue := &model.Issue{
		ID:        issueID,
		IssueKey:  "PROJ-1",
		ProjectID: uuid.New(),
		Type:      "bug",
		Title:     "Old Title",
		Priority:  "P2",
		Status:    "todo",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	newTitle := "New Title"

	issueRepo := &mocks.MockIssueRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
			return existingIssue, nil
		},
		BeginTxFn: func(ctx context.Context) (pgx.Tx, error) {
			return mockTx, nil
		},
		UpdateFn: func(ctx context.Context, tx pgx.Tx, id uuid.UUID, setClauses []string, args []interface{}) (*model.Issue, error) {
			updated := *existingIssue
			updated.Title = newTitle
			return &updated, nil
		},
	}

	var historyCreated bool
	historyRepo := &mocks.MockIssueHistoryRepo{
		CreateFn: func(ctx context.Context, tx pgx.Tx, iid uuid.UUID, fieldName string, oldValue, newValue *string, operatorID *string) error {
			historyCreated = true
			if fieldName != "title" {
				t.Errorf("expected field_name title, got %s", fieldName)
			}
			if *oldValue != "Old Title" {
				t.Errorf("expected old value Old Title, got %s", *oldValue)
			}
			if *newValue != "New Title" {
				t.Errorf("expected new value New Title, got %s", *newValue)
			}
			return nil
		},
	}

	projectRepo := &mocks.MockProjectRepo{}
	svc := newTestIssueService(issueRepo, projectRepo, historyRepo)

	issue, err := svc.Update(context.Background(), issueID, model.UpdateIssueRequest{
		Title: &newTitle,
	}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if issue.Title != "New Title" {
		t.Errorf("expected title New Title, got %s", issue.Title)
	}
	if !historyCreated {
		t.Error("expected history record to be created")
	}
}

func TestUpdateIssue_TracksAllFieldChanges(t *testing.T) {
	issueID := uuid.New()
	mockTx := newMockTx()

	oldDesc := "old desc"
	oldAssignee := "user1"
	existingIssue := &model.Issue{
		ID:          issueID,
		IssueKey:    "PROJ-1",
		ProjectID:   uuid.New(),
		Type:        "bug",
		Title:       "Old Title",
		Description: &oldDesc,
		Priority:    "P2",
		Status:      "todo",
		AssigneeID:  &oldAssignee,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	trackedFields := make(map[string]bool)

	issueRepo := &mocks.MockIssueRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
			return existingIssue, nil
		},
		BeginTxFn: func(ctx context.Context) (pgx.Tx, error) {
			return mockTx, nil
		},
		UpdateFn: func(ctx context.Context, tx pgx.Tx, id uuid.UUID, setClauses []string, args []interface{}) (*model.Issue, error) {
			return existingIssue, nil
		},
	}

	historyRepo := &mocks.MockIssueHistoryRepo{
		CreateFn: func(ctx context.Context, tx pgx.Tx, iid uuid.UUID, fieldName string, oldValue, newValue *string, operatorID *string) error {
			trackedFields[fieldName] = true
			return nil
		},
	}

	projectRepo := &mocks.MockProjectRepo{}
	svc := newTestIssueService(issueRepo, projectRepo, historyRepo)

	newTitle := "New Title"
	newType := "requirement"
	newPriority := "P0"
	newDesc := "new desc"
	newAssignee := "user2"

	_, err := svc.Update(context.Background(), issueID, model.UpdateIssueRequest{
		Title:       &newTitle,
		Type:        &newType,
		Priority:    &newPriority,
		Description: &newDesc,
		AssigneeID:  &newAssignee,
	}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedFields := []string{"title", "type", "priority", "description", "assignee_id"}
	for _, f := range expectedFields {
		if !trackedFields[f] {
			t.Errorf("expected history tracking for field %s", f)
		}
	}
}

func TestTransitionStatus_ValidTransition(t *testing.T) {
	issueID := uuid.New()
	mockTx := newMockTx()

	issueRepo := &mocks.MockIssueRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
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
		},
		BeginTxFn: func(ctx context.Context) (pgx.Tx, error) {
			return mockTx, nil
		},
		UpdateFn: func(ctx context.Context, tx pgx.Tx, id uuid.UUID, setClauses []string, args []interface{}) (*model.Issue, error) {
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
		},
	}

	historyRepo := &mocks.MockIssueHistoryRepo{
		CreateFn: func(ctx context.Context, tx pgx.Tx, iid uuid.UUID, fieldName string, oldValue, newValue *string, operatorID *string) error {
			return nil
		},
	}

	projectRepo := &mocks.MockProjectRepo{}
	svc := newTestIssueService(issueRepo, projectRepo, historyRepo)

	issue, err := svc.TransitionStatus(context.Background(), issueID, "in_progress", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if issue.Status != "in_progress" {
		t.Errorf("expected status in_progress, got %s", issue.Status)
	}
}

func TestTransitionStatus_InvalidTransition(t *testing.T) {
	issueID := uuid.New()

	issueRepo := &mocks.MockIssueRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
			return &model.Issue{
				ID:        issueID,
				Status:    "todo",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, nil
		},
	}

	historyRepo := &mocks.MockIssueHistoryRepo{}
	projectRepo := &mocks.MockProjectRepo{}
	svc := newTestIssueService(issueRepo, projectRepo, historyRepo)

	_, err := svc.TransitionStatus(context.Background(), issueID, "done", nil)
	if err == nil {
		t.Fatal("expected error for invalid transition from todo to done")
	}
	if !strings.Contains(err.Error(), "cannot transition") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTransitionStatus_FromClosedNotAllowed(t *testing.T) {
	issueID := uuid.New()

	issueRepo := &mocks.MockIssueRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
			return &model.Issue{
				ID:        issueID,
				Status:    "closed",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, nil
		},
	}

	historyRepo := &mocks.MockIssueHistoryRepo{}
	projectRepo := &mocks.MockProjectRepo{}
	svc := newTestIssueService(issueRepo, projectRepo, historyRepo)

	_, err := svc.TransitionStatus(context.Background(), issueID, "todo", nil)
	if err == nil {
		t.Fatal("expected error for transition from closed")
	}
	if !strings.Contains(err.Error(), "cannot transition from status: closed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTransitionStatus_AllValidPaths(t *testing.T) {
	tests := []struct {
		from string
		to   string
	}{
		{"todo", "in_progress"},
		{"todo", "rejected"},
		{"in_progress", "review"},
		{"in_progress", "todo"},
		{"review", "testing"},
		{"review", "in_progress"},
		{"testing", "done"},
		{"testing", "in_progress"},
		{"done", "closed"},
		{"done", "in_progress"},
		{"rejected", "todo"},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s_to_%s", tc.from, tc.to), func(t *testing.T) {
			issueID := uuid.New()
			mockTx := newMockTx()

			issueRepo := &mocks.MockIssueRepo{
				GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
					return &model.Issue{
						ID:        issueID,
						Status:    tc.from,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					}, nil
				},
				BeginTxFn: func(ctx context.Context) (pgx.Tx, error) {
					return mockTx, nil
				},
				UpdateFn: func(ctx context.Context, tx pgx.Tx, id uuid.UUID, setClauses []string, args []interface{}) (*model.Issue, error) {
					return &model.Issue{
						ID:        issueID,
						Status:    tc.to,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					}, nil
				},
			}

			historyRepo := &mocks.MockIssueHistoryRepo{
				CreateFn: func(ctx context.Context, tx pgx.Tx, iid uuid.UUID, fieldName string, oldValue, newValue *string, operatorID *string) error {
					return nil
				},
			}

			projectRepo := &mocks.MockProjectRepo{}
			svc := newTestIssueService(issueRepo, projectRepo, historyRepo)

			issue, err := svc.TransitionStatus(context.Background(), issueID, tc.to, nil)
			if err != nil {
				t.Fatalf("expected transition %s -> %s to succeed, got error: %v", tc.from, tc.to, err)
			}
			if issue.Status != tc.to {
				t.Errorf("expected status %s, got %s", tc.to, issue.Status)
			}
		})
	}
}
