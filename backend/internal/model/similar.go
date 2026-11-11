package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

// Similarity levels reported for a suspected duplicate.
const (
	SimilarLikely   = "likely"   // shares an instance id (session, room…) plus more identifiers
	SimilarPossible = "possible" // shares some identifiers, cites the issue, or has a close title
)

// SimilarIssue is an existing issue that may describe the same thing as the
// one being filed. Reasons say why it was picked, so the filer (often an
// agent) can judge it without opening the issue. See MF-35.
type SimilarIssue struct {
	ID     uuid.UUID `json:"id"`
	Key    string    `json:"key"`
	Title  string    `json:"title"`
	Type   string    `json:"type"`
	Status string    `json:"status"`
	Score  float64   `json:"score"`
	Level  string    `json:"level"`
	// SharedIdentifiers are the specific tokens (session ids, room/file ids,
	// issue keys, backtick-quoted names) both issues mention.
	SharedIdentifiers []string `json:"shared_identifiers,omitempty"`
	// References is true when the new text mentions this issue's key.
	References bool `json:"references,omitempty"`
	// TitleSimilarity is the character-bigram Jaccard of the two titles, 0..1.
	TitleSimilarity float64 `json:"title_similarity"`
}

// SimilarIssuesRequest asks which existing issues of a project look like the
// given draft. ExcludeKey leaves one issue (the draft itself, once created)
// out of the results.
type SimilarIssuesRequest struct {
	Type        string  `json:"type"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	ExcludeKey  string  `json:"exclude_key,omitempty"`
}

// CreatedIssue is the create response: the issue exactly as before, plus the
// suspected duplicates found at creation time under `similar_issues`. The
// issue's own fields stay at the top level so existing callers are unaffected.
type CreatedIssue struct {
	Issue         *Issue
	SimilarIssues []SimilarIssue
}

func (c CreatedIssue) MarshalJSON() ([]byte, error) {
	base, err := json.Marshal(c.Issue)
	if err != nil {
		return nil, err
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(base, &obj); err != nil {
		return nil, err
	}
	similar := c.SimilarIssues
	if similar == nil {
		similar = []SimilarIssue{}
	}
	raw, err := json.Marshal(similar)
	if err != nil {
		return nil, err
	}
	obj["similar_issues"] = raw
	return json.Marshal(obj)
}
