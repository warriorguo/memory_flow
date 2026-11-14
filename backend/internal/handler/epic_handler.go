package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/middleware"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/service"
)

// EpicHandler serves epics (MF-39): the per-project epic list, the epic view
// and progress, epic membership, and the memories an issue inherits from its
// epic.
type EpicHandler struct {
	svc      *service.EpicService
	resolver *IDResolver
}

func NewEpicHandler(svc *service.EpicService, resolver *IDResolver) *EpicHandler {
	return &EpicHandler{svc: svc, resolver: resolver}
}

// conflictResponse is the 409 body when a memory would restate another.
type conflictResponse struct {
	Error     string                 `json:"error"`
	Conflicts []model.MemoryConflict `json:"conflicts"`
}

// writeMemoryError maps a memory write failure to its response.
func writeMemoryError(w http.ResponseWriter, err error) {
	var ce *service.ConflictError
	if errors.As(err, &ce) {
		writeJSON(w, http.StatusConflict, conflictResponse{Error: ce.Error(), Conflicts: ce.Conflicts})
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

func (h *EpicHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID, err := h.resolver.ResolveProjectID(r.Context(), chi.URLParam(r, "projectId"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	epics, err := h.svc.ListEpics(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if epics == nil {
		epics = []model.EpicSummary{}
	}
	writeData(w, http.StatusOK, epics)
}

func (h *EpicHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolver.ResolveIssueID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	detail, err := h.svc.Detail(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeData(w, http.StatusOK, detail)
}

func (h *EpicHandler) Progress(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolver.ResolveIssueID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	p, err := h.svc.Progress(r.Context(), id, queryInt(r, "days", 30))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeData(w, http.StatusOK, p)
}

// SetEpic is PUT /issues/{id}/epic {"epic": "OZX-1642"}; an empty epic takes
// the issue out of its epic.
func (h *EpicHandler) SetEpic(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolver.ResolveIssueID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var req model.SetEpicRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, `body must be {"epic": "<epic key or id, or empty to remove>"}`)
		return
	}
	var epicID *uuid.UUID
	if ref := strings.TrimSpace(req.Epic); ref != "" {
		eid, err := h.resolver.ResolveIssueID(r.Context(), ref)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		epicID = &eid
	}
	var operator *string
	if userID := middleware.GetUserID(r.Context()); userID != "" {
		operator = &userID
	}
	res, err := h.svc.SetEpic(r.Context(), id, epicID, operator)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.FillEpicRef(r.Context(), res.Issue); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeData(w, http.StatusOK, res)
}

// IssueMemories is GET /issues/{id}/memories?reader=R: the issue's own
// memories plus its epic's, each marked with where it comes from.
func (h *EpicHandler) IssueMemories(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolver.ResolveIssueID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	reader := r.URL.Query().Get("reader")
	ms, err := h.svc.IssueMemories(r.Context(), id, reader)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeData(w, http.StatusOK, ms)
}

// MarkMemoriesRead is POST /memories/read.
func (h *EpicHandler) MarkMemoriesRead(w http.ResponseWriter, r *http.Request) {
	var req model.MarkMemoriesReadRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	n, err := h.svc.MarkMemoriesRead(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]int{"marked": n})
}

// Promote is POST /memories/{id}/promote {"copy": false, "force": false}.
func (h *EpicHandler) Promote(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid memory id")
		return
	}
	var req model.PromoteMemoryRequest
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	m, epic, err := h.svc.Promote(r.Context(), id, req)
	if err != nil {
		writeMemoryError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"memory": m, "epic": epic.IssueKey, "copied": req.Copy})
}
