package mfcli

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These tests cover the resolution chain that replaced the shell resolver block
// the skill used to paste: --url / $MEMORY_FLOW_URL, then the remote home
// server, then the local standalone's advertised endpoint, then its default
// port — plus the cache and the retry that heals a stale one.

// isolateHome points $HOME at a scratch directory so the endpoint file and the
// resolution cache cannot see (or disturb) the real ones.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MEMORY_FLOW_URL", "")
	t.Setenv("MEMORY_FLOW_REMOTE", "")
	if err := os.MkdirAll(filepath.Join(home, ".memory_flow"), 0o755); err != nil {
		t.Fatal(err)
	}
	return home
}

// apiStub serves just enough for the resolver's probe and a `projects` call.
func apiStub(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[],"total":0,"page":1,"page_size":1}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// deadURL returns a URL that nothing is listening on.
func deadURL(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	return "http://" + addr
}

func writeEndpointFileAt(t *testing.T, home, url string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(home, ".memory_flow", "endpoint"), []byte(url+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readCache(t *testing.T, home string) (endpointCache, bool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(home, ".memory_flow", "cli-endpoint.json"))
	if err != nil {
		return endpointCache{}, false
	}
	var c endpointCache
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("cache is not valid JSON: %v", err)
	}
	return c, true
}

func TestResolveExplicitURLWinsOverEnv(t *testing.T) {
	isolateHome(t)
	t.Setenv("MEMORY_FLOW_URL", "http://from-env:1")

	c := NewClient("http://from-flag:2", time.Second)
	base, err := c.Base()
	if err != nil {
		t.Fatal(err)
	}
	if base != "http://from-flag:2" || c.Source() != "--url" {
		t.Errorf("base = %q (%s), want the --url value", base, c.Source())
	}
}

func TestResolveUsesEnvWhenNoFlag(t *testing.T) {
	isolateHome(t)
	t.Setenv("MEMORY_FLOW_URL", "http://from-env:1/")

	c := NewClient("", time.Second)
	base, err := c.Base()
	if err != nil {
		t.Fatal(err)
	}
	if base != "http://from-env:1" {
		t.Errorf("base = %q, want the env value with its trailing slash trimmed", base)
	}
	if c.Source() != "MEMORY_FLOW_URL" {
		t.Errorf("source = %q", c.Source())
	}
}

// The remote home server is preferred whenever it answers, even if a local
// instance is also running — the local one may be behind until synced.
func TestResolvePrefersRemoteOverLocal(t *testing.T) {
	home := isolateHome(t)
	remote := apiStub(t)
	local := apiStub(t)
	t.Setenv("MEMORY_FLOW_REMOTE", remote.URL)
	writeEndpointFileAt(t, home, local.URL)

	c := NewClient("", time.Second)
	base, err := c.Base()
	if err != nil {
		t.Fatal(err)
	}
	if base != remote.URL {
		t.Errorf("base = %q, want the remote %q", base, remote.URL)
	}
	if c.Source() != "remote" {
		t.Errorf("source = %q, want remote", c.Source())
	}
}

// With the remote down, the standalone's advertised endpoint takes over — the
// case the whole fallback exists for.
func TestResolveFallsBackToEndpointFile(t *testing.T) {
	home := isolateHome(t)
	local := apiStub(t)
	t.Setenv("MEMORY_FLOW_REMOTE", deadURL(t))
	writeEndpointFileAt(t, home, local.URL)

	c := NewClient("", time.Second)
	base, err := c.Base()
	if err != nil {
		t.Fatal(err)
	}
	if base != local.URL {
		t.Errorf("base = %q, want the local endpoint %q", base, local.URL)
	}
	if c.Source() != "endpoint file" {
		t.Errorf("source = %q, want 'endpoint file'", c.Source())
	}
}

// A stale endpoint file — the standalone exited without cleaning up — must be
// skipped rather than used, because the resolver health-checks it.
func TestResolveSkipsStaleEndpointFile(t *testing.T) {
	home := isolateHome(t)
	remote := apiStub(t)
	t.Setenv("MEMORY_FLOW_REMOTE", remote.URL)
	writeEndpointFileAt(t, home, deadURL(t))

	c := NewClient("", time.Second)
	base, err := c.Base()
	if err != nil {
		t.Fatal(err)
	}
	if base != remote.URL {
		t.Errorf("base = %q, want the remote — the stale endpoint file should be ignored", base)
	}
}

func TestResolveFailsWithGuidanceWhenNothingAnswers(t *testing.T) {
	isolateHome(t)
	t.Setenv("MEMORY_FLOW_REMOTE", deadURL(t))
	// The last candidate is the standalone's fixed default port; if something
	// is genuinely listening there, this scenario cannot be reproduced.
	if probe(LocalDefault) {
		t.Skip("a Memory Flow instance is running on " + LocalDefault)
	}

	c := NewClient("", time.Second)
	_, err := c.Base()
	if err == nil {
		t.Fatal("resolution should fail when nothing answers")
	}
	for _, want := range []string{"unreachable", "Memory Flow.app", "--url"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q: %v", want, err)
		}
	}
}

