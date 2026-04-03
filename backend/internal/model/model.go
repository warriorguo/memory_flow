package model

import (
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID               uuid.UUID `json:"id"`
	Key              string    `json:"key"`
	Name             string    `json:"name"`
	Summary          *string   `json:"summary"`
	Description      *string   `json:"description"`
	DesignPrinciples *string   `json:"design_principles"`
	GitURL           *string   `json:"git_url"`
	CICDURL          *string   `json:"cicd_url"`
	DocURL           *string   `json:"doc_url"`
	OwnerID          *string   `json:"owner_id"`
	Status           string    `json:"status"`
	NextIssueNumber  int       `json:"next_issue_number"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Issue struct {
	ID          uuid.UUID `json:"id"`
	IssueKey    string    `json:"issue_key"`
	ProjectID   uuid.UUID `json:"project_id"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	Priority    string    `json:"priority"`
	Status      string    `json:"status"`
	AssigneeID  *string   `json:"assignee_id"`
	CreatorID   *string   `json:"creator_id"`
	Source      *string   `json:"source"`
	Version     *string   `json:"version"`
	GitURL      *string   `json:"git_url"`
	PRURL       *string   `json:"pr_url"`
	DocURL      *string   `json:"doc_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Tags        []Tag     `json:"tags,omitempty"`
}

type IssueHistory struct {
	ID         uuid.UUID `json:"id"`
	IssueID    uuid.UUID `json:"issue_id"`
	FieldName  string    `json:"field_name"`
	OldValue   *string   `json:"old_value"`
	NewValue   *string   `json:"new_value"`
	OperatorID *string   `json:"operator_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type Memory struct {
	ID               uuid.UUID  `json:"id"`
	ProjectID        *uuid.UUID `json:"project_id"`
	Type             string     `json:"type"`
	Title            string     `json:"title"`
	Content          string     `json:"content"`
	SourceObjectType *string    `json:"source_object_type"`
	SourceObjectID   *uuid.UUID `json:"source_object_id"`
	CreatorID        *string    `json:"creator_id"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// MemoryProjectRef is the embedded project info returned in Memory responses.
type MemoryProjectRef struct {
	ID   uuid.UUID `json:"id"`
	Key  string    `json:"key"`
	Name string    `json:"name"`
}

// MemoryResponse is the API response struct for a memory, with project info embedded.
type MemoryResponse struct {
	ID               uuid.UUID         `json:"id"`
	ProjectID        *uuid.UUID        `json:"project_id"`
	Project          *MemoryProjectRef `json:"project,omitempty"`
	Type             string            `json:"type"`
	Title            string            `json:"title"`
	Content          string            `json:"content"`
	SourceObjectType *string           `json:"source_object_type"`
	SourceObjectID   *uuid.UUID        `json:"source_object_id"`
	CreatorID        *string           `json:"creator_id"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

type IssueDependency struct {
	ID            uuid.UUID `json:"id"`
	SourceIssueID uuid.UUID `json:"source_issue_id"`
	TargetIssueID uuid.UUID `json:"target_issue_id"`
	Type          string    `json:"type"`     // "depends_on" or "blocks"
	Severity      string    `json:"severity"` // "critical" or "recommended"
	CreatedAt     time.Time `json:"created_at"`
}

// DependencyNode represents a node in the dependency tree.
type DependencyNode struct {
	IssueID     uuid.UUID         `json:"issue_id"`
	IssueKey    string            `json:"issue_key"`
	Title       string            `json:"title"`
	Status      string            `json:"status"`
	Priority    string            `json:"priority"`
	ProjectID   uuid.UUID         `json:"project_id"`
	ProjectKey  string            `json:"project_key"`
	ProjectName string            `json:"project_name"`
	Severity    string            `json:"severity"`
	DependsOn   []*DependencyNode `json:"depends_on,omitempty"`
	Blocks      []*DependencyNode `json:"blocks,omitempty"`
}

// DependencyDetail is a dependency with full issue info for list responses.
type DependencyDetail struct {
	ID            uuid.UUID `json:"id"`
	SourceIssueID uuid.UUID `json:"source_issue_id"`
	TargetIssueID uuid.UUID `json:"target_issue_id"`
	Type          string    `json:"type"`
	Severity      string    `json:"severity"`
	CreatedAt     time.Time `json:"created_at"`
	TargetIssue   *Issue    `json:"target_issue,omitempty"`
	SourceIssue   *Issue    `json:"source_issue,omitempty"`
}

type Tag struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Color     *string   `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID           uuid.UUID `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	DisplayName  *string   `json:"display_name"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// Request/response types

type CreateProjectRequest struct {
	Key              string  `json:"key"`
	Name             string  `json:"name"`
	Summary          *string `json:"summary"`
	Description      *string `json:"description"`
	DesignPrinciples *string `json:"design_principles"`
	GitURL           *string `json:"git_url"`
	CICDURL          *string `json:"cicd_url"`
	DocURL           *string `json:"doc_url"`
	OwnerID          *string `json:"owner_id"`
}

type UpdateProjectRequest struct {
	Name             *string `json:"name"`
	Summary          *string `json:"summary"`
	Description      *string `json:"description"`
	DesignPrinciples *string `json:"design_principles"`
	GitURL           *string `json:"git_url"`
	CICDURL          *string `json:"cicd_url"`
	DocURL           *string `json:"doc_url"`
	OwnerID          *string `json:"owner_id"`
	Status           *string `json:"status"`
}

type CreateIssueRequest struct {
	Type        string  `json:"type"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Priority    *string `json:"priority"`
	AssigneeID  *string `json:"assignee_id"`
	CreatorID   *string `json:"creator_id"`
	Source      *string `json:"source"`
	Version     *string `json:"version"`
	GitURL      *string `json:"git_url"`
	PRURL       *string `json:"pr_url"`
	DocURL      *string `json:"doc_url"`
}

