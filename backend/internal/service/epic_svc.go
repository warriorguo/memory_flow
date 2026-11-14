package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
)

// EpicService implements epics (MF-39): membership, the epic view and its
// progress, and memories shared from an epic to its issues.
type EpicService struct {
	issueRepo   repository.IssueRepository
	historyRepo repository.IssueHistoryRepository
	memoryRepo  repository.MemoryRepository
	epicRepo    *repository.EpicRepo
}

func NewEpicService(issueRepo repository.IssueRepository, historyRepo repository.IssueHistoryRepository, memoryRepo repository.MemoryRepository, epicRepo *repository.EpicRepo) *EpicService {
	return &EpicService{issueRepo: issueRepo, historyRepo: historyRepo, memoryRepo: memoryRepo, epicRepo: epicRepo}
}

// finishedStatuses no longer hold anything up: a dependency on one is met.
var finishedStatuses = map[string]bool{"done": true, "closed": true}

// notOwedStatuses are issues an epic does not count as remaining work.
var notOwedStatuses = map[string]bool{"rejected": true, model.StatusDuplicate: true}

// GetEpic returns the issue id names, requiring it to be an epic.
func (s *EpicService) GetEpic(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
	epic, err := s.issueRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if epic == nil {
		return nil, fmt.Errorf("epic not found")
	}
	if epic.Type != model.IssueTypeEpic {
		return nil, fmt.Errorf("%s is a %s, not an epic (make it one with: mf issue update %s --type epic)", epic.IssueKey, epic.Type, epic.IssueKey)
	}
	return epic, nil
}

// CheckMembership says whether an issue of type issueType may join epicID.
func (s *EpicService) CheckMembership(ctx context.Context, issueType string, epicID uuid.UUID) (*model.Issue, error) {
	if issueType == model.IssueTypeEpic {
		return nil, fmt.Errorf("epics do not nest: an epic cannot belong to another epic")
	}
	return s.GetEpic(ctx, epicID)
}

// CheckTypeChange guards changing an issue's type to or from epic: an epic
// with issues would orphan them, and an issue inside an epic cannot become one.
func (s *EpicService) CheckTypeChange(ctx context.Context, issue *model.Issue, newType string) error {
	if newType == issue.Type {
		return nil
	}
	if issue.Type == model.IssueTypeEpic {
		n, err := s.epicRepo.CountChildren(ctx, issue.ID)
		if err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("%s still has %d issue(s); move them out first (mf epic rm …)", issue.IssueKey, n)
		}
	}
	if newType == model.IssueTypeEpic && issue.EpicID != nil {
		return fmt.Errorf("%s belongs to an epic; take it out first (mf epic rm %s)", issue.IssueKey, issue.IssueKey)
	}
	return nil
}

// SetEpic puts issue id into epicID, moves it there from another epic, or —
// with epicID nil — takes it out. The change lands in the issue's history.
func (s *EpicService) SetEpic(ctx context.Context, id uuid.UUID, epicID *uuid.UUID, operatorID *string) (*model.SetEpicResult, error) {
	issue, err := s.issueRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if issue == nil {
		return nil, fmt.Errorf("issue not found")
	}
	var epic *model.Issue
	if epicID != nil {
		if *epicID == issue.ID {
			return nil, fmt.Errorf("an epic cannot contain itself")
		}
		if epic, err = s.CheckMembership(ctx, issue.Type, *epicID); err != nil {
			return nil, err
		}
	}
	res := &model.SetEpicResult{Issue: issue}
	var oldKey *string
	if issue.EpicID != nil {
		if prev, err := s.issueRepo.GetByID(ctx, *issue.EpicID); err == nil && prev != nil {
			res.Previous = issueRef(prev)
			oldKey = &prev.IssueKey
		}
	}
	if (epicID == nil && issue.EpicID == nil) || (epicID != nil && issue.EpicID != nil && *epicID == *issue.EpicID) {
		return res, nil
	}
	var newKey *string
	if epic != nil {
		newKey = &epic.IssueKey
	}

	tx, err := s.issueRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := s.historyRepo.Create(ctx, tx, id, "epic", oldKey, newKey, operatorID); err != nil {
		return nil, err
	}
	updated, err := s.issueRepo.Update(ctx, tx, id, []string{"epic_id = $1"}, []interface{}{epicID})
	if err != nil {
		return nil, fmt.Errorf("set epic: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}
	res.Issue = updated
	return res, nil
}

// FillEpicRef sets issue.Epic on a single-issue read.
func (s *EpicService) FillEpicRef(ctx context.Context, issue *model.Issue) error {
	if issue.EpicID == nil {
		return nil
	}
	epic, err := s.issueRepo.GetByID(ctx, *issue.EpicID)
	if err != nil {
		return err
	}
	if epic != nil {
		issue.Epic = issueRef(epic)
	}
	return nil
}

// ListEpics returns a project's epics with how far along each one is.
func (s *EpicService) ListEpics(ctx context.Context, projectID uuid.UUID) ([]model.EpicSummary, error) {
	epics, err := s.epicRepo.ListEpics(ctx, projectID)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(epics))
	for i := range epics {
		ids[i] = epics[i].ID
	}
	counts, err := s.epicRepo.StatusCountsByEpic(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]model.EpicSummary, len(epics))
	for i := range epics {
		out[i] = model.EpicSummary{Issue: epics[i], Counts: epicCounts(counts[epics[i].ID])}
	}
	return out, nil
}

