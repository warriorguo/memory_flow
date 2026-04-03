package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository/mocks"
)

func newTestDepService(depRepo *mocks.MockDependencyRepo, issueRepo *mocks.MockIssueRepo, projectRepo *mocks.MockProjectRepo) *DependencyService {
	return NewDependencyService(depRepo, issueRepo, projectRepo)
}

func TestCreateDependency_Success(t *testing.T) {
	sourceID := uuid.New()
	targetID := uuid.New()

	issueRepo := &mocks.MockIssueRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
			return &model.Issue{
				ID:        id,
				IssueKey:  "PROJ-1",
				ProjectID: uuid.New(),
				Priority:  "P2",
				Status:    "todo",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, nil
		},
	}

	depRepo := &mocks.MockDependencyRepo{
		HasPathFn: func(ctx context.Context, src, tgt uuid.UUID) (bool, error) {
			return false, nil
		},
		CreateFn: func(ctx context.Context, dep model.IssueDependency) (*model.IssueDependency, error) {
			dep.ID = uuid.New()
			dep.CreatedAt = time.Now()
			return &dep, nil
		},
	}

	projectRepo := &mocks.MockProjectRepo{}
	svc := newTestDepService(depRepo, issueRepo, projectRepo)

	dep, err := svc.Create(context.Background(), sourceID, model.CreateDependencyRequest{
		TargetIssueID: targetID,
		Type:          "depends_on",
		Severity:      "critical",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dep.SourceIssueID != sourceID {
		t.Errorf("expected source %v, got %v", sourceID, dep.SourceIssueID)
	}
	if dep.TargetIssueID != targetID {
		t.Errorf("expected target %v, got %v", targetID, dep.TargetIssueID)
	}
	if dep.Type != "depends_on" {
		t.Errorf("expected type depends_on, got %s", dep.Type)
	}
	if dep.Severity != "critical" {
		t.Errorf("expected severity critical, got %s", dep.Severity)
	}
}

func TestCreateDependency_SelfReference(t *testing.T) {
	id := uuid.New()
	svc := newTestDepService(&mocks.MockDependencyRepo{}, &mocks.MockIssueRepo{}, &mocks.MockProjectRepo{})

	_, err := svc.Create(context.Background(), id, model.CreateDependencyRequest{
		TargetIssueID: id,
		Type:          "depends_on",
		Severity:      "critical",
	})
	if err == nil {
		t.Fatal("expected error for self-reference")
	}
	if !strings.Contains(err.Error(), "cannot depend on itself") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateDependency_InvalidType(t *testing.T) {
	svc := newTestDepService(&mocks.MockDependencyRepo{}, &mocks.MockIssueRepo{}, &mocks.MockProjectRepo{})

	_, err := svc.Create(context.Background(), uuid.New(), model.CreateDependencyRequest{
		TargetIssueID: uuid.New(),
		Type:          "related",
		Severity:      "critical",
	})
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "invalid dependency type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateDependency_InvalidSeverity(t *testing.T) {
	svc := newTestDepService(&mocks.MockDependencyRepo{}, &mocks.MockIssueRepo{}, &mocks.MockProjectRepo{})

	_, err := svc.Create(context.Background(), uuid.New(), model.CreateDependencyRequest{
		TargetIssueID: uuid.New(),
		Type:          "depends_on",
		Severity:      "low",
	})
	if err == nil {
		t.Fatal("expected error for invalid severity")
	}
	if !strings.Contains(err.Error(), "invalid severity") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateDependency_CycleDetected(t *testing.T) {
	sourceID := uuid.New()
	targetID := uuid.New()

	issueRepo := &mocks.MockIssueRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
			return &model.Issue{
				ID:        id,
				IssueKey:  "PROJ-1",
				ProjectID: uuid.New(),
				Priority:  "P2",
				Status:    "todo",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, nil
		},
	}

	depRepo := &mocks.MockDependencyRepo{
		HasPathFn: func(ctx context.Context, src, tgt uuid.UUID) (bool, error) {
			// Simulate: target already has a path to source
			return true, nil
		},
	}

	svc := newTestDepService(depRepo, issueRepo, &mocks.MockProjectRepo{})

	_, err := svc.Create(context.Background(), sourceID, model.CreateDependencyRequest{
		TargetIssueID: targetID,
		Type:          "depends_on",
		Severity:      "critical",
	})
	if err == nil {
		t.Fatal("expected error for cycle")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateDependency_SourceNotFound(t *testing.T) {
	sourceID := uuid.New()

	issueRepo := &mocks.MockIssueRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
			if id == sourceID {
				return nil, nil
			}
			return &model.Issue{ID: id}, nil
		},
	}

	svc := newTestDepService(&mocks.MockDependencyRepo{}, issueRepo, &mocks.MockProjectRepo{})

	_, err := svc.Create(context.Background(), sourceID, model.CreateDependencyRequest{
		TargetIssueID: uuid.New(),
		Type:          "depends_on",
		Severity:      "critical",
	})
	if err == nil {
		t.Fatal("expected error for source not found")
	}
	if !strings.Contains(err.Error(), "source issue not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetEffectivePriority_NoCriticalDeps(t *testing.T) {
	issueID := uuid.New()

	issueRepo := &mocks.MockIssueRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
			return &model.Issue{
				ID:       id,
				Priority: "P2",
			}, nil
		},
	}

	depRepo := &mocks.MockDependencyRepo{
		GetDependsOnFn: func(ctx context.Context, id uuid.UUID) ([]model.IssueDependency, error) {
			return nil, nil
		},
	}

	svc := newTestDepService(depRepo, issueRepo, &mocks.MockProjectRepo{})

	priority, err := svc.GetEffectivePriority(context.Background(), issueID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if priority != "P2" {
		t.Errorf("expected P2, got %s", priority)
	}
}

func TestGetEffectivePriority_InheritsCritical(t *testing.T) {
	issueA := uuid.New()
	issueB := uuid.New()

	issueRepo := &mocks.MockIssueRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
			if id == issueA {
				return &model.Issue{ID: id, Priority: "P2"}, nil
			}
			return &model.Issue{ID: id, Priority: "P0"}, nil
		},
	}

	depRepo := &mocks.MockDependencyRepo{
		GetDependsOnFn: func(ctx context.Context, id uuid.UUID) ([]model.IssueDependency, error) {
			if id == issueA {
				return []model.IssueDependency{
					{SourceIssueID: issueA, TargetIssueID: issueB, Type: "depends_on", Severity: "critical"},
				}, nil
			}
			return nil, nil
		},
	}

	svc := newTestDepService(depRepo, issueRepo, &mocks.MockProjectRepo{})

	priority, err := svc.GetEffectivePriority(context.Background(), issueA)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if priority != "P0" {
		t.Errorf("expected P0 (inherited from critical dep), got %s", priority)
	}
}

