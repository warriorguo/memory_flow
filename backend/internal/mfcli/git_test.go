package mfcli

import "testing"

func TestNormalizeRepoURL(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"git@github.com:warriorguo/memory_flow.git", "https://github.com/warriorguo/memory_flow"},
		{"ssh://git@github.com/warriorguo/memory_flow.git", "https://github.com/warriorguo/memory_flow"},
		{"https://github.com/warriorguo/memory_flow.git", "https://github.com/warriorguo/memory_flow"},
		{"https://github.com/warriorguo/memory_flow", "https://github.com/warriorguo/memory_flow"},
		{"https://github.com/warriorguo/memory_flow/", "https://github.com/warriorguo/memory_flow"},
		{"git://github.com/warriorguo/memory_flow.git", "https://github.com/warriorguo/memory_flow"},
		{"git@gitlab.example.com:team/sub/proj.git", "https://gitlab.example.com/team/sub/proj"},
		{"  git@github.com:a/b.git  ", "https://github.com/a/b"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := NormalizeRepoURL(tt.in); got != tt.want {
			t.Errorf("NormalizeRepoURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCommitURL(t *testing.T) {
	tests := []struct {
		repo, sha, want string
	}{
		{"git@github.com:warriorguo/memory_flow.git", "5253083",
			"https://github.com/warriorguo/memory_flow/commit/5253083"},
		{"https://gitlab.com/group/proj.git", "abc1234",
			"https://gitlab.com/group/proj/-/commit/abc1234"},
		{"https://bitbucket.org/team/repo.git", "abc1234",
			"https://bitbucket.org/team/repo/commits/abc1234"},
	}
	for _, tt := range tests {
		got, err := CommitURL(tt.repo, tt.sha)
		if err != nil {
			t.Fatalf("CommitURL(%q, %q): %v", tt.repo, tt.sha, err)
		}
		if got != tt.want {
			t.Errorf("CommitURL(%q, %q) = %q, want %q", tt.repo, tt.sha, got, tt.want)
		}
	}
	if _, err := CommitURL("", "abc"); err == nil {
		t.Error("CommitURL with no repo should fail")
	}
}

func TestPRURL(t *testing.T) {
	got, err := PRURL("git@github.com:a/b.git", "42")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://github.com/a/b/pull/42"; got != want {
		t.Errorf("PRURL = %q, want %q", got, want)
	}
	got, err = PRURL("https://gitlab.com/a/b", "42")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://gitlab.com/a/b/-/merge_requests/42"; got != want {
		t.Errorf("PRURL = %q, want %q", got, want)
	}
}

func TestIsURL(t *testing.T) {
	for _, s := range []string{"https://github.com/a/b/commit/x", "http://localhost/x"} {
		if !IsURL(s) {
			t.Errorf("IsURL(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"5253083", "HEAD", "main", ""} {
		if IsURL(s) {
			t.Errorf("IsURL(%q) = true, want false", s)
		}
	}
}

// ExpandSHA falls back to the literal ref when git cannot resolve it, because
// the commit may live in a repository other than the current directory.
func TestExpandSHAFallsBackToShaLikeRef(t *testing.T) {
	got, err := ExpandSHA(t.TempDir(), "5253083")
	if err != nil {
		t.Fatalf("ExpandSHA: %v", err)
	}
	if got != "5253083" {
		t.Errorf("ExpandSHA = %q, want %q", got, "5253083")
	}

	if _, err := ExpandSHA(t.TempDir(), "some-branch-name"); err == nil {
		t.Error("a non-sha ref that git cannot resolve should be an error")
	}
}

func TestProjectKeyOf(t *testing.T) {
	tests := []struct{ in, want string }{
		{"ORT-100", "ORT"},
		{"MF-1", "MF"},
		{"WEBSZ-22", "WEBSZ"},
		{"e5f16f6f-444f-479d-882f-68f589241d40", ""},
		{"not-a-key", ""},
		{"MF", ""},
	}
	for _, tt := range tests {
		if got := projectKeyOf(tt.in); got != tt.want {
			t.Errorf("projectKeyOf(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
