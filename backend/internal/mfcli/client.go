package mfcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultRemote is the home server. Override with MEMORY_FLOW_REMOTE.
const DefaultRemote = "https://memory-flow.local.playquota.com"

// LocalDefault is the port the standalone binary serves on by default.
const LocalDefault = "http://127.0.0.1:8080"

const (
	probeTimeout = 2 * time.Second
	// cacheTTL bounds how long a resolved base URL is reused without probing.
	// Short enough that moving on/off the home network self-corrects quickly,
	// long enough that a burst of commands pays the probe cost only once.
	cacheTTL = 90 * time.Second
)

// Client talks to a Memory Flow API instance, resolving which instance to use
// on first request. See [Client.Resolve] for the resolution order.
type Client struct {
	// Explicit pins the base URL (from --url); resolution is skipped.
	Explicit string
	Timeout  time.Duration
	// NoCache skips reading the cached endpoint, forcing a fresh probe.
	NoCache bool

	base   string
	http   *http.Client
	source string // how base was determined, for `mf endpoint`
}

func NewClient(explicit string, timeout time.Duration) *Client {
	return &Client{
		Explicit: explicit,
		Timeout:  timeout,
		http:     &http.Client{Timeout: timeout, Transport: newTransport()},
	}
}

// Base returns the resolved base URL, resolving on first use.
func (c *Client) Base() (string, error) {
	if c.base != "" {
		return c.base, nil
	}
	if err := c.Resolve(); err != nil {
		return "", err
	}
	return c.base, nil
}

// Source describes where the base URL came from ("--url", "remote", "cache", …).
func (c *Client) Source() string { return c.source }

// Resolve picks the instance to talk to, preferring the remote home server and
// falling back to a local standalone app:
//
//  1. --url / MEMORY_FLOW_URL  (pinned, never probed)
//  2. a fresh cached result from a previous run
//  3. the remote home server, if it answers
//  4. ~/.memory_flow/endpoint, the port the running standalone advertises
//  5. http://127.0.0.1:8080, the standalone's default
func (c *Client) Resolve() error {
	if c.Explicit != "" {
		c.base, c.source = strings.TrimRight(c.Explicit, "/"), "--url"
		return nil
	}
	if env := os.Getenv("MEMORY_FLOW_URL"); env != "" {
		c.base, c.source = strings.TrimRight(env, "/"), "MEMORY_FLOW_URL"
		return nil
	}
	if !c.NoCache {
		if cached, ok := readEndpointCache(); ok {
			c.base, c.source = cached, "cache"
			return nil
		}
	}

	remote := os.Getenv("MEMORY_FLOW_REMOTE")
	if remote == "" {
		remote = DefaultRemote
	}
	candidates := []struct{ url, source string }{
		{remote, "remote"},
		{readEndpointFile(), "endpoint file"},
		{LocalDefault, "local default"},
	}
	for _, cand := range candidates {
		if cand.url == "" {
			continue
		}
		if probe(cand.url) {
			c.base, c.source = strings.TrimRight(cand.url, "/"), cand.source
			writeEndpointCache(c.base)
			return nil
		}
	}
	return fmt.Errorf("Memory Flow is unreachable: the remote (%s) did not answer and no local instance is running.\n"+
		"Start the local app (open \"/Applications/Memory Flow.app\", or run 'memory_flow serve') and retry, or pass --url", remote)
}

// probe reports whether a Memory Flow API is answering at base.
func probe(base string) bool {
	ok, err := probeOnce(base)
	// A first contact that fails TLS verification may just mean the platform's
	// trust store is unreachable; retry against a CA bundle before concluding
	// the instance is down. See [enableTLSFallback].
	if err != nil && isTLSVerifyError(err) && enableTLSFallback() {
		ok, _ = probeOnce(base)
	}
	return ok
}

