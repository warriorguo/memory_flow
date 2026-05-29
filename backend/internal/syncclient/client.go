// Package syncclient drives data sync between a local database and a remote
// Memory Flow instance over its HTTP sync API.
package syncclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/synccore"
)

const (
	exportPath = "/api/v1/sync/export"
	importPath = "/api/v1/sync/import"
)

var httpClient = &http.Client{Timeout: 5 * time.Minute}

// Push exports the local dataset and merges it into the remote server.
// Returns the server's merge result. token, when non-empty, is sent as the
// X-Sync-Token header to satisfy the server's shared-secret guard.
func Push(ctx context.Context, db database.DB, serverURL, token string) (*synccore.ImportResult, error) {
	snap, err := synccore.Export(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("export local: %w", err)
	}

	body, err := json.Marshal(snap)
	if err != nil {
		return nil, fmt.Errorf("marshal snapshot: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL(serverURL)+importPath, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	setToken(req, token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("post to server: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, remoteError("import", resp)
	}

	var res synccore.ImportResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("decode server result: %w", err)
	}
	return &res, nil
}

// Pull fetches the remote dataset and merges it into the local database.
// Returns the local merge result. token, when non-empty, is sent as the
// X-Sync-Token header to satisfy the server's shared-secret guard.
func Pull(ctx context.Context, db database.DB, serverURL, token string) (*synccore.ImportResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL(serverURL)+exportPath, nil)
	if err != nil {
		return nil, err
	}
	setToken(req, token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get from server: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, remoteError("export", resp)
	}

	var snap synccore.Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		return nil, fmt.Errorf("decode server snapshot: %w", err)
	}

	res, err := synccore.Import(ctx, db, &snap)
	if err != nil {
		return nil, fmt.Errorf("import locally: %w", err)
	}
	return res, nil
}

func baseURL(s string) string { return strings.TrimRight(s, "/") }

// syncTokenHeader mirrors handler.SyncTokenHeader (duplicated to avoid importing
// the handler package from the client).
const syncTokenHeader = "X-Sync-Token"

func setToken(req *http.Request, token string) {
	if token != "" {
		req.Header.Set(syncTokenHeader, token)
	}
}

func remoteError(op string, resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("remote %s failed: %s: %s", op, resp.Status, strings.TrimSpace(string(b)))
}