// Detail is the epic view: the epic, its issues, how far along they are, the
// dependencies among them and which of them are blocked.
func (s *EpicService) Detail(ctx context.Context, epicID uuid.UUID) (*model.EpicDetail, error) {
	epic, err := s.GetEpic(ctx, epicID)
	if err != nil {
		return nil, err
	}
	children, err := s.epicRepo.ListChildren(ctx, epicID)
	if err != nil {
		return nil, err
	}
	deps, err := s.epicRepo.ListDependencies(ctx, epicID)
	if err != nil {
		return nil, err
	}
	if children == nil {
		children = []model.EpicChild{}
	}
	d := &model.EpicDetail{
		Epic:         epic,
		Children:     children,
		Counts:       epicCounts(statusCounts(children)),
		Dependencies: []model.EpicDependency{},
		Blocked:      blockedIssues(children, deps),
	}
	for _, dep := range deps {
		if dep.SourceIn && dep.TargetIn {
			d.Dependencies = append(d.Dependencies, model.EpicDependency{
				Source: dep.Source.IssueKey, Target: dep.Target.IssueKey, Type: dep.Type, Severity: dep.Severity,
			})
		}
	}
	return d, nil
}

// Progress is the epic counterpart of the project progress summary and trend.
func (s *EpicService) Progress(ctx context.Context, epicID uuid.UUID, days int) (*model.EpicProgress, error) {
	if _, err := s.GetEpic(ctx, epicID); err != nil {
		return nil, err
	}
	if days <= 0 {
		days = 30
	}
	children, err := s.epicRepo.ListChildren(ctx, epicID)
	if err != nil {
		return nil, err
	}
	deps, err := s.epicRepo.ListDependencies(ctx, epicID)
	if err != nil {
		return nil, err
	}
	p := &model.EpicProgress{
		Counts:         epicCounts(statusCounts(children)),
		PriorityCounts: map[string]int{},
		TypeCounts:     map[string]int{},
		Blocked:        blockedIssues(children, deps),
	}
	for _, c := range children {
		p.PriorityCounts[c.Priority]++
		p.TypeCounts[c.Type]++
	}

	const layout = "2006-01-02"
	from := time.Now().UTC().AddDate(0, 0, -days)
	done, err := s.epicRepo.DoneDays(ctx, epicID, from)
	if err != nil {
		return nil, err
	}
	created := map[string]int{}
	for _, c := range children {
		if !c.CreatedAt.Before(from) {
			created[c.CreatedAt.UTC().Format(layout)]++
		}
	}
	end := time.Now().UTC()
	for d := from; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format(layout)
		p.Trend = append(p.Trend, model.TrendPoint{Date: key, Created: created[key], Done: done[key]})
	}
	return p, nil
}

func statusCounts(children []model.EpicChild) map[string]int {
	m := map[string]int{}
	for _, c := range children {
		m[c.Status]++
	}
	return m
}

func epicCounts(byStatus map[string]int) model.EpicCounts {
	c := model.EpicCounts{StatusCounts: map[string]int{}}
	for status, n := range byStatus {
		c.StatusCounts[status] = n
		if notOwedStatuses[status] {
			continue
		}
		c.Total += n
		if finishedStatuses[status] {
			c.Done += n
		}
	}
	if c.Total > 0 {
		c.DoneRatio = float64(c.Done) / float64(c.Total)
	}
	return c
}

