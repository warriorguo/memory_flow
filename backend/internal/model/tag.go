package model

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// TagPalette is the set of colors a tag gets when nobody picks one. They are
// GitHub-label-like: saturated enough to tell apart in a list, and each one
// readable with either black or white text (see [TagTextColor]).
//
// Migration 000012 backfills color-less tags from this same list using
// [PaletteIndex]'s formula, so keep the order in sync with those SQL files.
var TagPalette = []string{
	"#d73a4a", // red
	"#0075ca", // blue
	"#a2eeef", // cyan
	"#7057ff", // purple
	"#008672", // teal
	"#e4e669", // yellow
	"#d876e3", // pink
	"#fbca04", // amber
	"#0e8a16", // green
	"#b60205", // dark red
	"#1d76db", // mid blue
	"#f9d0c4", // peach
}

// PaletteIndex is the palette slot a name hashes to: first code point plus
// length in characters. It is deliberately simple so SQL (PostgreSQL's ascii()
// and SQLite's unicode()) can compute the very same value in the backfill.
func PaletteIndex(name string) int {
	if name == "" {
		return 0
	}
	first, _ := utf8.DecodeRuneInString(name)
	return (int(first) + utf8.RuneCountInString(name)) % len(TagPalette)
}

// PickTagColor chooses a color for a new tag: the palette entry fewest existing
// tags use, so colors spread out as tags accumulate. Ties go to the name's own
// slot (then onward), which keeps the choice deterministic.
func PickTagColor(name string, existing []Tag) string {
	used := make(map[string]int, len(TagPalette))
	for _, t := range existing {
		if t.Color != nil {
			used[strings.ToLower(*t.Color)]++
		}
	}
	start := PaletteIndex(name)
	best, bestCount := TagPalette[start], -1
	for i := range TagPalette {
		c := TagPalette[(start+i)%len(TagPalette)]
		if n := used[c]; bestCount < 0 || n < bestCount {
			best, bestCount = c, n
		}
	}
	return best
}

// NormalizeTagColor accepts "#rrggbb", "rrggbb" or the "#rgb" shorthand (any
// case) and returns the lowercase "#rrggbb" form the column stores.
func NormalizeTagColor(s string) (string, error) {
	c := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(s), "#"))
	if len(c) == 3 {
		c = string([]byte{c[0], c[0], c[1], c[1], c[2], c[2]})
	}
	if len(c) != 6 {
		return "", fmt.Errorf("invalid tag color %q: want a hex color like #1890ff", s)
	}
	if _, err := strconv.ParseUint(c, 16, 32); err != nil {
		return "", fmt.Errorf("invalid tag color %q: want a hex color like #1890ff", s)
	}
	return "#" + c, nil
}

// TagRGB parses a stored "#rrggbb" color. ok is false for anything else.
func TagRGB(color string) (r, g, b uint8, ok bool) {
	c, err := NormalizeTagColor(color)
	if err != nil {
		return 0, 0, 0, false
	}
	v, _ := strconv.ParseUint(c[1:], 16, 32)
	return uint8(v >> 16), uint8(v >> 8), uint8(v), true
}

// TagTextColor returns "#000000" or "#ffffff", whichever reads better on the
// given background, using the perceived-brightness formula (ITU-R BT.601).
// The web UI applies the same rule so a label looks the same everywhere.
func TagTextColor(background string) string {
	r, g, b, ok := TagRGB(background)
	if !ok {
		return "#000000"
	}
	if (299*int(r)+587*int(g)+114*int(b))/1000 >= 150 {
		return "#000000"
	}
	return "#ffffff"
}
