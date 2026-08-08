package handler

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/synccore"
)

// maxSnapshotBytes bounds the import body. Snapshots are the full dataset, so
// this is far larger than the default 1MB request cap.
const maxSnapshotBytes = 256 << 20 // 256 MB

// SyncTokenHeader carries the shared secret that authorizes sync requests.
const SyncTokenHeader = "X-Sync-Token"

// SyncHandler exposes full-dataset export/import for data transfer between a
// standalone instance and the server. It is mounted on both.
//
// When token is non-empty, every request must present a matching token in the
// X-Sync-Token header; otherwise the endpoints are open (e.g. a localhost-only
// standalone with no secret configured).
type SyncHandler struct {
	db    database.DB
	token string
}

func NewSyncHandler(db database.DB, token string) *SyncHandler {
	return &SyncHandler{db: db, token: token}
}

// authorized reports whether the request may use the sync endpoints, writing a
// 401 if not.
func (h *SyncHandler) authorized(w http.ResponseWriter, r *http.Request) bool {
	if h.token == "" {
		return true
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get(SyncTokenHeader)), []byte(h.token)) == 1 {
		return true
	}
	writeError(w, http.StatusUnauthorized, "invalid or missing sync token")
	return false
}

// Export returns a snapshot of every table.
func (h *SyncHandler) Export(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	snap, err := synccore.Export(r.Context(), h.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// AssetContent serves an asset's bytes to a syncing peer, keyed by asset id
// rather than issue+filename so the transfer does not depend on either side's
// naming being current.
func (h *SyncHandler) AssetContent(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid asset id")
		return
	}

	content, checksum, err := synccore.AssetContent(r.Context(), h.db, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if content == nil {
		// Either the asset is unknown here, or this instance is itself waiting
		// for the bytes. Neither is something the peer can fix by retrying now.
		writeError(w, http.StatusNotFound, "asset content is not available on this instance")
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("ETag", `"`+checksum+`"`)
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

// PutAssetContent accepts bytes for an asset whose metadata already arrived.
// The checksum must match what the metadata claims — see
// [synccore.PutAssetContent].
func (h *SyncHandler) PutAssetContent(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid asset id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxSnapshotBytes)
	content, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "asset content too large")
		return
	}
	if err := synccore.PutAssetContent(r.Context(), h.db, id, content); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]string{"status": "stored", "id": id.String()})
}

// Import merges a posted snapshot into this instance (last-writer-wins).
func (h *SyncHandler) Import(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxSnapshotBytes)
	var snap synccore.Snapshot
	if err := json.NewDecoder(r.Body).Decode(&snap); err != nil {
		writeError(w, http.StatusBadRequest, "invalid snapshot: "+err.Error())
		return
	}
	res, err := synccore.Import(r.Context(), h.db, &snap)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}