// blockedIssues lists the open issues of the epic held up by an unfinished
// critical dependency — "depends_on X" or "X blocks it" — wherever X lives.
func blockedIssues(children []model.EpicChild, deps []repository.EpicDependencyRow) []model.BlockedIssue {
	byID := map[uuid.UUID]*model.BlockedIssue{}
	var order []uuid.UUID
	open := map[uuid.UUID]model.IssueRef{}
	for _, c := range children {
		if !finishedStatuses[c.Status] && !notOwedStatuses[c.Status] {
			open[c.ID] = *issueRef(&c.Issue)
		}
	}
	add := func(blocked uuid.UUID, by model.IssueRef) {
		ref, ok := open[blocked]
		if !ok || finishedStatuses[by.Status] {
			return
		}
		b := byID[blocked]
		if b == nil {
			b = &model.BlockedIssue{Issue: ref}
			byID[blocked] = b
			order = append(order, blocked)
		}
		b.BlockedBy = append(b.BlockedBy, by)
	}
	for _, d := range deps {
		if d.Severity != "critical" {
			continue
		}
		switch d.Type {
		case "depends_on":
			add(d.Source.ID, d.Target)
		case "blocks":
			add(d.Target.ID, d.Source)
		}
	}
	out := make([]model.BlockedIssue, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Issue.IssueKey < out[j].Issue.IssueKey })
	return out
}

func issueRef(i *model.Issue) *model.IssueRef {
	return &model.IssueRef{ID: i.ID, IssueKey: i.IssueKey, Title: i.Title, Status: i.Status, Priority: i.Priority}
}

// --- memories ----------------------------------------------------------------

// MemoryRevision fingerprints the parts of a memory a reader reads.
func MemoryRevision(title, content string) string {
	sum := sha256.Sum256([]byte(title + "\x00" + content))
	return hex.EncodeToString(sum[:8])
}

const memoryPageSize = 200

func (s *EpicService) memoriesOf(ctx context.Context, sourceID uuid.UUID) ([]model.MemoryResponse, error) {
	ms, _, err := s.memoryRepo.List(ctx, model.MemoryFilter{SourceObjectID: &sourceID, Page: 1, PageSize: memoryPageSize})
	return ms, err
}

// IssueMemories returns the memories an issue can see: its own, then those
// of its epic. Epic memories carry reader's read state; when reader is empty
// they are reported unread.
func (s *EpicService) IssueMemories(ctx context.Context, issueID uuid.UUID, reader string) ([]model.IssueMemory, error) {
	issue, err := s.issueRepo.GetByID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if issue == nil {
		return nil, fmt.Errorf("issue not found")
	}
	own, err := s.memoriesOf(ctx, issue.ID)
	if err != nil {
		return nil, err
	}
	out := make([]model.IssueMemory, 0, len(own))
	for _, m := range own {
		out = append(out, model.IssueMemory{MemoryResponse: m, Scope: model.MemoryScopeIssue, Revision: MemoryRevision(m.Title, m.Content)})
	}
	if issue.EpicID == nil {
		return out, nil
	}
	epic, err := s.issueRepo.GetByID(ctx, *issue.EpicID)
	if err != nil {
		return nil, err
	}
	if epic == nil {
		return out, nil
	}
	shared, err := s.memoriesOf(ctx, epic.ID)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(shared))
	for i := range shared {
		ids[i] = shared[i].ID
	}
	reads, err := s.epicRepo.MemoryReads(ctx, reader, ids)
	if err != nil {
		return nil, err
	}
	for _, m := range shared {
		im := model.IssueMemory{MemoryResponse: m, Scope: model.MemoryScopeEpic, EpicKey: epic.IssueKey, Revision: MemoryRevision(m.Title, m.Content)}
		seen, ok := reads[m.ID]
		im.Unread = !ok
		im.Updated = ok && seen != im.Revision
		out = append(out, im)
	}
	return out, nil
}

// MarkMemoriesRead records the versions reader has read. A mark whose
// revision is stale — the memory changed after the reader fetched it — is
// skipped, so the newer version still shows as changed.
func (s *EpicService) MarkMemoriesRead(ctx context.Context, req model.MarkMemoriesReadRequest) (int, error) {
	if req.Reader == "" {
		return 0, fmt.Errorf("reader is required")
	}
	marked := 0
	for _, mk := range req.Memories {
		m, err := s.memoryRepo.GetByID(ctx, mk.MemoryID)
		if err != nil {
			return marked, err
		}
		if m == nil {
			continue
		}
		rev := MemoryRevision(m.Title, m.Content)
		if mk.Revision != "" && mk.Revision != rev {
			continue
		}
		if err := s.epicRepo.MarkMemoryRead(ctx, m.ID, req.Reader, rev); err != nil {
			return marked, err
		}
		marked++
	}
	return marked, nil
}

// Thresholds for "this memory seems to restate that one". Titles compare by
// overlap of character bigrams (a short title fully inside a longer one
// counts), contents by Jaccard similarity.
const (
	memTitleOverlap   = 0.6
	memTitleMinGrams  = 3
	memContentJaccard = 0.5
)

