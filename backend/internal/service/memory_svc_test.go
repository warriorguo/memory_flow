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

func newTestMemoryService(repo *mocks.MockMemoryRepo) *MemoryService {
	return NewMemoryService(repo)
}

func TestCreateMemory_Success(t *testing.T) {
	repo := &mocks.MockMemoryRepo{
		CreateFn: func(ctx context.Context, req model.CreateMemoryRequest) (*model.MemoryResponse, error) {
			return &model.MemoryResponse{
				ID:        uuid.New(),
				Type:      req.Type,
				Title:     req.Title,
				Content:   req.Content,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestMemoryService(repo)
	memory, err := svc.Create(context.Background(), model.CreateMemoryRequest{
		Type:    "recall",
		Title:   "Test Memory",
		Content: "Some content",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if memory.Type != "recall" {
		t.Errorf("expected type recall, got %s", memory.Type)
	}
	if memory.Title != "Test Memory" {
		t.Errorf("expected title Test Memory, got %s", memory.Title)
	}
}

func TestCreateMemory_MissingTitle(t *testing.T) {
	repo := &mocks.MockMemoryRepo{}
	svc := newTestMemoryService(repo)

	_, err := svc.Create(context.Background(), model.CreateMemoryRequest{
		Type:    "recall",
		Content: "Some content",
	})
	if err == nil {
		t.Fatal("expected error for missing title")
	}
	if !strings.Contains(err.Error(), "title is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateMemory_InvalidType(t *testing.T) {
	tests := []struct {
		name     string
		memType  string
	}{
		{"empty type", ""},
		{"invalid type", "note"},
	}

	repo := &mocks.MockMemoryRepo{}
	svc := newTestMemoryService(repo)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), model.CreateMemoryRequest{
				Type:    tc.memType,
				Title:   "Test",
				Content: "content",
			})
			if err == nil {
				t.Fatal("expected error for invalid type")
			}
		})
	}
}

func TestUpdateMemory_Success(t *testing.T) {
	id := uuid.New()
	newTitle := "Updated Title"
	repo := &mocks.MockMemoryRepo{
		UpdateFn: func(ctx context.Context, uid uuid.UUID, req model.UpdateMemoryRequest) (*model.MemoryResponse, error) {
			return &model.MemoryResponse{
				ID:        uid,
				Type:      "recall",
				Title:     *req.Title,
				Content:   "content",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestMemoryService(repo)
	memory, err := svc.Update(context.Background(), id, model.UpdateMemoryRequest{
		Title: &newTitle,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if memory.Title != "Updated Title" {
		t.Errorf("expected title Updated Title, got %s", memory.Title)
	}
}

func TestUpdateMemory_InvalidType(t *testing.T) {
	repo := &mocks.MockMemoryRepo{}
	svc := newTestMemoryService(repo)

	badType := "invalid"
	_, err := svc.Update(context.Background(), uuid.New(), model.UpdateMemoryRequest{
		Type: &badType,
	})
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "invalid memory type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteMemory_Success(t *testing.T) {
	id := uuid.New()
	var deleted bool
	repo := &mocks.MockMemoryRepo{
		DeleteFn: func(ctx context.Context, uid uuid.UUID) error {
			if uid != id {
				t.Errorf("expected id %v, got %v", id, uid)
			}
			deleted = true
			return nil
		},
	}

	svc := newTestMemoryService(repo)
	err := svc.Delete(context.Background(), id)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !deleted {
		t.Error("expected delete to be called")
	}
}
