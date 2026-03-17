package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository/mocks"
)

func TestGetSummary_Success(t *testing.T) {
	projectID := uuid.New()

	issueRepo := &mocks.MockIssueRepo{
		CountByStatusFn: func(ctx context.Context, pid uuid.UUID) (map[string]int, error) {
			return map[string]int{"todo": 5, "in_progress": 3, "done": 2}, nil
		},
		CountByPriorityFn: func(ctx context.Context, pid uuid.UUID) (map[string]int, error) {
			return map[string]int{"P0": 2, "P1": 3, "P2": 5}, nil
		},
		CountByTypeFn: func(ctx context.Context, pid uuid.UUID) (map[string]int, error) {
			return map[string]int{"bug": 4, "requirement": 6}, nil
		},
	}

	svc := NewProgressService(issueRepo)
	summary, err := svc.GetSummary(context.Background(), projectID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if summary.Total != 10 {
		t.Errorf("expected total 10, got %d", summary.Total)
	}
	if summary.StatusCounts["todo"] != 5 {
		t.Errorf("expected todo count 5, got %d", summary.StatusCounts["todo"])
	}
	if summary.PriorityCounts["P0"] != 2 {
		t.Errorf("expected P0 count 2, got %d", summary.PriorityCounts["P0"])
	}
	if summary.TypeCounts["bug"] != 4 {
		t.Errorf("expected bug count 4, got %d", summary.TypeCounts["bug"])
	}
}

func TestGetTrend_Success(t *testing.T) {
	projectID := uuid.New()

	issueRepo := &mocks.MockIssueRepo{
		GetTrendFn: func(ctx context.Context, pid uuid.UUID, days int) ([]model.TrendPoint, error) {
			if days != 30 {
				t.Errorf("expected days 30, got %d", days)
			}
			return []model.TrendPoint{
				{Date: "2024-01-01", Created: 3, Done: 1},
				{Date: "2024-01-02", Created: 2, Done: 2},
			}, nil
		},
	}

	svc := NewProgressService(issueRepo)
	trend, err := svc.GetTrend(context.Background(), projectID, 30)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(trend) != 2 {
		t.Errorf("expected 2 trend points, got %d", len(trend))
	}
	if trend[0].Created != 3 {
		t.Errorf("expected first point created 3, got %d", trend[0].Created)
	}
}
