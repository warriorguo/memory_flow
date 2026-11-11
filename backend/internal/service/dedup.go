package service

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/warriorguo/memory_flow/backend/internal/model"
)

// Duplicate detection (MF-35).
//
// Two issues about the same bug are usually worded differently but cite the
// same concrete things: a telemetry session directory, a room id, a file, an
// issue key. So the strong signal is shared *specific identifiers*; a
// character-bigram title similarity catches the cases where the titles are
// simply close. Both are cheap, need no index (so an issue created a minute
// ago is already visible) and run the same on PostgreSQL and SQLite.
//
// Embeddings were evaluated and left out of the first cut: a pure-Go model
// ranked the motivating pair first but with a 0.004 margin and ~2–5GB RSS
// (see the recall memory on MF-35). A new signal (e.g. embeddings) adds a
// term to the score in rankSimilar.

const (
	maxSimilarResults = 5
	// Title bigram Jaccard at or above this alone makes a "possible" match.
	titlePossibleThreshold = 0.45
)

var (
	backtickRe = regexp.MustCompile("`([^`\n]{3,120})`")
	issueKeyRe = regexp.MustCompile(`\b[A-Z][A-Z0-9]{1,9}-\d+\b`)
	tokenRe    = regexp.MustCompile(`[A-Za-z0-9_./\-]{6,}`)
	digitRe    = regexp.MustCompile(`\d`)
	sepRe      = regexp.MustCompile(`[_./\-]`)
	// Paths (a/b/c) and file names (DoorSpawn.cs) name a concrete thing even
	// without a digit.
	pathRe     = regexp.MustCompile(`^[A-Za-z0-9_.\-]+(/[A-Za-z0-9_.\-]+)+$`)
	fileNameRe = regexp.MustCompile(`^[A-Za-z0-9_\-]{2,}\.[A-Za-z][A-Za-z0-9]{0,5}$`)
	// Bare numbers, dates and versions ("2026-11-11", "1.4.2") recur across
	// unrelated issues; they are not identifiers on their own.
	numericOnlyRe = regexp.MustCompile(`^[0-9._/\-:]+$`)
)

// dedupDoc is one issue (or the draft) prepared for comparison.
type dedupDoc struct {
	issue   *model.Issue
	ids     map[string]bool
	bigrams map[string]bool
}

// extractIdentifiers returns the specific tokens in text, lowercased:
// backtick-quoted spans, issue keys, paths, file names, and tokens of 6+
// chars that mix a digit with a separator (session dirs, room ids).
func extractIdentifiers(text string) map[string]bool {
	ids := map[string]bool{}
	add := func(s string) {
		s = strings.Trim(strings.TrimSpace(s), ".-/_:")
		if len([]rune(s)) < 3 {
			return
		}
		ids[strings.ToLower(s)] = true
	}
	for _, m := range backtickRe.FindAllStringSubmatch(text, -1) {
		span := strings.TrimSpace(m[1])
		// A quoted sentence is prose, not a name.
		if strings.ContainsAny(span, " \t") && len([]rune(span)) > 40 {
			continue
		}
		if numericOnlyRe.MatchString(span) && len(span) <= 10 {
			continue
		}
		add(span)
	}
	for _, k := range issueKeyRe.FindAllString(text, -1) {
		add(k)
	}
	for _, t := range tokenRe.FindAllString(text, -1) {
		t = strings.Trim(t, ".-/_")
		if len(t) < 6 {
			continue
		}
		mixed := digitRe.MatchString(t) && sepRe.MatchString(t)
		if !mixed && !pathRe.MatchString(t) && !fileNameRe.MatchString(t) {
			continue
		}
		if numericOnlyRe.MatchString(t) && len(t) <= 10 {
			continue
		}
		add(t)
	}
	return ids
}

var issueKeyIDRe = regexp.MustCompile(`^[a-z][a-z0-9]{1,9}-\d+$`)

