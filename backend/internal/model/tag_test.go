package model

import "testing"

func TestNormalizeTagColor(t *testing.T) {
	for in, want := range map[string]string{"#1890FF": "#1890ff", "1890ff": "#1890ff", "#abc": "#aabbcc", " #0E8A16 ": "#0e8a16"} {
		if got, err := NormalizeTagColor(in); err != nil || got != want {
			t.Errorf("NormalizeTagColor(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "blue", "#12345", "#gggggg", "#1234567"} {
		if _, err := NormalizeTagColor(bad); err == nil {
			t.Errorf("NormalizeTagColor(%q) accepted", bad)
		}
	}
}

func TestTagTextColorContrast(t *testing.T) {
	for bg, want := range map[string]string{"#ffffff": "#000000", "#000000": "#ffffff", "#fbca04": "#000000", "#0075ca": "#ffffff", "junk": "#000000"} {
		if got := TagTextColor(bg); got != want {
			t.Errorf("TagTextColor(%s) = %s, want %s", bg, got, want)
		}
	}
}

func TestPickTagColorSpreadsAndIsDeterministic(t *testing.T) {
	if PickTagColor("room", nil) != TagPalette[PaletteIndex("room")] {
		t.Error("empty set should use the name's own slot")
	}
	var existing []Tag
	seen := map[string]bool{}
	for i := 0; i < len(TagPalette); i++ {
		c := PickTagColor("x", existing)
		if seen[c] {
			t.Fatalf("color %s reused before the palette ran out", c)
		}
		seen[c] = true
		existing = append(existing, Tag{Name: "x", Color: &c})
	}
}
