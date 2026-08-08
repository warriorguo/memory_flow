package service

import (
	"reflect"
	"testing"
)

func TestAssetRefs(t *testing.T) {
	tests := []struct {
		name        string
		description string
		want        []string
	}{
		{
			name:        "bare reference",
			description: "参考图见 asset:enemy_ref.png，音效见 asset:hit.wav",
			want:        []string{"enemy_ref.png", "hit.wav"},
		},
		{
			name:        "markdown image and link",
			description: "![参考图](asset:enemy_ref.png)\n[设计稿](asset:spec.pdf)",
			want:        []string{"enemy_ref.png", "spec.pdf"},
		},
		{
			name:        "repeated reference is listed once",
			description: "asset:a.png and again asset:a.png",
			want:        []string{"a.png"},
		},
		{
			name:        "no references",
			description: "just prose, and a URL https://example.com/asset",
			want:        nil,
		},
		{
			name:        "reference at the end of a sentence keeps its punctuation",
			description: "see asset:spec.md.",
			want:        []string{"spec.md."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AssetRefs(tt.description); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AssetRefs() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Prose runs references into sentences, so "asset:spec.md." must still find
// spec.md rather than reporting a broken reference.
func TestResolveAssetRefTrimsTrailingPunctuation(t *testing.T) {
	known := map[string]bool{"spec.md": true, "v1.2.png": true}

	for _, tt := range []struct {
		ref      string
		want     string
		resolved bool
	}{
		{"spec.md", "spec.md", true},
		{"spec.md.", "spec.md", true},
		{"spec.md,", "spec.md", true},
		{"v1.2.png", "v1.2.png", true},
		{"absent.png", "absent.png", false},
	} {
		got, ok := ResolveAssetRef(tt.ref, known)
		if got != tt.want || ok != tt.resolved {
			t.Errorf("ResolveAssetRef(%q) = %q, %v; want %q, %v", tt.ref, got, ok, tt.want, tt.resolved)
		}
	}
}

func TestReferencedIn(t *testing.T) {
	desc := "布局见 ![图](asset:layout.png)，日志 asset:crash.log。"

	for _, tt := range []struct {
		filename string
		want     bool
	}{
		{"layout.png", true},
		{"crash.log", true},
		{"unused.png", false},
	} {
		if got := ReferencedIn(desc, tt.filename); got != tt.want {
			t.Errorf("ReferencedIn(%q) = %v, want %v", tt.filename, got, tt.want)
		}
	}
}
