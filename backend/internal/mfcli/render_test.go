package mfcli

import (
	"strings"
	"testing"

	"github.com/warriorguo/memory_flow/backend/internal/model"
)

func TestTagLabelsPlainAndColor(t *testing.T) {
	red, bad := "#d73a4a", "nope"
	tags := []model.Tag{{Name: "room", Color: &red}, {Name: "ai", Color: &bad}, {Name: "bare"}}

	plain := (&env{}).tagLabels(tags)
	if plain != " [room] [ai] [bare]" {
		t.Errorf("plain = %q", plain)
	}

	colored := (&env{color: true}).tagLabels(tags)
	if !strings.Contains(colored, "\x1b[48;2;215;58;74m\x1b[97m room \x1b[0m") {
		t.Errorf("room should be white on red: %q", colored)
	}
	if !strings.Contains(colored, "[ai]") || !strings.Contains(colored, "[bare]") {
		t.Errorf("colorless tags should fall back to brackets: %q", colored)
	}
	if (&env{color: true}).tagLabels(nil) != "" {
		t.Error("no tags should render nothing")
	}
}

// Color codes must not count toward column width, or every column after a
// colored title drifts.
func TestAnsiTableAlignsAroundEscapes(t *testing.T) {
	yellow := "#fbca04"
	e := &env{color: true}
	rows := [][]string{
		{"MF-1", "short" + e.tagLabels([]model.Tag{{Name: "ui", Color: &yellow}}), "bug"},
		{"MF-22", "a much longer title", "requirement"},
	}
	var b strings.Builder
	ansiTable(&b, []string{"KEY", "TITLE", "TYPE"}, rows)

	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	col := -1
	for i, line := range lines {
		visible := ansiEscape.ReplaceAllString(line, "")
		idx := strings.LastIndex(visible, " ") + 1
		if i == 0 {
			col = idx
		} else if idx != col {
			t.Errorf("line %d third column at %d, want %d:\n%s", i, idx, col, visible)
		}
	}
}
