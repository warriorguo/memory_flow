package mfcli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExtractGlobals(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantRest []string
		wantURL  string
		wantJSON bool
	}{
		{
			name:     "globals before the command",
			args:     []string{"--json", "issue", "show", "MF-1"},
			wantRest: []string{"issue", "show", "MF-1"},
			wantJSON: true,
		},
		{
			name:     "globals after the command",
			args:     []string{"issue", "show", "MF-1", "--json"},
			wantRest: []string{"issue", "show", "MF-1"},
			wantJSON: true,
		},
		{
			name:     "url with a separate value",
			args:     []string{"--url", "http://x:9", "ctx"},
			wantRest: []string{"ctx"},
			wantURL:  "http://x:9",
		},
		{
			name:     "url with an inline value",
			args:     []string{"--url=http://x:9", "ctx"},
			wantRest: []string{"ctx"},
			wantURL:  "http://x:9",
		},
		{
			name:     "subcommand flags are left alone",
			args:     []string{"issue", "create", "MF", "--title", "x", "--json"},
			wantRest: []string{"issue", "create", "MF", "--title", "x"},
			wantJSON: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, rest, err := extractGlobals(tt.args)
			if err != nil {
				t.Fatalf("extractGlobals: %v", err)
			}
			if strings.Join(rest, " ") != strings.Join(tt.wantRest, " ") {
				t.Errorf("rest = %v, want %v", rest, tt.wantRest)
			}
			if opts.url != tt.wantURL {
				t.Errorf("url = %q, want %q", opts.url, tt.wantURL)
			}
			if opts.asJSON != tt.wantJSON {
				t.Errorf("asJSON = %v, want %v", opts.asJSON, tt.wantJSON)
			}
		})
	}

	if _, _, err := extractGlobals([]string{"ctx", "--url"}); err == nil {
		t.Error("--url without a value should fail")
	}
	if _, _, err := extractGlobals([]string{"ctx", "--timeout", "soon"}); err == nil {
		t.Error("--timeout with a non-numeric value should fail")
	}
}

func TestParseArgs(t *testing.T) {
	fs := newFlagSet("test")
	title := fs.String("title", "", "")
	pos, err := parseArgs(fs, []string{"MF", "--title", "hello world"})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if len(pos) != 1 || pos[0] != "MF" {
		t.Errorf("positional = %v, want [MF]", pos)
	}
	if *title != "hello world" {
		t.Errorf("title = %q, want %q", *title, "hello world")
	}

	// A positional after a flag is a mistake worth reporting, not silently dropping.
	fs2 := newFlagSet("test")
	fs2.String("title", "", "")
	if _, err := parseArgs(fs2, []string{"--title", "x", "MF"}); err == nil {
		t.Error("a trailing positional should be rejected")
	}
}

func TestNeed(t *testing.T) {
	if err := need([]string{"MF-1"}, 1, "issue show <KEY>"); err != nil {
		t.Errorf("exact arity should pass: %v", err)
	}
	if err := need(nil, 1, "issue show <KEY>"); err == nil {
		t.Error("too few arguments should fail")
	}
	if err := need([]string{"MF-1", "extra"}, 1, "issue show <KEY>"); err == nil {
		t.Error("too many arguments should fail")
	}
}

// --- end-to-end command tests against a stub API ---------------------------

// stubAPI records requests and replies with canned responses keyed by
// "METHOD /path".
type stubAPI struct {
	t         *testing.T
	responses map[string]string
	requests  []string
	bodies    map[string]string
}