// identifierWeight separates instance identifiers from names. A session dir
// or room id (several digits plus separators, e.g. 2026-11-11_172018,
// story_3_f1_room_4_-1) pins down one occurrence and weighs 1. Code names,
// paths, file names and cited issue keys are shared by every issue about the
// same subsystem, so they weigh 0.5: two issues need several of them, or one
// instance id, to look like the same bug.
func identifierWeight(id string) float64 {
	if issueKeyIDRe.MatchString(id) {
		return 0.5
	}
	if len(id) >= 8 && len(digitRe.FindAllString(id, -1)) >= 2 && sepRe.MatchString(id) {
		return 1
	}
	return 0.5
}

// titleBigrams is the set of adjacent-rune pairs of a title, ignoring case,
// spaces and punctuation, so it works for Chinese and English alike.
func titleBigrams(title string) map[string]bool {
	var rs []rune
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			rs = append(rs, r)
		}
	}
	out := map[string]bool{}
	for i := 0; i+1 < len(rs); i++ {
		out[string(rs[i:i+2])] = true
	}
	return out
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if b[k] {
			inter++
		}
	}
	return float64(inter) / float64(len(a)+len(b)-inter)
}

func newDedupDoc(issue *model.Issue) *dedupDoc {
	text := issue.Title
	if issue.Description != nil {
		text += "\n" + *issue.Description
	}
	return &dedupDoc{issue: issue, ids: extractIdentifiers(text), bigrams: titleBigrams(issue.Title)}
}

// rankSimilar scores every candidate against the draft and returns the
// suspected duplicates, best first. excludeKey drops one issue (the draft
// itself, when it already exists).
func rankSimilar(draft *model.Issue, candidates []model.Issue, excludeKey string) []model.SimilarIssue {
	q := newDedupDoc(draft)
	docs := make([]*dedupDoc, 0, len(candidates))
	for i := range candidates {
		if excludeKey != "" && strings.EqualFold(candidates[i].IssueKey, excludeKey) {
			continue
		}
		docs = append(docs, newDedupDoc(&candidates[i]))
	}

	// Document frequency of each draft identifier across the pool: an
	// identifier half the project mentions says little about any one issue.
	df := map[string]int{}
	for _, d := range docs {
		for id := range q.ids {
			if d.ids[id] {
				df[id]++
			}
		}
	}
	// An identifier found in more than ~2% of the pool (at least 3 issues) is
	// generic and scores a quarter of its weight.
	genericAt := int(math.Max(3, float64(len(docs))/50))

	type scored struct {
		model.SimilarIssue
		created time.Time
	}
	var hits []scored
	for _, d := range docs {
		var shared []string
		idScore, strong := 0.0, false
		for id := range q.ids {
			if !d.ids[id] {
				continue
			}
			shared = append(shared, id)
			w := identifierWeight(id)
			if df[id] > genericAt {
				w /= 4
			}
			if w >= 1 {
				strong = true
			}
			idScore += w
		}
		sort.Strings(shared)
		refs := q.ids[strings.ToLower(d.issue.IssueKey)]
		titleSim := jaccard(q.bigrams, d.bigrams)

		// Citing an issue's key usually means "related to", not "same as",
		// so a reference alone is only a possible match.
		level := ""
		switch {
		case strong && idScore >= 2:
			level = model.SimilarLikely
		case strong || idScore >= 1.5 || refs || titleSim >= titlePossibleThreshold:
			level = model.SimilarPossible
		}
		if level == "" {
			continue
		}
		score := idScore + titleSim
		if refs {
			score++
		}
		hits = append(hits, scored{created: d.issue.CreatedAt, SimilarIssue: model.SimilarIssue{
			ID: d.issue.ID, Key: d.issue.IssueKey, Title: d.issue.Title,
			Type: d.issue.Type, Status: d.issue.Status,
			Score: math.Round(score*100) / 100, Level: level,
			SharedIdentifiers: shared, References: refs,
			TitleSimilarity: math.Round(titleSim*100) / 100,
		}})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].created.After(hits[j].created) // newer first on ties
	})
	if len(hits) > maxSimilarResults {
		hits = hits[:maxSimilarResults]
	}
	out := make([]model.SimilarIssue, len(hits))
	for i, h := range hits {
		out[i] = h.SimilarIssue
	}
	return out
}
