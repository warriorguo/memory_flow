package mfcli

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/warriorguo/memory_flow/backend/internal/model"
)

// table writes rows in aligned columns. Output is plain text so it stays
// readable in a terminal and cheap to parse when piped.
func table(w io.Writer, header []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if len(header) > 0 {
		fmt.Fprintln(tw, strings.Join(header, "\t"))
	}
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	tw.Flush()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// dash renders an empty optional value as "-" so columns stay aligned.
func dash(s *string) string {
	if v := deref(s); v != "" {
		return v
	}
	return "-"
}

// truncate shortens s to at most n runes, marking the cut with an ellipsis.
func truncate(s string, n int) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", "")
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

func shortTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04")
}

// issueRows renders the standard issue table: Key | Title | Type | Priority |
// Status | Assignee. Tags follow the title, GitHub-style, via labels.
func issueRows(issues []model.Issue, labels func([]model.Tag) string) [][]string {
	rows := make([][]string, 0, len(issues))
	for _, issue := range issues {
		rows = append(rows, []string{
			issue.IssueKey,
			truncate(issue.Title, 60) + labels(issue.Tags),
			issue.Type,
			issue.Priority,
			issue.Status,
			dash(issue.AssigneeID),
		})
	}
	return rows
}

var issueHeader = []string{"KEY", "TITLE", "TYPE", "PRI", "STATUS", "ASSIGNEE"}

// issueTable prints issues in the standard table, coloring tag labels when
// writing to a terminal.
func (e *env) issueTable(issues []model.Issue) {
	rows := issueRows(issues, e.tagLabels)
	if e.color {
		ansiTable(e.out, issueHeader, rows)
		return
	}
	table(e.out, issueHeader, rows)
}

// tagLabels renders tags for the end of a title: " [room] [enemy-ai]" as plain
// text, or as labels filled with each tag's color on a color terminal.
func (e *env) tagLabels(tags []model.Tag) string {
	if len(tags) == 0 {
		return ""
	}
	var b strings.Builder
	for _, t := range tags {
		b.WriteByte(' ')
		if e.color {
			b.WriteString(colorLabel(deref(t.Color), " "+t.Name+" "))
		} else {
			b.WriteString("[" + t.Name + "]")
		}
	}
	return b.String()
}

// colorLabel paints text on the given background with whichever of black or
// white reads better on it (the same rule the web UI uses). An unparseable
// color falls back to brackets so the label is still visible.
func colorLabel(background, text string) string {
	r, g, b, ok := model.TagRGB(background)
	if !ok {
		return "[" + strings.TrimSpace(text) + "]"
	}
	fg := "30" // black
	if model.TagTextColor(background) == "#ffffff" {
		fg = "97" // bright white
	}
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm\x1b[%sm%s\x1b[0m", r, g, b, fg, text)
}

var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*m")

// visibleLen is a cell's width once ANSI color codes are stripped, counted in
// runes like tabwriter does.
func visibleLen(s string) int {
	return utf8.RuneCountInString(ansiEscape.ReplaceAllString(s, ""))
}

// ansiTable is table for cells that contain color codes: tabwriter would count
// the invisible escape bytes as width and misalign every column after them.
func ansiTable(w io.Writer, header []string, rows [][]string) {
	all := rows
	if len(header) > 0 {
		all = append([][]string{header}, rows...)
	}
	var widths []int
	for _, row := range all {
		for i, cell := range row {
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			if n := visibleLen(cell); n > widths[i] {
				widths[i] = n
			}
		}
	}
	for _, row := range all {
		var b strings.Builder
		for i, cell := range row {
			b.WriteString(cell)
			if i < len(row)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-visibleLen(cell)+2))
			}
		}
		fmt.Fprintln(w, b.String())
	}
}

// printIssue renders a single issue in full, including its description.
func printIssue(w io.Writer, issue *model.Issue) {
	fmt.Fprintf(w, "%s  %s\n", issue.IssueKey, issue.Title)
	fmt.Fprintf(w, "  type      %s\n", issue.Type)
	fmt.Fprintf(w, "  status    %s\n", issue.Status)
	fmt.Fprintf(w, "  priority  %s\n", issue.Priority)
	fmt.Fprintf(w, "  assignee  %s\n", dash(issue.AssigneeID))
	if v := deref(issue.Version); v != "" {
		fmt.Fprintf(w, "  version   %s\n", v)
	}
	if v := deref(issue.Source); v != "" {
		fmt.Fprintf(w, "  source    %s\n", v)
	}
	fmt.Fprintf(w, "  git_url   %s\n", dash(issue.GitURL))
	if v := deref(issue.PRURL); v != "" {
		fmt.Fprintf(w, "  pr_url    %s\n", v)
	}
	if v := deref(issue.DocURL); v != "" {
		fmt.Fprintf(w, "  doc_url   %s\n", v)
	}
	if len(issue.Tags) > 0 {
		names := make([]string, 0, len(issue.Tags))
		for _, t := range issue.Tags {
			names = append(names, t.Name)
		}
		fmt.Fprintf(w, "  tags      %s\n", strings.Join(names, ", "))
	}
	fmt.Fprintf(w, "  created   %s\n", shortTime(issue.CreatedAt))
	fmt.Fprintf(w, "  updated   %s\n", shortTime(issue.UpdatedAt))
	if desc := strings.TrimSpace(deref(issue.Description)); desc != "" {
		fmt.Fprintf(w, "\n%s\n", desc)
	}
}

// printDepTree renders the dependency tree as an indented outline.
func printDepTree(w io.Writer, node *model.DependencyNode, indent string, relation string) {
	label := fmt.Sprintf("%s [%s/%s] %s", node.IssueKey, node.Priority, node.Status, truncate(node.Title, 60))
	if relation != "" {
		label = relation + " " + label
	}
	if node.Severity != "" && relation != "" {
		label += fmt.Sprintf("  (%s)", node.Severity)
	}
	fmt.Fprintln(w, indent+label)
	for _, child := range node.DependsOn {
		printDepTree(w, child, indent+"  ", "depends on →")
	}
	for _, child := range node.Blocks {
		printDepTree(w, child, indent+"  ", "blocks →")
	}
}

// sortedCounts renders a count map deterministically, ordered by the given
// preferred sequence first and then alphabetically for anything unrecognized.
func sortedCounts(counts map[string]int, preferred []string) string {
	seen := map[string]bool{}
	var parts []string
	for _, key := range preferred {
		if n, ok := counts[key]; ok {
			parts = append(parts, fmt.Sprintf("%d %s", n, key))
			seen[key] = true
		}
	}
	var extra []string
	for key := range counts {
		if !seen[key] {
			extra = append(extra, key)
		}
	}
	sort.Strings(extra)
	for _, key := range extra {
		parts = append(parts, fmt.Sprintf("%d %s", counts[key], key))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}