func probeOnce(base string) (bool, error) {
	client := &http.Client{Timeout: probeTimeout, Transport: newTransport()}
	resp, err := client.Get(strings.TrimRight(base, "/") + "/api/v1/projects?page_size=1")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<10))
	return resp.StatusCode == http.StatusOK, nil
}

// payload is a request body already encoded, with the content type to send it
// under. Assets travel as raw bytes rather than JSON, so the body cannot always
// be marshalled from a Go value.
type payload struct {
	data        []byte
	contentType string
}

func jsonPayload(v any) (*payload, error) {
	encoded, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	return &payload{data: encoded, contentType: "application/json"}, nil
}

// request performs an API call and returns the raw response body. On a
// connection-level failure it drops the cached endpoint and re-resolves once,
// so a stale cache (e.g. after leaving the home network) self-heals instead of
// surfacing as an error.
func (c *Client) request(method, path string, body *payload) (json.RawMessage, error) {
	raw, err := c.attempt(method, path, body)
	if err == nil {
		return raw, nil
	}

	// Retry once against a CA bundle if the platform verifier is unusable.
	if isTLSVerifyError(err) && enableTLSFallback() {
		c.http.Transport = newTransport()
		if raw, tlsErr := c.attempt(method, path, body); tlsErr == nil {
			return raw, nil
		}
	}

	var netErr *connError
	if !asConnError(err, &netErr) || c.source != "cache" {
		return nil, err
	}
	clearEndpointCache()
	c.base, c.NoCache = "", true
	if rerr := c.Resolve(); rerr != nil {
		return nil, rerr
	}
	return c.attempt(method, path, body)
}

func (c *Client) attempt(method, path string, body *payload) (json.RawMessage, error) {
	base, err := c.Base()
	if err != nil {
		return nil, err
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body.data)
	}

	req, err := http.NewRequest(method, base+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", body.contentType)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &connError{err}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, &connError{err}
	}
	if resp.StatusCode >= 300 {
		return nil, apiError(resp.StatusCode, method, path, raw)
	}
	return raw, nil
}

// maxDownloadBytes bounds an asset download. The server caps uploads far below
// this; the limit is only here so a misbehaving endpoint cannot exhaust memory.
const maxDownloadBytes = 256 << 20

// download fetches raw bytes — an asset's content — rather than a JSON
// envelope, returning the body and its content type.
func (c *Client) download(path string) ([]byte, string, error) {
	base, err := c.Base()
	if err != nil {
		return nil, "", err
	}

	do := func() (*http.Response, error) {
		req, err := http.NewRequest(http.MethodGet, base+path, nil)
		if err != nil {
			return nil, err
		}
		return c.http.Do(req)
	}

	resp, err := do()
	if err != nil && isTLSVerifyError(err) && enableTLSFallback() {
		c.http.Transport = newTransport()
		resp, err = do()
	}
	if err != nil {
		return nil, "", &connError{err}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadBytes))
	if err != nil {
		return nil, "", &connError{err}
	}
	if resp.StatusCode >= 300 {
		return nil, "", apiError(resp.StatusCode, http.MethodGet, path, raw)
	}
	return raw, resp.Header.Get("Content-Type"), nil
}

// uploadBytes sends raw content under an explicit content type — how assets are
// created and replaced.
func (c *Client) uploadBytes(method, path string, content []byte, contentType string, out any) (json.RawMessage, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	raw, err := c.request(method, path, &payload{data: content, contentType: contentType})
	if err != nil {
		return nil, err
	}
	return decodeInto(raw, out)
}

// connError marks a transport-level failure, which is retryable against a
// freshly resolved endpoint (unlike an HTTP error, which is the server's answer).
type connError struct{ err error }

func (e *connError) Error() string { return e.err.Error() }
func (e *connError) Unwrap() error { return e.err }

func asConnError(err error, target **connError) bool {
	if ce, ok := err.(*connError); ok {
		*target = ce
		return true
	}
	return false
}

