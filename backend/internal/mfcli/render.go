package mfcli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

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

// issueRows renders the standard issue table: Key | Title | Type | Priority | Status | Assignee.
func issueRows(issues []model.Issue) [][]string {
	rows := make([][]string, 0, len(issues))
	for _, issue := range issues {
		rows = append(rows, []string{
			issue.IssueKey,
			truncate(issue.Title, 60),
			issue.Type,
			issue.Priority,
			issue.Status,
			dash(issue.AssigneeID),
		})
	}
	return rows
}

var issueHeader = []string{"KEY", "TITLE", "TYPE", "PRI", "STATUS", "ASSIGNEE"}

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