type UpdateIssueRequest struct {
	Type        *string `json:"type"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Priority    *string `json:"priority"`
	AssigneeID  *string `json:"assignee_id"`
	Source      *string `json:"source"`
	Version     *string `json:"version"`
	GitURL      *string `json:"git_url"`
	PRURL       *string `json:"pr_url"`
	DocURL      *string `json:"doc_url"`
}

type TransitionStatusRequest struct {
	Status string `json:"status"`
}

type CreateMemoryRequest struct {
	ProjectID        *uuid.UUID `json:"project_id"`
	Type             string     `json:"type"`
	Title            string     `json:"title"`
	Content          string     `json:"content"`
	SourceObjectType *string    `json:"source_object_type"`
	SourceObjectID   *uuid.UUID `json:"source_object_id"`
	CreatorID        *string    `json:"creator_id"`
}

type UpdateMemoryRequest struct {
	Type    *string `json:"type"`
	Title   *string `json:"title"`
	Content *string `json:"content"`
}

type CreateTagRequest struct {
	Name  string  `json:"name"`
	Color *string `json:"color"`
}

type AddTagRequest struct {
	TagID uuid.UUID `json:"tag_id"`
}

type CreateDependencyRequest struct {
	TargetIssueID uuid.UUID `json:"target_issue_id"`
	Type          string    `json:"type"`     // "depends_on" or "blocks"
	Severity      string    `json:"severity"` // "critical" or "recommended"
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type TrendPoint struct {
	Date    string `json:"date"`
	Created int    `json:"created"`
	Done    int    `json:"done"`
}

type ProgressSummary struct {
	StatusCounts   map[string]int `json:"status_counts"`
	PriorityCounts map[string]int `json:"priority_counts"`
	TypeCounts     map[string]int `json:"type_counts"`
	Total          int            `json:"total"`
}

// Filter types

type ProjectFilter struct {
	Name    *string
	Status  *string
	OwnerID *string
	Page    int
	PageSize int
}

type IssueFilter struct {
	ProjectID  *uuid.UUID
	Type       *string
	Status     *string
	Priority   *string
	AssigneeID *string
	CreatorID  *string
	Keyword    *string `json:"keyword"`
	TagID      *string `json:"tag"`
	Page       int
	PageSize   int
}

type MemoryFilter struct {
	ProjectID        *uuid.UUID
	Type             *string
	Keyword          *string
	SourceObjectType *string
	SourceObjectID   *uuid.UUID
	Page             int
	PageSize         int
}