func newStubAPI(t *testing.T, responses map[string]string) (*stubAPI, *httptest.Server) {
	s := &stubAPI{t: t, responses: responses, bodies: map[string]string{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		s.requests = append(s.requests, key)
		if r.Body != nil {
			buf := make([]byte, 4096)
			n, _ := r.Body.Read(buf)
			if n > 0 {
				s.bodies[key] = string(buf[:n])
			}
		}
		body, ok := s.responses[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"stub has no response for ` + key + `"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return s, srv
}

func (s *stubAPI) called(key string) int {
	n := 0
	for _, req := range s.requests {
		if req == key {
			n++
		}
	}
	return n
}

func runCLI(t *testing.T, url string, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr strings.Builder
	full := append([]string{"--url", url}, args...)
	code := Main(full, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func issueJSON(key, status, gitURL string) string {
	git := "null"
	if gitURL != "" {
		git = `"` + gitURL + `"`
	}
	return `{"data":{"id":"11111111-1111-1111-1111-111111111111","key":"` + key + `","issue_key":"` + key +
		`","project_id":"22222222-2222-2222-2222-222222222222","type":"bug","title":"Something broke",` +
		`"priority":"P1","status":"` + status + `","git_url":` + git + `}}`
}

func TestIssueShow(t *testing.T) {
	_, srv := newStubAPI(t, map[string]string{
		"GET /api/v1/issues/MF-1": issueJSON("MF-1", "todo", ""),
	})
	stdout, stderr, code := runCLI(t, srv.URL, "issue", "show", "MF-1")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	for _, want := range []string{"MF-1", "Something broke", "todo", "P1"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output missing %q:\n%s", want, stdout)
		}
	}
}

func TestIssueShowJSON(t *testing.T) {
	_, srv := newStubAPI(t, map[string]string{
		"GET /api/v1/issues/MF-1": issueJSON("MF-1", "todo", ""),
	})
	stdout, _, code := runCLI(t, srv.URL, "issue", "show", "MF-1", "--json")
	if code != 0 {
		t.Fatal("expected success")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\n%s", err, stdout)
	}
	if parsed["data"] == nil {
		t.Error("--json should emit the full response envelope")
	}
}

// A todo issue cannot go straight to done, so `mf issue done` must walk through
// in_progress rather than surfacing the API's rejection.
func TestIssueDoneWalksTransitions(t *testing.T) {
	stub, srv := newStubAPI(t, map[string]string{
		"GET /api/v1/issues/MF-1":          issueJSON("MF-1", "todo", "https://github.com/a/b/commit/abc"),
		"PATCH /api/v1/issues/MF-1/status": issueJSON("MF-1", "done", "https://github.com/a/b/commit/abc"),
	})
	stdout, stderr, code := runCLI(t, srv.URL, "issue", "done", "MF-1")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if got := stub.called("PATCH /api/v1/issues/MF-1/status"); got != 2 {
		t.Errorf("status transitions = %d, want 2 (todo -> in_progress -> done)", got)
	}
	if !strings.Contains(stdout, "in_progress") || !strings.Contains(stdout, "done") {
		t.Errorf("output should name the walked path:\n%s", stdout)
	}
}

// The completion workflow requires a commit link; closing without one is the
// mistake the CLI exists to prevent.
func TestIssueDoneRequiresGitURL(t *testing.T) {
	stub, srv := newStubAPI(t, map[string]string{
		"GET /api/v1/issues/MF-1": issueJSON("MF-1", "in_progress", ""),
	})
	_, stderr, code := runCLI(t, srv.URL, "issue", "done", "MF-1")
	if code == 0 {
		t.Fatal("closing an issue with no git_url should fail")
	}
	if !strings.Contains(stderr, "attach-git") {
		t.Errorf("the error should point at the fix:\n%s", stderr)
	}
	if n := stub.called("PATCH /api/v1/issues/MF-1/status"); n != 0 {
		t.Errorf("no transition should have been attempted, got %d", n)
	}
}

func TestIssueDoneForce(t *testing.T) {
	stub, srv := newStubAPI(t, map[string]string{
		"GET /api/v1/issues/MF-1":          issueJSON("MF-1", "in_progress", ""),
		"PATCH /api/v1/issues/MF-1/status": issueJSON("MF-1", "done", ""),
	})
	_, stderr, code := runCLI(t, srv.URL, "issue", "done", "MF-1", "--force")
	if code != 0 {
		t.Fatalf("--force should close anyway, stderr: %s", stderr)
	}
	if n := stub.called("PATCH /api/v1/issues/MF-1/status"); n != 1 {
		t.Errorf("transitions = %d, want 1", n)
	}
}

// attach-git turns a bare sha into a commit URL using the project's git_url.
func TestIssueAttachGit(t *testing.T) {
	stub, srv := newStubAPI(t, map[string]string{
		"GET /api/v1/projects/MF": `{"data":{"id":"22222222-2222-2222-2222-222222222222","key":"MF",` +
			`"name":"Memory Flow","status":"active","git_url":"https://github.com/warriorguo/memory_flow.git"}}`,
		"PUT /api/v1/issues/MF-1": issueJSON("MF-1", "todo", "https://github.com/warriorguo/memory_flow/commit/5253083"),
	})
	stdout, stderr, code := runCLI(t, srv.URL, "issue", "attach-git", "MF-1", "5253083")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	body := stub.bodies["PUT /api/v1/issues/MF-1"]
	if !strings.Contains(body, "https://github.com/warriorguo/memory_flow/commit/5253083") {
		t.Errorf("request body should carry the built commit URL, got: %s", body)
	}
	if !strings.Contains(stdout, "/commit/5253083") {
		t.Errorf("output should show the recorded URL:\n%s", stdout)
	}
}

// A full URL is recorded verbatim, without consulting the project or git.
func TestIssueAttachGitAcceptsURL(t *testing.T) {
	url := "https://github.com/warriorguo/memory_flow/pull/7"
	stub, srv := newStubAPI(t, map[string]string{
		"PUT /api/v1/issues/MF-1": issueJSON("MF-1", "todo", url),
	})
	_, stderr, code := runCLI(t, srv.URL, "issue", "attach-git", "MF-1", url)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if n := stub.called("GET /api/v1/projects/MF"); n != 0 {
		t.Error("a full URL should not trigger a project lookup")
	}
}

func TestIssueListHidesClosedByDefault(t *testing.T) {
	list := `{"data":[` +
		`{"id":"11111111-1111-1111-1111-111111111111","key":"MF-1","issue_key":"MF-1","type":"bug","title":"Open one","priority":"P1","status":"todo"},` +
		`{"id":"33333333-3333-3333-3333-333333333333","key":"MF-2","issue_key":"MF-2","type":"bug","title":"Closed one","priority":"P2","status":"done"}` +
		`],"total":2,"page":1,"page_size":200}`
	_, srv := newStubAPI(t, map[string]string{"GET /api/v1/projects/MF/issues": list})

	stdout, stderr, code := runCLI(t, srv.URL, "issues", "MF")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "MF-1") {
		t.Error("open issues should be listed")
	}
	if strings.Contains(stdout, "MF-2") {
		t.Error("done issues should be hidden without --all")
	}

	stdout, _, _ = runCLI(t, srv.URL, "issues", "MF", "--all")
	if !strings.Contains(stdout, "MF-2") {
		t.Error("--all should include done issues")
	}
}

// An API error should surface its message, not a raw status code.
func TestAPIErrorIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"issue not found"}`))
	}))
	t.Cleanup(srv.Close)

	_, stderr, code := runCLI(t, srv.URL, "issue", "show", "MF-999")
	if code == 0 {
		t.Fatal("a 404 should be a non-zero exit")
	}
	if !strings.Contains(stderr, "issue not found") {
		t.Errorf("stderr should carry the API message: %s", stderr)
	}
}

func TestUnknownCommand(t *testing.T) {
	_, stderr, code := runCLI(t, "http://127.0.0.1:1", "wat")
	if code == 0 {
		t.Fatal("unknown commands should fail")
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestHelpExitsZero(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := Main([]string{"help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help exit = %d", code)
	}
	if !strings.Contains(stdout.String(), "mf issue attach-git") {
		t.Error("root help should show the core workflow example")
	}
}

// An explicit --url is used as given, with no probing of other candidates.
func TestExplicitURLSkipsResolution(t *testing.T) {
	c := NewClient("http://example.invalid:9/", 2*time.Second)
	base, err := c.Base()
	if err != nil {
		t.Fatal(err)
	}
	if base != "http://example.invalid:9" {
		t.Errorf("base = %q, want the pinned URL without its trailing slash", base)
	}
	if c.Source() != "--url" {
		t.Errorf("source = %q, want --url", c.Source())
	}
}
