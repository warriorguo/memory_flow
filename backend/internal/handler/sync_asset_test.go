package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	"github.com/warriorguo/memory_flow/backend/internal/synccore"
	sqlitemigrations "github.com/warriorguo/memory_flow/backend/migrations_sqlite"
)

// The sync blob endpoints are the only part of asset sync that crosses the
// wire, so they get a real database rather than a mock.
func newSyncTestDB(t *testing.T) database.DB {
	t.Helper()
	raw, err := database.OpenSQLiteDB(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateSQLite(raw, sqlitemigrations.FS); err != nil {
		t.Fatal(err)
	}
	db := database.WrapSQLite(raw)
	t.Cleanup(db.Close)
	return db
}

func seedAsset(t *testing.T, db database.DB, content []byte) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	proj, err := repository.NewProjectRepo(db).Create(ctx, model.CreateProjectRequest{Key: "OZX", Name: "OZX"})
	if err != nil {
		t.Fatal(err)
	}
	issueRepo := repository.NewIssueRepo(db)
	tx, err := issueRepo.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	issue, err := issueRepo.Create(ctx, tx, "OZX-1", proj.ID, model.CreateIssueRequest{Type: "bug", Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	asset, err := repository.NewAssetRepo(db, repository.NewDBContentStore()).Create(ctx, issue.ID,
		model.PutAssetRequest{Filename: "clip.mp4", MimeType: "video/mp4", Content: content})
	if err != nil {
		t.Fatal(err)
	}
	return asset.ID
}

func syncRouter(db database.DB, token string) chi.Router {
	h := NewSyncHandler(db, token)
	r := chi.NewRouter()
	r.Get("/sync/assets/{id}", h.AssetContent)
	r.Put("/sync/assets/{id}", h.PutAssetContent)
	return r
}

func TestSyncAssetContentRoundTrip(t *testing.T) {
	source := newSyncTestDB(t)
	content := []byte{0x00, 0x01, 0xfe, 0xff, 'v', 'i', 'd'}
	assetID := seedAsset(t, source, content)

	// Download from the instance that holds the bytes.
	w := httptest.NewRecorder()
	syncRouter(source, "").ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sync/assets/"+assetID.String(), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET status = %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), content) {
		t.Errorf("body = %v, want %v", w.Body.Bytes(), content)
	}

	// A peer that merged the metadata but has no bytes yet.
	peer := newSyncTestDB(t)
	snap, err := synccore.Export(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := synccore.Import(context.Background(), peer, snap); err != nil {
		t.Fatal(err)
	}

	peerRouter := syncRouter(peer, "")
	w = httptest.NewRecorder()
	peerRouter.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sync/assets/"+assetID.String(), nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("peer without bytes: status = %d, want 404", w.Code)
	}

	// Upload the bytes to the peer.
	w = httptest.NewRecorder()
	peerRouter.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/sync/assets/"+assetID.String(), bytes.NewReader(content)))
	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	peerRouter.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sync/assets/"+assetID.String(), nil))
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), content) {
		t.Errorf("after upload: status %d body %v", w.Code, w.Body.Bytes())
	}
}

func TestSyncAssetContentRejectsMismatchAndUnauthorized(t *testing.T) {
	db := newSyncTestDB(t)
	assetID := seedAsset(t, db, []byte("right"))

	// Content that does not match the recorded checksum.
	w := httptest.NewRecorder()
	syncRouter(db, "").ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/sync/assets/"+assetID.String(), bytes.NewReader([]byte("wrong"))))
	if w.Code != http.StatusBadRequest {
		t.Errorf("mismatched upload: status = %d, want 400", w.Code)
	}

	// The blob endpoints sit behind the same shared secret as export/import.
	guarded := syncRouter(db, "s3cret")
	w = httptest.NewRecorder()
	guarded.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sync/assets/"+assetID.String(), nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("no token: status = %d, want 401", w.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/sync/assets/"+assetID.String(), nil)
	req.Header.Set(SyncTokenHeader, "s3cret")
	w = httptest.NewRecorder()
	guarded.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("with token: status = %d, want 200", w.Code)
	}
}
