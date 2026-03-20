package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

// MockProjectRepo is a mock implementation of repository.ProjectRepository.
type MockProjectRepo struct {
	CreateFn               func(ctx context.Context, req model.CreateProjectRequest) (*model.Project, error)
	GetByIDFn              func(ctx context.Context, id uuid.UUID) (*model.Project, error)
	ListFn                 func(ctx context.Context, filter model.ProjectFilter) ([]model.Project, int, error)
	UpdateFn               func(ctx context.Context, id uuid.UUID, req model.UpdateProjectRequest) (*model.Project, error)
	ArchiveFn              func(ctx context.Context, id uuid.UUID) error
	IncrementIssueNumberFn func(ctx context.Context, tx pgx.Tx, id uuid.UUID) (int, string, error)
}

func (m *MockProjectRepo) Create(ctx context.Context, req model.CreateProjectRequest) (*model.Project, error) {
	return m.CreateFn(ctx, req)
}
func (m *MockProjectRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	return m.GetByIDFn(ctx, id)
}
func (m *MockProjectRepo) List(ctx context.Context, filter model.ProjectFilter) ([]model.Project, int, error) {
	return m.ListFn(ctx, filter)
}
func (m *MockProjectRepo) Update(ctx context.Context, id uuid.UUID, req model.UpdateProjectRequest) (*model.Project, error) {
	return m.UpdateFn(ctx, id, req)
}
func (m *MockProjectRepo) Archive(ctx context.Context, id uuid.UUID) error {
	return m.ArchiveFn(ctx, id)
}
func (m *MockProjectRepo) IncrementIssueNumber(ctx context.Context, tx pgx.Tx, id uuid.UUID) (int, string, error) {
	return m.IncrementIssueNumberFn(ctx, tx, id)
}

// MockIssueRepo is a mock implementation of repository.IssueRepository.
type MockIssueRepo struct {
	CreateFn          func(ctx context.Context, tx pgx.Tx, issueKey string, projectID uuid.UUID, req model.CreateIssueRequest) (*model.Issue, error)
	GetByIDFn         func(ctx context.Context, id uuid.UUID) (*model.Issue, error)
	GetByKeyFn        func(ctx context.Context, key string) (*model.Issue, error)
	ListFn            func(ctx context.Context, filter model.IssueFilter) ([]model.Issue, int, error)
	UpdateFn          func(ctx context.Context, tx pgx.Tx, id uuid.UUID, setClauses []string, args []interface{}) (*model.Issue, error)
	CountByStatusFn   func(ctx context.Context, projectID uuid.UUID) (map[string]int, error)
	CountByPriorityFn func(ctx context.Context, projectID uuid.UUID) (map[string]int, error)
	CountByTypeFn     func(ctx context.Context, projectID uuid.UUID) (map[string]int, error)
	GetTrendFn        func(ctx context.Context, projectID uuid.UUID, days int) ([]model.TrendPoint, error)
	BeginTxFn         func(ctx context.Context) (pgx.Tx, error)
}

func (m *MockIssueRepo) Create(ctx context.Context, tx pgx.Tx, issueKey string, projectID uuid.UUID, req model.CreateIssueRequest) (*model.Issue, error) {
	return m.CreateFn(ctx, tx, issueKey, projectID, req)
}
func (m *MockIssueRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
	return m.GetByIDFn(ctx, id)
}
func (m *MockIssueRepo) GetByKey(ctx context.Context, key string) (*model.Issue, error) {
	return m.GetByKeyFn(ctx, key)
}
func (m *MockIssueRepo) List(ctx context.Context, filter model.IssueFilter) ([]model.Issue, int, error) {
	return m.ListFn(ctx, filter)
}
func (m *MockIssueRepo) Update(ctx context.Context, tx pgx.Tx, id uuid.UUID, setClauses []string, args []interface{}) (*model.Issue, error) {
	return m.UpdateFn(ctx, tx, id, setClauses, args)
}
func (m *MockIssueRepo) CountByStatus(ctx context.Context, projectID uuid.UUID) (map[string]int, error) {
	return m.CountByStatusFn(ctx, projectID)
}
func (m *MockIssueRepo) CountByPriority(ctx context.Context, projectID uuid.UUID) (map[string]int, error) {
	return m.CountByPriorityFn(ctx, projectID)
}
func (m *MockIssueRepo) CountByType(ctx context.Context, projectID uuid.UUID) (map[string]int, error) {
	return m.CountByTypeFn(ctx, projectID)
}
func (m *MockIssueRepo) GetTrend(ctx context.Context, projectID uuid.UUID, days int) ([]model.TrendPoint, error) {
	return m.GetTrendFn(ctx, projectID, days)
}
func (m *MockIssueRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return m.BeginTxFn(ctx)
}

// MockIssueHistoryRepo is a mock implementation of repository.IssueHistoryRepository.
type MockIssueHistoryRepo struct {
	CreateFn        func(ctx context.Context, tx pgx.Tx, issueID uuid.UUID, fieldName string, oldValue, newValue *string, operatorID *string) error
	ListByIssueIDFn func(ctx context.Context, issueID uuid.UUID) ([]model.IssueHistory, error)
}

