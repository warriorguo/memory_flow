package model

import (
	"encoding/json"
	"testing"
)

// TestIssueMarshalKeyAlias verifies that issue responses expose the key under
// both `key` (matching the project resource) and `issue_key` (deprecated alias),
// and that the JSON round-trips back into IssueKey. See MF-18.
func TestIssueMarshalKeyAlias(t *testing.T) {
	in := Issue{IssueKey: "MF-18", Title: "x"}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	if got := string(m["key"]); got != `"MF-18"` {
		t.Errorf("key = %s, want \"MF-18\"", got)
	}
	if got := string(m["issue_key"]); got != `"MF-18"` {
		t.Errorf("issue_key = %s, want \"MF-18\"", got)
	}

	var out Issue
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if out.IssueKey != "MF-18" {
		t.Errorf("round-trip IssueKey = %q, want MF-18", out.IssueKey)
	}
}
