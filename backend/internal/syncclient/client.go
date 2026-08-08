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

	"github.com/google/uuid"
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

	// The metadata merge tells us which blobs the server is now missing; send
	// those and nothing else, so an unchanged attachment never crosses the wire
	// twice.
	pushAssets(ctx, db, serverURL, token, &res)
	return &res, nil
}

// maxSyncTransferBytes bounds one sync's blob traffic. Anything left over stays
// marked pending and moves on the next sync, so a pile of large attachments
// cannot turn a routine sync into an unbounded transfer.
const maxSyncTransferBytes = 512 << 20

// pushAssets uploads the bytes for every asset the server reported pending.
// Failures are recorded rather than fatal: the row stays pending on the server
// and the next sync retries it.
func pushAssets(ctx context.Context, db database.DB, serverURL, token string, res *synccore.ImportResult) {
	budget := int64(maxSyncTransferBytes)

	for _, id := range res.AssetsPending {
		content, _, err := synccore.AssetContent(ctx, db, id)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("read asset %s: %v", id, err))
			continue
		}
		if content == nil {
			// We do not have these bytes either — the server will keep asking, and
			// whichever side does have them will supply them.
			continue
		}
		if int64(len(content)) > budget {
			res.Errors = append(res.Errors, fmt.Sprintf(
				"stopped after %d bytes of asset transfer; %s and any remaining assets will move on the next sync",
				maxSyncTransferBytes-budget, id))
			return
		}

		if err := putAssetContent(ctx, serverURL, token, id, content); err != nil {
			res.Errors = append(res.Errors, err.Error())
			continue
		}
		budget -= int64(len(content))
		res.AssetsTransferred++
	}
}

// pullAssets fetches the bytes for every asset this instance is missing after a
// metadata merge.
func pullAssets(ctx context.Context, db database.DB, serverURL, token string, res *synccore.ImportResult) {
	budget := int64(maxSyncTransferBytes)

	for _, id := range res.AssetsPending {
		content, err := getAssetContent(ctx, serverURL, token, id)
		if err != nil {
			res.Errors = append(res.Errors, err.Error())
			continue
		}
		if content == nil {
			// The server does not hold these bytes either.
			continue
		}
		if int64(len(content)) > budget {
			res.Errors = append(res.Errors, fmt.Sprintf(
				"stopped after %d bytes of asset transfer; %s and any remaining assets will move on the next sync",
				maxSyncTransferBytes-budget, id))
			return
		}

		if err := synccore.PutAssetContent(ctx, db, id, content); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("store asset %s: %v", id, err))
			continue
		}
		budget -= int64(len(content))
		res.AssetsTransferred++
	}
}

func assetPath(id uuid.UUID) string { return "/api/v1/sync/assets/" + id.String() }

func putAssetContent(ctx context.Context, serverURL, token string, id uuid.UUID, content []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, baseURL(serverURL)+assetPath(id), bytes.NewReader(content))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	setToken(req, token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload asset %s: %w", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload asset %s: %w", id, remoteError("asset upload", resp))
	}
	io.Copy(io.Discard, resp.Body)
	return nil
}

// getAssetContent returns nil content (and no error) when the remote does not
// have the bytes — a normal state, not a failure.
func getAssetContent(ctx context.Context, serverURL, token string, id uuid.UUID) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL(serverURL)+assetPath(id), nil)
	if err != nil {
		return nil, err
	}
	setToken(req, token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download asset %s: %w", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download asset %s: %w", id, remoteError("asset download", resp))
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxSyncTransferBytes))
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

	// Metadata is merged; now fetch the bytes for whatever we do not hold.
	pullAssets(ctx, db, serverURL, token, res)
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
