package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
)

var validIssueTypes = map[string]bool{"requirement": true, "bug": true}
var validPriorities = map[string]bool{"P0": true, "P1": true, "P2": true}

type IssueService struct {
	issueRepo   repository.IssueRepository
	projectRepo repository.ProjectRepository
	historyRepo repository.IssueHistoryRepository
}

func NewIssueService(issueRepo repository.IssueRepository, projectRepo repository.ProjectRepository, historyRepo repository.IssueHistoryRepository) *IssueService {
	return &IssueService{
		issueRepo:   issueRepo,
		projectRepo: projectRepo,
		historyRepo: historyRepo,
	}
}

func (s *IssueService) Create(ctx context.Context, projectID uuid.UUID, req model.CreateIssueRequest) (*model.Issue, error) {
	issue, _, err := s.CreateWithSimilar(ctx, projectID, req)
	return issue, err
}

// dedupClosedLookback is how far back done/closed issues still count as
// duplicate candidates: a bug filed again shortly after its fix is worth
// flagging, one from last year is not.
const dedupClosedLookback = 30 * 24 * time.Hour

// FindSimilar returns the existing issues of a project that look like the
// draft in req, best first (MF-35).
func (s *IssueService) FindSimilar(ctx context.Context, projectID uuid.UUID, req model.SimilarIssuesRequest) ([]model.SimilarIssue, error) {
	desc := ""
	if req.Description != nil {
		desc = *req.Description
	}
	if strings.TrimSpace(req.Title) == "" && strings.TrimSpace(desc) == "" {
		return nil, fmt.Errorf("title or description is required")
	}
	candidates, err := s.issueRepo.ListDedupCandidates(ctx, projectID, time.Now().UTC().Add(-dedupClosedLookback))
	if err != nil {
		return nil, err
	}
	draft := &model.Issue{Type: req.Type, Title: req.Title, Description: req.Description}
	similar := rankSimilar(draft, candidates, req.ExcludeKey)
	if similar == nil {
		similar = []model.SimilarIssue{}
	}
	return similar, nil
}