func (m *MockIssueHistoryRepo) Create(ctx context.Context, tx pgx.Tx, issueID uuid.UUID, fieldName string, oldValue, newValue *string, operatorID *string) error {
	return m.CreateFn(ctx, tx, issueID, fieldName, oldValue, newValue, operatorID)
}
func (m *MockIssueHistoryRepo) ListByIssueID(ctx context.Context, issueID uuid.UUID) ([]model.IssueHistory, error) {
	return m.ListByIssueIDFn(ctx, issueID)
}

// MockMemoryRepo is a mock implementation of repository.MemoryRepository.
type MockMemoryRepo struct {
	CreateFn  func(ctx context.Context, req model.CreateMemoryRequest) (*model.Memory, error)
	GetByIDFn func(ctx context.Context, id uuid.UUID) (*model.Memory, error)
	ListFn    func(ctx context.Context, filter model.MemoryFilter) ([]model.Memory, int, error)
	UpdateFn  func(ctx context.Context, id uuid.UUID, req model.UpdateMemoryRequest) (*model.Memory, error)
	DeleteFn  func(ctx context.Context, id uuid.UUID) error
}

func (m *MockMemoryRepo) Create(ctx context.Context, req model.CreateMemoryRequest) (*model.Memory, error) {
	return m.CreateFn(ctx, req)
}
func (m *MockMemoryRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Memory, error) {
	return m.GetByIDFn(ctx, id)
}
func (m *MockMemoryRepo) List(ctx context.Context, filter model.MemoryFilter) ([]model.Memory, int, error) {
	return m.ListFn(ctx, filter)
}
func (m *MockMemoryRepo) Update(ctx context.Context, id uuid.UUID, req model.UpdateMemoryRequest) (*model.Memory, error) {
	return m.UpdateFn(ctx, id, req)
}
func (m *MockMemoryRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.DeleteFn(ctx, id)
}

// MockTagRepo is a mock implementation of repository.TagRepository.
type MockTagRepo struct {
	CreateFn           func(ctx context.Context, req model.CreateTagRequest) (*model.Tag, error)
	GetByIDFn          func(ctx context.Context, id uuid.UUID) (*model.Tag, error)
	ListFn             func(ctx context.Context) ([]model.Tag, error)
	AddToIssueFn       func(ctx context.Context, issueID, tagID uuid.UUID) error
	RemoveFromIssueFn  func(ctx context.Context, issueID, tagID uuid.UUID) error
	GetByIssueIDFn     func(ctx context.Context, issueID uuid.UUID) ([]model.Tag, error)
	AddToMemoryFn      func(ctx context.Context, memoryID, tagID uuid.UUID) error
	RemoveFromMemoryFn func(ctx context.Context, memoryID, tagID uuid.UUID) error
	GetByMemoryIDFn    func(ctx context.Context, memoryID uuid.UUID) ([]model.Tag, error)
}

func (m *MockTagRepo) Create(ctx context.Context, req model.CreateTagRequest) (*model.Tag, error) {
	return m.CreateFn(ctx, req)
}
func (m *MockTagRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Tag, error) {
	return m.GetByIDFn(ctx, id)
}
func (m *MockTagRepo) List(ctx context.Context) ([]model.Tag, error) {
	return m.ListFn(ctx)
}
func (m *MockTagRepo) AddToIssue(ctx context.Context, issueID, tagID uuid.UUID) error {
	return m.AddToIssueFn(ctx, issueID, tagID)
}
func (m *MockTagRepo) RemoveFromIssue(ctx context.Context, issueID, tagID uuid.UUID) error {
	return m.RemoveFromIssueFn(ctx, issueID, tagID)
}
func (m *MockTagRepo) GetByIssueID(ctx context.Context, issueID uuid.UUID) ([]model.Tag, error) {
	return m.GetByIssueIDFn(ctx, issueID)
}
func (m *MockTagRepo) AddToMemory(ctx context.Context, memoryID, tagID uuid.UUID) error {
	return m.AddToMemoryFn(ctx, memoryID, tagID)
}
func (m *MockTagRepo) RemoveFromMemory(ctx context.Context, memoryID, tagID uuid.UUID) error {
	return m.RemoveFromMemoryFn(ctx, memoryID, tagID)
}
func (m *MockTagRepo) GetByMemoryID(ctx context.Context, memoryID uuid.UUID) ([]model.Tag, error) {
	return m.GetByMemoryIDFn(ctx, memoryID)
}

// MockUserRepo is a mock implementation of repository.UserRepository.
type MockUserRepo struct {
	CreateFn        func(ctx context.Context, username, passwordHash string, displayName *string, role string) (*model.User, error)
	GetByIDFn       func(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByUsernameFn func(ctx context.Context, username string) (*model.User, error)
}

func (m *MockUserRepo) Create(ctx context.Context, username, passwordHash string, displayName *string, role string) (*model.User, error) {
	return m.CreateFn(ctx, username, passwordHash, displayName, role)
}
func (m *MockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return m.GetByIDFn(ctx, id)
}
func (m *MockUserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	return m.GetByUsernameFn(ctx, username)
}

// MockTx is a minimal mock for pgx.Tx used in tests.
type MockTx struct {
	pgx.Tx
	CommitFn   func(ctx context.Context) error
	RollbackFn func(ctx context.Context) error
}

func (m *MockTx) Commit(ctx context.Context) error {
	if m.CommitFn != nil {
		return m.CommitFn(ctx)
	}
	return nil
}

func (m *MockTx) Rollback(ctx context.Context) error {
	if m.RollbackFn != nil {
		return m.RollbackFn(ctx)
	}
	return nil
}
