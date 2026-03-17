package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

// ProjectRepository defines the interface for project data access.
type ProjectRepository interface {
	Create(ctx context.Context, req model.CreateProjectRequest) (*model.Project, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error)
	List(ctx context.Context, filter model.ProjectFilter) ([]model.Project, int, error)
	Update(ctx context.Context, id uuid.UUID, req model.UpdateProjectRequest) (*model.Project, error)
	Archive(ctx context.Context, id uuid.UUID) error
	IncrementIssueNumber(ctx context.Context, tx pgx.Tx, id uuid.UUID) (int, string, error)
}

// IssueRepository defines the interface for issue data access.
type IssueRepository interface {
	Create(ctx context.Context, tx pgx.Tx, issueKey string, projectID uuid.UUID, req model.CreateIssueRequest) (*model.Issue, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Issue, error)
	List(ctx context.Context, filter model.IssueFilter) ([]model.Issue, int, error)
	Update(ctx context.Context, tx pgx.Tx, id uuid.UUID, setClauses []string, args []interface{}) (*model.Issue, error)
	CountByStatus(ctx context.Context, projectID uuid.UUID) (map[string]int, error)
	CountByPriority(ctx context.Context, projectID uuid.UUID) (map[string]int, error)
	CountByType(ctx context.Context, projectID uuid.UUID) (map[string]int, error)
	GetTrend(ctx context.Context, projectID uuid.UUID, days int) ([]model.TrendPoint, error)
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

// IssueHistoryRepository defines the interface for issue history data access.
type IssueHistoryRepository interface {
	Create(ctx context.Context, tx pgx.Tx, issueID uuid.UUID, fieldName string, oldValue, newValue *string, operatorID *string) error
	ListByIssueID(ctx context.Context, issueID uuid.UUID) ([]model.IssueHistory, error)
}

// MemoryRepository defines the interface for memory data access.
type MemoryRepository interface {
	Create(ctx context.Context, req model.CreateMemoryRequest) (*model.Memory, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Memory, error)
	List(ctx context.Context, filter model.MemoryFilter) ([]model.Memory, int, error)
	Update(ctx context.Context, id uuid.UUID, req model.UpdateMemoryRequest) (*model.Memory, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// TagRepository defines the interface for tag data access.
type TagRepository interface {
	Create(ctx context.Context, req model.CreateTagRequest) (*model.Tag, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Tag, error)
	List(ctx context.Context) ([]model.Tag, error)
	AddToIssue(ctx context.Context, issueID, tagID uuid.UUID) error
	RemoveFromIssue(ctx context.Context, issueID, tagID uuid.UUID) error
	GetByIssueID(ctx context.Context, issueID uuid.UUID) ([]model.Tag, error)
	AddToMemory(ctx context.Context, memoryID, tagID uuid.UUID) error
	RemoveFromMemory(ctx context.Context, memoryID, tagID uuid.UUID) error
	GetByMemoryID(ctx context.Context, memoryID uuid.UUID) ([]model.Tag, error)
}

// UserRepository defines the interface for user data access.
type UserRepository interface {
	Create(ctx context.Context, username, passwordHash string, displayName *string, role string) (*model.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
}