func TestGetEffectivePriority_IgnoresRecommended(t *testing.T) {
	issueA := uuid.New()
	issueB := uuid.New()

	issueRepo := &mocks.MockIssueRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
			if id == issueA {
				return &model.Issue{ID: id, Priority: "P2"}, nil
			}
			return &model.Issue{ID: id, Priority: "P0"}, nil
		},
	}

	depRepo := &mocks.MockDependencyRepo{
		GetDependsOnFn: func(ctx context.Context, id uuid.UUID) ([]model.IssueDependency, error) {
			if id == issueA {
				return []model.IssueDependency{
					{SourceIssueID: issueA, TargetIssueID: issueB, Type: "depends_on", Severity: "recommended"},
				}, nil
			}
			return nil, nil
		},
	}

	svc := newTestDepService(depRepo, issueRepo, &mocks.MockProjectRepo{})

	priority, err := svc.GetEffectivePriority(context.Background(), issueA)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if priority != "P2" {
		t.Errorf("expected P2 (recommended deps don't affect priority), got %s", priority)
	}
}

func TestGetEffectivePriority_MultiLevel(t *testing.T) {
	issueA := uuid.New()
	issueB := uuid.New()
	issueC := uuid.New()

	issueRepo := &mocks.MockIssueRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
			switch id {
			case issueA:
				return &model.Issue{ID: id, Priority: "P2"}, nil
			case issueB:
				return &model.Issue{ID: id, Priority: "P1"}, nil
			case issueC:
				return &model.Issue{ID: id, Priority: "P0"}, nil
			}
			return nil, nil
		},
	}

	depRepo := &mocks.MockDependencyRepo{
		GetDependsOnFn: func(ctx context.Context, id uuid.UUID) ([]model.IssueDependency, error) {
			switch id {
			case issueA:
				return []model.IssueDependency{
					{SourceIssueID: issueA, TargetIssueID: issueB, Type: "depends_on", Severity: "critical"},
				}, nil
			case issueB:
				return []model.IssueDependency{
					{SourceIssueID: issueB, TargetIssueID: issueC, Type: "depends_on", Severity: "critical"},
				}, nil
			}
			return nil, nil
		},
	}

	svc := newTestDepService(depRepo, issueRepo, &mocks.MockProjectRepo{})

	// A -> B -> C (all critical), C is P0, so effective priority of A should be P0
	priority, err := svc.GetEffectivePriority(context.Background(), issueA)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if priority != "P0" {
		t.Errorf("expected P0 (multi-level inheritance), got %s", priority)
	}
}
