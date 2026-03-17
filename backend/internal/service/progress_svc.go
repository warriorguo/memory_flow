package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
)

type ProgressService struct {
	issueRepo repository.IssueRepository
}

func NewProgressService(issueRepo repository.IssueRepository) *ProgressService {
	return &ProgressService{issueRepo: issueRepo}
}

func (s *ProgressService) GetSummary(ctx context.Context, projectID uuid.UUID) (*model.ProgressSummary, error) {
	statusCounts, err := s.issueRepo.CountByStatus(ctx, projectID)
	if err != nil {
		return nil, err
	}

	priorityCounts, err := s.issueRepo.CountByPriority(ctx, projectID)
	if err != nil {
		return nil, err
	}

	typeCounts, err := s.issueRepo.CountByType(ctx, projectID)
	if err != nil {
		return nil, err
	}

	total := 0
	for _, c := range statusCounts {
		total += c
	}

	return &model.ProgressSummary{
		StatusCounts:   statusCounts,
		PriorityCounts: priorityCounts,
		TypeCounts:     typeCounts,
		Total:          total,
	}, nil
}

func (s *ProgressService) GetTrend(ctx context.Context, projectID uuid.UUID, days int) ([]model.TrendPoint, error) {
	if days <= 0 {
		days = 30
	}
	return s.issueRepo.GetTrend(ctx, projectID, days)
}
