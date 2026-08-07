package mfcli

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// NormalizeRepoURL turns any form of git remote into its browsable https base,
// with no trailing ".git" or slash:
//
//	git@github.com:owner/repo.git        -> https://github.com/owner/repo
//	ssh://git@github.com/owner/repo.git  -> https://github.com/owner/repo
//	https://github.com/owner/repo.git    -> https://github.com/owner/repo
func NormalizeRepoURL(remote string) string {
	u := strings.TrimSpace(remote)
	if u == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(u, "ssh://"):
		u = "https://" + strings.TrimPrefix(u, "ssh://")
		u = strings.Replace(u, "https://git@", "https://", 1)
	case strings.HasPrefix(u, "git://"):
		u = "https://" + strings.TrimPrefix(u, "git://")
	case strings.Contains(u, "@") && strings.Contains(u, ":") && !strings.Contains(u, "://"):
		// scp-style: [user@]host:path
		at := strings.Index(u, "@")
		rest := u[at+1:]
		u = "https://" + strings.Replace(rest, ":", "/", 1)
	}
	u = strings.TrimSuffix(u, "/")
	u = strings.TrimSuffix(u, ".git")
	return u
}

// CommitURL builds the web URL for a commit in the given repository. Hosting
// providers disagree on the path segment, so the host picks it.
func CommitURL(repoURL, sha string) (string, error) {
	base := NormalizeRepoURL(repoURL)
	if base == "" {
		return "", fmt.Errorf("no repository URL available")
	}
	segment := "/commit/"
	switch {
	case strings.Contains(base, "gitlab"):
		segment = "/-/commit/"
	case strings.Contains(base, "bitbucket"):
		segment = "/commits/"
	}
	return base + segment + sha, nil
}

// PRURL builds the web URL for a pull/merge request number.
func PRURL(repoURL, number string) (string, error) {
	base := NormalizeRepoURL(repoURL)
	if base == "" {
		return "", fmt.Errorf("no repository URL available")
	}
	segment := "/pull/"
	switch {
	case strings.Contains(base, "gitlab"):
		segment = "/-/merge_requests/"
	case strings.Contains(base, "bitbucket"):
		segment = "/pull-requests/"
	}
	return base + segment + number, nil
}

var shaLike = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)

// IsURL reports whether ref is already a full URL, in which case it is used
// verbatim rather than being expanded into a commit URL.
func IsURL(ref string) bool {
	return strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://")
}

// ExpandSHA resolves a commit-ish (short sha, HEAD, branch name) to its full
// sha using the git repo at dir. If git cannot answer — no repo, no git, an
// unknown ref — an already sha-looking ref is used as given, since the commit
// may live in a repository other than the current directory.
func ExpandSHA(dir, ref string) (string, error) {
	out, err := gitOutput(dir, "rev-parse", "--verify", ref+"^{commit}")
	if err == nil && out != "" {
		return out, nil
	}
	if shaLike.MatchString(ref) {
		return ref, nil
	}
	return "", fmt.Errorf("cannot resolve %q to a commit: not a sha, and `git rev-parse` failed in %s", ref, dirLabel(dir))
}

// RepoRemote returns the origin remote URL of the repo at dir, if any.
func RepoRemote(dir string) string {
	out, err := gitOutput(dir, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return out
}

// HeadSHA returns the current commit of the repo at dir, if any.
func HeadSHA(dir string) string {
	out, err := gitOutput(dir, "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return out
}

func gitOutput(dir string, args ...string) (string, error) {
	full := append([]string{"-C", dirOrDot(dir)}, args...)
	out, err := exec.Command("git", full...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func dirOrDot(dir string) string {
	if dir == "" {
		return "."
	}
	return dir
}

func dirLabel(dir string) string {
	if dir == "" {
		return "the current directory"
	}
	return dir
}
