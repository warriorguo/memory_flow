package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
)

type MemoryService struct {
	repo repository.MemoryRepository
}

func NewMemoryService(repo repository.MemoryRepository) *MemoryService {
	return &MemoryService{repo: repo}
}

var validMemoryTypes = map[string]bool{"recall": true, "write": true}

func (s *MemoryService) Create(ctx context.Context, req model.CreateMemoryRequest) (*model.Memory, error) {
	if req.Type == "" {
		return nil, fmt.Errorf("memory type is required")
	}
	if !validMemoryTypes[req.Type] {
		return nil, fmt.Errorf("invalid memory type: must be 'recall' or 'write'")
	}
	if req.Title == "" {
		return nil, fmt.Errorf("memory title is required")
	}
	if req.Content == "" {
		return nil, fmt.Errorf("memory content is required")
	}
	return s.repo.Create(ctx, req)
}

func (s *MemoryService) GetByID(ctx context.Context, id uuid.UUID) (*model.Memory, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *MemoryService) List(ctx context.Context, filter model.MemoryFilter) ([]model.Memory, int, error) {
	return s.repo.List(ctx, filter)
}

func (s *MemoryService) Update(ctx context.Context, id uuid.UUID, req model.UpdateMemoryRequest) (*model.Memory, error) {
	if req.Type != nil && !validMemoryTypes[*req.Type] {
		return nil, fmt.Errorf("invalid memory type: must be 'recall' or 'write'")
	}
	return s.repo.Update(ctx, id, req)
}

func (s *MemoryService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