// A successful resolution is cached so a burst of commands probes once.
func TestResolveCachesAndReuses(t *testing.T) {
	home := isolateHome(t)
	remote := apiStub(t)
	t.Setenv("MEMORY_FLOW_REMOTE", remote.URL)

	first := NewClient("", time.Second)
	if _, err := first.Base(); err != nil {
		t.Fatal(err)
	}
	cached, ok := readCache(t, home)
	if !ok || cached.URL != remote.URL {
		t.Fatalf("cache = %+v, want %q", cached, remote.URL)
	}

	// A second client reads the cache instead of probing — provable by pointing
	// the remote somewhere dead: only the cache can still produce a URL.
	t.Setenv("MEMORY_FLOW_REMOTE", deadURL(t))
	second := NewClient("", time.Second)
	base, err := second.Base()
	if err != nil {
		t.Fatal(err)
	}
	if base != remote.URL || second.Source() != "cache" {
		t.Errorf("base = %q (%s), want the cached %q", base, second.Source(), remote.URL)
	}
}

func TestResolveIgnoresExpiredCache(t *testing.T) {
	home := isolateHome(t)
	remote := apiStub(t)
	t.Setenv("MEMORY_FLOW_REMOTE", remote.URL)

	stale, _ := json.Marshal(endpointCache{
		URL: "http://stale.invalid:1",
		At:  time.Now().Add(-2 * cacheTTL).Unix(),
	})
	if err := os.WriteFile(filepath.Join(home, ".memory_flow", "cli-endpoint.json"), stale, 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewClient("", time.Second)
	base, err := c.Base()
	if err != nil {
		t.Fatal(err)
	}
	if base != remote.URL {
		t.Errorf("base = %q, want a fresh probe result once the cache expired", base)
	}
}

// --refresh forces a probe even when the cache is fresh.
func TestRefreshBypassesCache(t *testing.T) {
	home := isolateHome(t)
	remote := apiStub(t)
	fresh, _ := json.Marshal(endpointCache{URL: "http://stale.invalid:1", At: time.Now().Unix()})
	if err := os.WriteFile(filepath.Join(home, ".memory_flow", "cli-endpoint.json"), fresh, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MEMORY_FLOW_REMOTE", remote.URL)

	c := NewClient("", time.Second)
	c.NoCache = true
	base, err := c.Base()
	if err != nil {
		t.Fatal(err)
	}
	if base != remote.URL {
		t.Errorf("base = %q, want --refresh to ignore the cached %q", base, "http://stale.invalid:1")
	}
}

// The important recovery path: a cached endpoint that has since gone away (the
// laptop left the home network) must re-resolve and retry rather than error.
func TestStaleCacheReresolvesAndRetries(t *testing.T) {
	home := isolateHome(t)
	live := apiStub(t)
	t.Setenv("MEMORY_FLOW_REMOTE", live.URL)

	dead := deadURL(t)
	cached, _ := json.Marshal(endpointCache{URL: dead, At: time.Now().Unix()})
	if err := os.WriteFile(filepath.Join(home, ".memory_flow", "cli-endpoint.json"), cached, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr strings.Builder
	if code := Main([]string{"projects"}, &stdout, &stderr); code != 0 {
		t.Fatalf("a dead cached endpoint should self-heal, got exit %d: %s", code, stderr.String())
	}

	updated, ok := readCache(t, home)
	if !ok || updated.URL != live.URL {
		t.Errorf("cache = %+v, want it rewritten to %q", updated, live.URL)
	}
}

// An HTTP error is the server's answer, not a transport failure, so it must not
// trigger the re-resolve-and-retry path.
func TestHTTPErrorDoesNotRetry(t *testing.T) {
	isolateHome(t)
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"boom"}`))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, time.Second)
	if _, err := c.get("/api/v1/projects", nil); err == nil {
		t.Fatal("a 500 should be an error")
	} else if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error should carry the API message: %v", err)
	}
	if hits != 1 {
		t.Errorf("request count = %d, want 1 (no retry on an HTTP error)", hits)
	}
}

func TestQueryOmitsEmptyValues(t *testing.T) {
	if got := query(map[string]string{"a": "", "b": ""}); got != "" {
		t.Errorf("query with only empty values = %q, want empty", got)
	}
	got := query(map[string]string{"status": "todo", "type": ""})
	if got != "?status=todo" {
		t.Errorf("query = %q, want ?status=todo", got)
	}
}
