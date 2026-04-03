package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
)

var validDepTypes = map[string]bool{"depends_on": true, "blocks": true}
var validSeverities = map[string]bool{"critical": true, "recommended": true}

// priorityRank maps priority strings to numeric values (lower = higher priority).
var priorityRank = map[string]int{"P0": 0, "P1": 1, "P2": 2}

type DependencyService struct {
	depRepo     repository.DependencyRepository
	issueRepo   repository.IssueRepository
	projectRepo repository.ProjectRepository
}

func NewDependencyService(depRepo repository.DependencyRepository, issueRepo repository.IssueRepository, projectRepo repository.ProjectRepository) *DependencyService {
	return &DependencyService{
		depRepo:     depRepo,
		issueRepo:   issueRepo,
		projectRepo: projectRepo,
	}
}

func (s *DependencyService) Create(ctx context.Context, sourceIssueID uuid.UUID, req model.CreateDependencyRequest) (*model.IssueDependency, error) {
	if !validDepTypes[req.Type] {
		return nil, fmt.Errorf("invalid dependency type: must be 'depends_on' or 'blocks'")
	}
	if !validSeverities[req.Severity] {
		return nil, fmt.Errorf("invalid severity: must be 'critical' or 'recommended'")
	}

	if sourceIssueID == req.TargetIssueID {
		return nil, fmt.Errorf("an issue cannot depend on itself")
	}

	// Verify both issues exist
	source, err := s.issueRepo.GetByID(ctx, sourceIssueID)
	if err != nil {
		return nil, fmt.Errorf("get source issue: %w", err)
	}
	if source == nil {
		return nil, fmt.Errorf("source issue not found")
	}

	target, err := s.issueRepo.GetByID(ctx, req.TargetIssueID)
	if err != nil {
		return nil, fmt.Errorf("get target issue: %w", err)
	}
	if target == nil {
		return nil, fmt.Errorf("target issue not found")
	}

	// Cycle detection: for depends_on, check if target already has a path to source.
	// For blocks, the inverse: source blocks target means target depends_on source logically,
	// so check if source already has a path to target via depends_on.
	if req.Type == "depends_on" {
		// Creating: source depends_on target
		// Cycle if: target already has a path to source (target -> ... -> source)
		hasPath, err := s.depRepo.HasPath(ctx, req.TargetIssueID, sourceIssueID)
		if err != nil {
			return nil, fmt.Errorf("cycle detection: %w", err)
		}
		if hasPath {
			return nil, fmt.Errorf("cannot create dependency: would create a cycle")
		}
	} else {
		// Creating: source blocks target, which means target depends_on source
		// Cycle if: source already depends on target (source -> ... -> target)
		hasPath, err := s.depRepo.HasPath(ctx, sourceIssueID, req.TargetIssueID)
		if err != nil {
			return nil, fmt.Errorf("cycle detection: %w", err)
		}
		if hasPath {
			return nil, fmt.Errorf("cannot create dependency: would create a cycle")
		}
	}

	dep := model.IssueDependency{
		SourceIssueID: sourceIssueID,
		TargetIssueID: req.TargetIssueID,
		Type:          req.Type,
		Severity:      req.Severity,
	}

	return s.depRepo.Create(ctx, dep)
}

func (s *DependencyService) Delete(ctx context.Context, depID uuid.UUID) error {
	return s.depRepo.Delete(ctx, depID)
}

func (s *DependencyService) ListByIssueID(ctx context.Context, issueID uuid.UUID) ([]model.IssueDependency, error) {
	return s.depRepo.ListByIssueID(ctx, issueID)
}

// GetDependencyTree builds a tree rooted at the given issue.
// "depends_on" children are issues that the root depends on (downstream).
// "blocks" children are issues that depend on the root (upstream).
func (s *DependencyService) GetDependencyTree(ctx context.Context, issueID uuid.UUID) (*model.DependencyNode, error) {
	issue, err := s.issueRepo.GetByID(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("get root issue: %w", err)
	}
	if issue == nil {
		return nil, fmt.Errorf("issue not found")
	}

	project, err := s.projectRepo.GetByID(ctx, issue.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}

	root := &model.DependencyNode{
		IssueID:     issue.ID,
		IssueKey:    issue.IssueKey,
		Title:       issue.Title,
		Status:      issue.Status,
		Priority:    issue.Priority,
		ProjectID:   issue.ProjectID,
		ProjectKey:  project.Key,
		ProjectName: project.Name,
		Severity:    "",
	}

	visited := map[uuid.UUID]bool{issueID: true}

	// Build depends_on subtree (issues this one depends on)
	if err := s.buildDependsOnTree(ctx, root, visited); err != nil {
		return nil, err
	}

	// Reset visited for blocks direction
	visited = map[uuid.UUID]bool{issueID: true}

	// Build blocks subtree (issues that depend on this one)
	if err := s.buildBlocksTree(ctx, root, visited); err != nil {
		return nil, err
	}

	return root, nil
}

