package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// IssueTypeEpic is the issue type that groups other issues (MF-39). An epic
// is an ordinary issue — key, description, status, assets, comments and
// memories all work as they do elsewhere — that other issues point at through
// epic_id. Epics do not nest.
const IssueTypeEpic = "epic"

// Memory scopes on an issue's memory list: recorded against the issue itself,
// or against the epic it belongs to.
const (
	MemoryScopeIssue = "issue"
	MemoryScopeEpic  = "epic"
)

// EpicCounts is how far along an epic's issues are. Rejected and duplicate
// issues are left out of Total: they are not work the epic still owes.
type EpicCounts struct {
	Total        int            `json:"total"`
	Done         int            `json:"done"`
	DoneRatio    float64        `json:"done_ratio"`
	StatusCounts map[string]int `json:"status_counts"`
}

// EpicSummary is one row of a project's epic list.
type EpicSummary struct {
	Issue
	Counts EpicCounts `json:"counts"`
}

// BlockedIssue is an epic issue that cannot start because a critical
// dependency is not finished yet.
type BlockedIssue struct {
	Issue     IssueRef   `json:"issue"`
	BlockedBy []IssueRef `json:"blocked_by"`
}

// EpicChild is an issue of an epic, with its project key for cross-project
// epics.
type EpicChild struct {
	Issue
	ProjectKey string `json:"project_key"`
}

// EpicDependency is a dependency between two issues of the same epic.
type EpicDependency struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Type     string `json:"type"`
	Severity string `json:"severity"`
}

// EpicDetail is everything the epic view shows.
type EpicDetail struct {
	Epic         *Issue           `json:"epic"`
	Children     []EpicChild      `json:"children"`
	Counts       EpicCounts       `json:"counts"`
	Dependencies []EpicDependency `json:"dependencies"`
	Blocked      []BlockedIssue   `json:"blocked"`
}

// EpicProgress is the epic counterpart of the project progress endpoints.
type EpicProgress struct {
	Counts         EpicCounts     `json:"counts"`
	PriorityCounts map[string]int `json:"priority_counts"`
	TypeCounts     map[string]int `json:"type_counts"`
	Trend          []TrendPoint   `json:"trend"`
	Blocked        []BlockedIssue `json:"blocked"`
}

// SetEpicRequest puts an issue into an epic, moves it to another, or (with an
// empty Epic) takes it out.
type SetEpicRequest struct {
	Epic string `json:"epic"`
}

// SetEpicResult reports the move, so a caller can say "moved from X".
type SetEpicResult struct {
	Issue    *Issue    `json:"issue"`
	Previous *IssueRef `json:"previous,omitempty"`
}

// IssueMemory is a memory as seen from one issue: its own, or one inherited
// from the issue's epic. The read state is per reader and only reported for
// epic memories — those are the ones other people change under you.
type IssueMemory struct {
	MemoryResponse
	Scope   string `json:"scope"`
	EpicKey string `json:"epic_key,omitempty"`
	// Revision fingerprints title and content; send it back to mark this
	// version read.
	Revision string `json:"revision"`
	// Unread: the reader has never read this memory. Updated: they read an
	// older version of it.
	Unread  bool `json:"unread"`
	Updated bool `json:"updated"`
}

// MemoryReadMark marks one version of a memory read.
type MemoryReadMark struct {
	MemoryID uuid.UUID `json:"memory_id"`
	Revision string    `json:"revision"`
}

// MarkMemoriesReadRequest records what a reader has read.
type MarkMemoriesReadRequest struct {
	Reader   string           `json:"reader"`
	Memories []MemoryReadMark `json:"memories"`
}

// MemoryRead is the stored read receipt (synced between instances).
type MemoryRead struct {
	MemoryID    uuid.UUID `json:"memory_id"`
	Reader      string    `json:"reader"`
	ContentHash string    `json:"content_hash"`
	ReadAt      time.Time `json:"read_at"`
}

// PromoteMemoryRequest lifts an issue memory to its epic. Copy keeps the
// original on the issue; Force skips the conflict check.
type PromoteMemoryRequest struct {
	Copy  bool `json:"copy"`
	Force bool `json:"force"`
}

// MemoryConflict is an existing memory that a new one seems to restate.
type MemoryConflict struct {
	MemoryResponse
	Similarity float64 `json:"similarity"`
}

// Issue's MarshalJSON would be promoted through embedding and drop the
// wrapper's own fields, so the wrappers splice them into the issue object.
func (s EpicSummary) MarshalJSON() ([]byte, error) {
	return withFields(s.Issue, map[string]any{"counts": s.Counts})
}

func (c EpicChild) MarshalJSON() ([]byte, error) {
	return withFields(c.Issue, map[string]any{"project_key": c.ProjectKey})
}

func withFields(issue Issue, extra map[string]any) ([]byte, error) {
	b, err := json.Marshal(issue)
	if err != nil {
		return nil, err
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil, err
	}
	for k, v := range extra {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		obj[k] = raw
	}
	return json.Marshal(obj)
}