// FindConflicts returns the memories on sourceID that a memory with this
// title and content seems to restate, most similar first.
func (s *EpicService) FindConflicts(ctx context.Context, sourceID uuid.UUID, title, content string, exclude uuid.UUID) ([]model.MemoryConflict, error) {
	existing, err := s.memoriesOf(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	tg, cg := titleBigrams(title), titleBigrams(content)
	var out []model.MemoryConflict
	for _, m := range existing {
		if m.ID == exclude {
			continue
		}
		mt := titleBigrams(m.Title)
		score := 0.0
		if len(tg) >= memTitleMinGrams && len(mt) >= memTitleMinGrams {
			if o := overlap(tg, mt); o >= memTitleOverlap {
				score = o
			}
		}
		if j := jaccard(cg, titleBigrams(m.Content)); j >= memContentJaccard && j > score {
			score = j
		}
		if score > 0 {
			out = append(out, model.MemoryConflict{MemoryResponse: m, Similarity: score})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Similarity > out[j].Similarity })
	if len(out) > 5 {
		out = out[:5]
	}
	return out, nil
}

func overlap(a, b map[string]bool) float64 {
	small, large := a, b
	if len(b) < len(a) {
		small, large = b, a
	}
	if len(small) == 0 {
		return 0
	}
	inter := 0
	for k := range small {
		if large[k] {
			inter++
		}
	}
	return float64(inter) / float64(len(small))
}

// ConflictError is returned when a memory would restate an existing one.
type ConflictError struct {
	Conflicts []model.MemoryConflict
	Target    string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s already has %d memory(ies) that seem to say the same thing; update one of them instead, or pass force to record it anyway", e.Target, len(e.Conflicts))
}

// CheckCreate runs the conflict check for a new memory on an issue.
func (s *EpicService) CheckCreate(ctx context.Context, req model.CreateMemoryRequest) error {
	if !req.CheckConflicts || req.SourceObjectID == nil {
		return nil
	}
	conflicts, err := s.FindConflicts(ctx, *req.SourceObjectID, req.Title, req.Content, uuid.Nil)
	if err != nil {
		return err
	}
	if len(conflicts) == 0 {
		return nil
	}
	target := req.SourceObjectID.String()
	if is, err := s.issueRepo.GetByID(ctx, *req.SourceObjectID); err == nil && is != nil {
		target = is.IssueKey
	}
	return &ConflictError{Conflicts: conflicts, Target: target}
}

// Promote lifts a memory recorded on an issue to that issue's epic, so the
// epic's other issues see it. With copy the original stays on the issue.
func (s *EpicService) Promote(ctx context.Context, memoryID uuid.UUID, req model.PromoteMemoryRequest) (*model.MemoryResponse, *model.Issue, error) {
	m, err := s.memoryRepo.GetByID(ctx, memoryID)
	if err != nil {
		return nil, nil, err
	}
	if m == nil {
		return nil, nil, fmt.Errorf("memory not found")
	}
	if m.SourceObjectID == nil {
		return nil, nil, fmt.Errorf("memory %s is not recorded on an issue", m.ID)
	}
	issue, err := s.issueRepo.GetByID(ctx, *m.SourceObjectID)
	if err != nil {
		return nil, nil, err
	}
	if issue == nil {
		return nil, nil, fmt.Errorf("memory %s is not recorded on an issue", m.ID)
	}
	if issue.Type == model.IssueTypeEpic {
		return nil, nil, fmt.Errorf("memory %s is already on epic %s", m.ID, issue.IssueKey)
	}
	if issue.EpicID == nil {
		return nil, nil, fmt.Errorf("%s does not belong to an epic", issue.IssueKey)
	}
	epic, err := s.GetEpic(ctx, *issue.EpicID)
	if err != nil {
		return nil, nil, err
	}
	if !req.Force {
		conflicts, err := s.FindConflicts(ctx, epic.ID, m.Title, m.Content, m.ID)
		if err != nil {
			return nil, nil, err
		}
		if len(conflicts) > 0 {
			return nil, nil, &ConflictError{Conflicts: conflicts, Target: epic.IssueKey}
		}
	}
	sourceType := model.IssueTypeEpic
	if req.Copy {
		created, err := s.memoryRepo.Create(ctx, model.CreateMemoryRequest{
			ProjectID: &epic.ProjectID, Type: m.Type, Title: m.Title, Content: m.Content,
			SourceObjectType: &sourceType, SourceObjectID: &epic.ID, CreatorID: m.CreatorID,
		})
		return created, epic, err
	}
	if err := s.epicRepo.MoveMemory(ctx, m.ID, epic.ID, sourceType, epic.ProjectID); err != nil {
		return nil, nil, err
	}
	moved, err := s.memoryRepo.GetByID(ctx, m.ID)
	return moved, epic, err
}