func (s *DependencyService) buildDependsOnTree(ctx context.Context, node *model.DependencyNode, visited map[uuid.UUID]bool) error {
	deps, err := s.depRepo.GetDependsOn(ctx, node.IssueID)
	if err != nil {
		return err
	}

	for _, dep := range deps {
		if visited[dep.TargetIssueID] {
			continue
		}
		visited[dep.TargetIssueID] = true

		issue, err := s.issueRepo.GetByID(ctx, dep.TargetIssueID)
		if err != nil || issue == nil {
			continue
		}

		project, err := s.projectRepo.GetByID(ctx, issue.ProjectID)
		if err != nil {
			continue
		}

		child := &model.DependencyNode{
			IssueID:     issue.ID,
			IssueKey:    issue.IssueKey,
			Title:       issue.Title,
			Status:      issue.Status,
			Priority:    issue.Priority,
			ProjectID:   issue.ProjectID,
			ProjectKey:  project.Key,
			ProjectName: project.Name,
			Severity:    dep.Severity,
		}

		if err := s.buildDependsOnTree(ctx, child, visited); err != nil {
			return err
		}

		node.DependsOn = append(node.DependsOn, child)
	}

	return nil
}

func (s *DependencyService) buildBlocksTree(ctx context.Context, node *model.DependencyNode, visited map[uuid.UUID]bool) error {
	// Find issues where target_issue_id = node.IssueID and type = 'depends_on'
	// i.e., issues that depend on this node (this node blocks them)
	deps, err := s.depRepo.GetBlocks(ctx, node.IssueID)
	if err != nil {
		return err
	}

	// Also find reverse depends_on: issues where this issue is the target
	// These are issues that depend_on this one, meaning this one blocks them
	reverseDeps, err := s.getReverseDependsOn(ctx, node.IssueID)
	if err != nil {
		return err
	}

	// Process explicit blocks relationships
	for _, dep := range deps {
		if visited[dep.TargetIssueID] {
			continue
		}
		visited[dep.TargetIssueID] = true

		issue, err := s.issueRepo.GetByID(ctx, dep.TargetIssueID)
		if err != nil || issue == nil {
			continue
		}

		project, err := s.projectRepo.GetByID(ctx, issue.ProjectID)
		if err != nil {
			continue
		}

		child := &model.DependencyNode{
			IssueID:     issue.ID,
			IssueKey:    issue.IssueKey,
			Title:       issue.Title,
			Status:      issue.Status,
			Priority:    issue.Priority,
			ProjectID:   issue.ProjectID,
			ProjectKey:  project.Key,
			ProjectName: project.Name,
			Severity:    dep.Severity,
		}

		node.Blocks = append(node.Blocks, child)
	}

	// Process reverse depends_on (issues that depend on this node)
	for _, dep := range reverseDeps {
		if visited[dep.SourceIssueID] {
			continue
		}
		visited[dep.SourceIssueID] = true

		issue, err := s.issueRepo.GetByID(ctx, dep.SourceIssueID)
		if err != nil || issue == nil {
			continue
		}

		project, err := s.projectRepo.GetByID(ctx, issue.ProjectID)
		if err != nil {
			continue
		}

		child := &model.DependencyNode{
			IssueID:     issue.ID,
			IssueKey:    issue.IssueKey,
			Title:       issue.Title,
			Status:      issue.Status,
			Priority:    issue.Priority,
			ProjectID:   issue.ProjectID,
			ProjectKey:  project.Key,
			ProjectName: project.Name,
			Severity:    dep.Severity,
		}

		node.Blocks = append(node.Blocks, child)
	}

	return nil
}

// getReverseDependsOn finds issues where the given issueID is the target of a depends_on relationship.
// This requires a direct DB query since the DependencyRepository interface doesn't have this.
func (s *DependencyService) getReverseDependsOn(ctx context.Context, issueID uuid.UUID) ([]model.IssueDependency, error) {
	// We use ListByIssueID and filter for reverse depends_on
	all, err := s.depRepo.ListByIssueID(ctx, issueID)
	if err != nil {
		return nil, err
	}

	var result []model.IssueDependency
	for _, dep := range all {
		if dep.TargetIssueID == issueID && dep.Type == "depends_on" {
			result = append(result, dep)
		}
	}
	return result, nil
}

// GetEffectivePriority computes the effective priority of an issue considering
// critical dependencies. The effective priority is the highest (lowest P-number)
// among the issue's own priority and the priorities of all issues it critically
// depends on (transitively).
func (s *DependencyService) GetEffectivePriority(ctx context.Context, issueID uuid.UUID) (string, error) {
	issue, err := s.issueRepo.GetByID(ctx, issueID)
	if err != nil {
		return "", fmt.Errorf("get issue: %w", err)
	}
	if issue == nil {
		return "", fmt.Errorf("issue not found")
	}

	bestRank := priorityRank[issue.Priority]
	visited := map[uuid.UUID]bool{issueID: true}

	if err := s.walkCriticalDeps(ctx, issueID, &bestRank, visited); err != nil {
		return "", err
	}

	// Convert rank back to priority string
	for p, r := range priorityRank {
		if r == bestRank {
			return p, nil
		}
	}
	return issue.Priority, nil
}

func (s *DependencyService) walkCriticalDeps(ctx context.Context, issueID uuid.UUID, bestRank *int, visited map[uuid.UUID]bool) error {
	deps, err := s.depRepo.GetDependsOn(ctx, issueID)
	if err != nil {
		return err
	}

	for _, dep := range deps {
		if dep.Severity != "critical" {
			continue
		}
		if visited[dep.TargetIssueID] {
			continue
		}
		visited[dep.TargetIssueID] = true

		target, err := s.issueRepo.GetByID(ctx, dep.TargetIssueID)
		if err != nil || target == nil {
			continue
		}

		if rank, ok := priorityRank[target.Priority]; ok && rank < *bestRank {
			*bestRank = rank
		}

		if err := s.walkCriticalDeps(ctx, dep.TargetIssueID, bestRank, visited); err != nil {
			return err
		}
	}

	return nil
}
