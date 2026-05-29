package handler

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

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