// CreateWithSimilar creates the issue and also returns the suspected
// duplicates found just before it was inserted. Creation never waits on the
// filer's judgement: callers without a TTY must still be able to file. A
// failed lookup only loses the hint, never the issue.
//
// With req.NotDuplicate the filer has already looked at the candidates; the
// keys they waved through are recorded in the history as not_duplicate_of.
func (s *IssueService) CreateWithSimilar(ctx context.Context, projectID uuid.UUID, req model.CreateIssueRequest) (*model.Issue, []model.SimilarIssue, error) {
	if req.Type == "" {
		return nil, nil, fmt.Errorf("issue type is required")
	}
	if !validIssueTypes[req.Type] {
		return nil, nil, fmt.Errorf("invalid issue type: must be 'requirement' or 'bug'")
	}
	if req.Title == "" {
		return nil, nil, fmt.Errorf("issue title is required")
	}
	if req.Priority != nil && !validPriorities[*req.Priority] {
		return nil, nil, fmt.Errorf("invalid priority: must be 'P0', 'P1', or 'P2'")
	}

	similar, err := s.FindSimilar(ctx, projectID, model.SimilarIssuesRequest{
		Type: req.Type, Title: req.Title, Description: req.Description,
	})
	if err != nil {
		log.Printf("duplicate lookup for new issue in %s failed: %v", projectID, err)
		similar = nil
	}

	tx, err := s.issueRepo.BeginTx(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	num, projectKey, err := s.projectRepo.IncrementIssueNumber(ctx, tx, projectID)
	if err != nil {
		return nil, nil, fmt.Errorf("increment issue number: %w", err)
	}

	issueKey := fmt.Sprintf("%s-%d", projectKey, num)

	issue, err := s.issueRepo.Create(ctx, tx, issueKey, projectID, req)
	if err != nil {
		return nil, nil, fmt.Errorf("create issue: %w", err)
	}

	if req.NotDuplicate && len(similar) > 0 && s.historyRepo != nil {
		keys := make([]string, len(similar))
		for i, si := range similar {
			keys[i] = si.Key
		}
		judged := strings.Join(keys, ",")
		if err := s.historyRepo.Create(ctx, tx, issue.ID, "not_duplicate_of", nil, &judged, req.CreatorID); err != nil {
			return nil, nil, fmt.Errorf("record not-duplicate judgement: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit transaction: %w", err)
	}

	return issue, similar, nil
}

func (s *IssueService) GetByID(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
	return s.issueRepo.GetByID(ctx, id)
}

func (s *IssueService) GetByKey(ctx context.Context, key string) (*model.Issue, error) {
	return s.issueRepo.GetByKey(ctx, key)
}

func (s *IssueService) List(ctx context.Context, filter model.IssueFilter) ([]model.Issue, int, error) {
	return s.issueRepo.List(ctx, filter)
}

func (s *IssueService) Update(ctx context.Context, id uuid.UUID, req model.UpdateIssueRequest, operatorID *string) (*model.Issue, error) {
	if req.Type != nil && !validIssueTypes[*req.Type] {
		return nil, fmt.Errorf("invalid issue type: must be 'requirement' or 'bug'")
	}
	if req.Priority != nil && !validPriorities[*req.Priority] {
		return nil, fmt.Errorf("invalid priority: must be 'P0', 'P1', or 'P2'")
	}

	existing, err := s.issueRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get existing issue: %w", err)
	}
	if existing == nil {
		return nil, fmt.Errorf("issue not found")
	}

	tx, err := s.issueRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var setClauses []string
	var args []interface{}
	argIdx := 1

	if req.Type != nil && *req.Type != existing.Type {
		old := existing.Type
		setClauses = append(setClauses, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, *req.Type)
		argIdx++
		if err := s.historyRepo.Create(ctx, tx, id, "type", &old, req.Type, operatorID); err != nil {
			return nil, err
		}
	}
	if req.Title != nil && *req.Title != existing.Title {
		old := existing.Title
		setClauses = append(setClauses, fmt.Sprintf("title = $%d", argIdx))
		args = append(args, *req.Title)
		argIdx++
		if err := s.historyRepo.Create(ctx, tx, id, "title", &old, req.Title, operatorID); err != nil {
			return nil, err
		}
	}
	if req.Description != nil {
		oldVal := ""
		if existing.Description != nil {
			oldVal = *existing.Description
		}
		if *req.Description != oldVal {
			setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIdx))
			args = append(args, *req.Description)
			argIdx++
			if err := s.historyRepo.Create(ctx, tx, id, "description", existing.Description, req.Description, operatorID); err != nil {
				return nil, err
			}
		}
	}
	if req.Priority != nil && *req.Priority != existing.Priority {
		old := existing.Priority
		setClauses = append(setClauses, fmt.Sprintf("priority = $%d", argIdx))
		args = append(args, *req.Priority)
		argIdx++
		if err := s.historyRepo.Create(ctx, tx, id, "priority", &old, req.Priority, operatorID); err != nil {
			return nil, err
		}
	}
	if req.AssigneeID != nil {
		oldVal := ""
		if existing.AssigneeID != nil {
			oldVal = *existing.AssigneeID
		}
		if *req.AssigneeID != oldVal {
			setClauses = append(setClauses, fmt.Sprintf("assignee_id = $%d", argIdx))
			args = append(args, *req.AssigneeID)
			argIdx++
			if err := s.historyRepo.Create(ctx, tx, id, "assignee_id", existing.AssigneeID, req.AssigneeID, operatorID); err != nil {
				return nil, err
			}
		}
	}
	if req.Source != nil {
		oldVal := ""
		if existing.Source != nil {
			oldVal = *existing.Source
		}
		if *req.Source != oldVal {
			setClauses = append(setClauses, fmt.Sprintf("source = $%d", argIdx))
			args = append(args, *req.Source)
			argIdx++
			if err := s.historyRepo.Create(ctx, tx, id, "source", existing.Source, req.Source, operatorID); err != nil {
				return nil, err
			}
		}
	}
	if req.Version != nil {
		oldVal := ""
		if existing.Version != nil {
			oldVal = *existing.Version
		}
		if *req.Version != oldVal {
			setClauses = append(setClauses, fmt.Sprintf("version = $%d", argIdx))
			args = append(args, *req.Version)
			argIdx++
			if err := s.historyRepo.Create(ctx, tx, id, "version", existing.Version, req.Version, operatorID); err != nil {
				return nil, err
			}
		}
	}
	if req.GitURL != nil {
		oldVal := ""
		if existing.GitURL != nil {
			oldVal = *existing.GitURL
		}
		if *req.GitURL != oldVal {
			setClauses = append(setClauses, fmt.Sprintf("git_url = $%d", argIdx))
			args = append(args, *req.GitURL)
			argIdx++
			if err := s.historyRepo.Create(ctx, tx, id, "git_url", existing.GitURL, req.GitURL, operatorID); err != nil {
				return nil, err
			}
		}
	}
	if req.PRURL != nil {
		oldVal := ""
		if existing.PRURL != nil {
			oldVal = *existing.PRURL
		}
		if *req.PRURL != oldVal {
			setClauses = append(setClauses, fmt.Sprintf("pr_url = $%d", argIdx))
			args = append(args, *req.PRURL)
			argIdx++
			if err := s.historyRepo.Create(ctx, tx, id, "pr_url", existing.PRURL, req.PRURL, operatorID); err != nil {
				return nil, err
			}
		}
	}
	if req.DocURL != nil {
		oldVal := ""
		if existing.DocURL != nil {
			oldVal = *existing.DocURL
		}
		if *req.DocURL != oldVal {
			setClauses = append(setClauses, fmt.Sprintf("doc_url = $%d", argIdx))
			args = append(args, *req.DocURL)
			argIdx++
			if err := s.historyRepo.Create(ctx, tx, id, "doc_url", existing.DocURL, req.DocURL, operatorID); err != nil {
				return nil, err
			}
		}
	}

	if len(setClauses) == 0 {
		return existing, nil
	}

	issue, err := s.issueRepo.Update(ctx, tx, id, setClauses, args)
	if err != nil {
		return nil, fmt.Errorf("update issue: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return issue, nil
}

func (s *IssueService) TransitionStatus(ctx context.Context, id uuid.UUID, newStatus string, operatorID *string) (*model.Issue, error) {
	existing, err := s.issueRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get existing issue: %w", err)
	}
	if existing == nil {
		return nil, fmt.Errorf("issue not found")
	}

	if _, ok := model.AllowedTransitions(existing.Status); !ok {
		return nil, fmt.Errorf("cannot transition from status: %s", existing.Status)
	}
	if !model.CanTransition(existing.Status, newStatus) {
		return nil, fmt.Errorf("cannot transition from %s to %s", existing.Status, newStatus)
	}

	tx, err := s.issueRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	oldStatus := existing.Status
	if err := s.historyRepo.Create(ctx, tx, id, "status", &oldStatus, &newStatus, operatorID); err != nil {
		return nil, err
	}

	setClauses := []string{"status = $1"}
	args := []interface{}{newStatus}

	issue, err := s.issueRepo.Update(ctx, tx, id, setClauses, args)
	if err != nil {
		return nil, fmt.Errorf("update issue status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return issue, nil
}

func (s *IssueService) GetHistory(ctx context.Context, issueID uuid.UUID) ([]model.IssueHistory, error) {
	return s.historyRepo.ListByIssueID(ctx, issueID)
}
