package model

import (
	"strings"
	"testing"
)

func TestCanTransition(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		{"todo", "in_progress", true},
		{"todo", "done", false},
		{"in_progress", "done", true},
		{"testing", "done", true},
		{"review", "done", false},
		{"done", "closed", true},
		{"closed", "todo", false}, // closed is terminal
		{"nonsense", "todo", false},
	}
	for _, tt := range tests {
		if got := CanTransition(tt.from, tt.to); got != tt.want {
			t.Errorf("CanTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestTransitionPath(t *testing.T) {
	tests := []struct {
		name     string
		from, to string
		want     string // path joined with " "
		wantOK   bool
	}{
		{"no-op", "done", "done", "", true},
		{"single hop", "in_progress", "done", "done", true},
		{"todo to done walks through in_progress", "todo", "done", "in_progress done", true},
		{"review to done walks through testing", "review", "done", "testing done", true},
		{"suspended to done", "suspended", "done", "todo in_progress done", true},
		{"rejected reopens then closes", "rejected", "closed", "todo in_progress done closed", true},
		{"unknown source", "bogus", "done", "", false},
		{"unreachable target", "todo", "bogus", "", false},
		{"closed is terminal", "closed", "todo", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, ok := TransitionPath(tt.from, tt.to)
			if ok != tt.wantOK {
				t.Fatalf("TransitionPath(%q, %q) ok = %v, want %v", tt.from, tt.to, ok, tt.wantOK)
			}
			if got := strings.Join(path, " "); got != tt.want {
				t.Errorf("TransitionPath(%q, %q) = %q, want %q", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

// TestTransitionPathIsLegal guards the contract the CLI relies on: every hop in
// a returned path must itself be a legal single transition, so replaying the
// path against the API cannot be rejected mid-walk.
func TestTransitionPathIsLegal(t *testing.T) {
	statuses := []string{"todo", "in_progress", "review", "testing", "done", "closed", "suspended", "rejected"}
	for _, from := range statuses {
		for _, to := range statuses {
			path, ok := TransitionPath(from, to)
			if !ok {
				continue
			}
			at := from
			for _, next := range path {
				if !CanTransition(at, next) {
					t.Errorf("path %v from %q to %q contains illegal hop %q -> %q", path, from, to, at, next)
				}
				at = next
			}
			if at != to {
				t.Errorf("path %v from %q ends at %q, want %q", path, from, at, to)
			}
		}
	}
}

func TestAllowedTransitions(t *testing.T) {
	if _, ok := AllowedTransitions("todo"); !ok {
		t.Error("todo should be a known status")
	}
	if _, ok := AllowedTransitions("closed"); ok {
		t.Error("closed has no outgoing transitions and should report unknown")
	}
}