// apiError turns a non-2xx response into a readable error, unwrapping the
// API's {"error": "..."} envelope when present.
func apiError(status int, method, path string, raw []byte) error {
	var env struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && env.Error != "" {
		return fmt.Errorf("%s %s: %s (HTTP %d)", method, path, env.Error, status)
	}
	msg := strings.TrimSpace(string(raw))
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	return fmt.Errorf("%s %s: %s (HTTP %d)", method, path, msg, status)
}

// get/post/put/patch/del decode the `data` field of the response envelope into
// out (when out is non-nil) and return the full raw body for --json output.

func (c *Client) get(path string, out any) (json.RawMessage, error) {
	return c.call(http.MethodGet, path, nil, out)
}

func (c *Client) post(path string, body, out any) (json.RawMessage, error) {
	return c.call(http.MethodPost, path, body, out)
}

func (c *Client) put(path string, body, out any) (json.RawMessage, error) {
	return c.call(http.MethodPut, path, body, out)
}

func (c *Client) patch(path string, body, out any) (json.RawMessage, error) {
	return c.call(http.MethodPatch, path, body, out)
}

func (c *Client) del(path string) (json.RawMessage, error) {
	return c.call(http.MethodDelete, path, nil, nil)
}

func (c *Client) call(method, path string, body, out any) (json.RawMessage, error) {
	var encoded *payload
	if body != nil {
		var err error
		if encoded, err = jsonPayload(body); err != nil {
			return nil, err
		}
	}
	raw, err := c.request(method, path, encoded)
	if err != nil {
		return nil, err
	}
	return decodeInto(raw, out)
}

// decodeInto unwraps the {"data": …} envelope into out, returning the full raw
// body so --json can print it verbatim.
func decodeInto(raw json.RawMessage, out any) (json.RawMessage, error) {
	if out == nil {
		return raw, nil
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return raw, fmt.Errorf("decode response: %w", err)
	}
	if len(env.Data) == 0 {
		return raw, nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return raw, fmt.Errorf("decode response data: %w", err)
	}
	return raw, nil
}

// listTotal reads the `total` field of a list envelope.
func listTotal(raw json.RawMessage) int {
	var env struct {
		Total int `json:"total"`
	}
	json.Unmarshal(raw, &env)
	return env.Total
}

// query builds a query string from non-empty pairs.
func query(pairs map[string]string) string {
	values := url.Values{}
	for k, v := range pairs {
		if v != "" {
			values.Set(k, v)
		}
	}
	if len(values) == 0 {
		return ""
	}
	return "?" + values.Encode()
}

// --- endpoint file & cache -------------------------------------------------

func memoryFlowDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".memory_flow")
}

// readEndpointFile returns the URL the running standalone advertises. The
// standalone writes it on startup and removes it on graceful shutdown, so a
// stale file simply fails the probe.
func readEndpointFile() string {
	dir := memoryFlowDir()
	if dir == "" {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(dir, "endpoint"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func cachePath() string {
	dir := memoryFlowDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "cli-endpoint.json")
}

type endpointCache struct {
	URL string `json:"url"`
	At  int64  `json:"at"`
}

func readEndpointCache() (string, bool) {
	p := cachePath()
	if p == "" {
		return "", false
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	var c endpointCache
	if err := json.Unmarshal(raw, &c); err != nil || c.URL == "" {
		return "", false
	}
	if time.Since(time.Unix(c.At, 0)) > cacheTTL {
		return "", false
	}
	return c.URL, true
}

func writeEndpointCache(base string) {
	p := cachePath()
	if p == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	raw, err := json.Marshal(endpointCache{URL: base, At: time.Now().Unix()})
	if err != nil {
		return
	}
	os.WriteFile(p, raw, 0o644)
}

func clearEndpointCache() {
	if p := cachePath(); p != "" {
		os.Remove(p)
	}
}
