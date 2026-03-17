package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/service"
)

type MemoryHandler struct {
	svc *service.MemoryService
}

func NewMemoryHandler(svc *service.MemoryService) *MemoryHandler {
	return &MemoryHandler{svc: svc}
}

func (h *MemoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateMemoryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	memory, err := h.svc.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeData(w, http.StatusCreated, memory)
}

func (h *MemoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid memory id")
		return
	}

	memory, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if memory == nil {
		writeError(w, http.StatusNotFound, "memory not found")
		return
	}

	writeData(w, http.StatusOK, memory)
}

func (h *MemoryHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := model.MemoryFilter{
		ProjectID:        queryUUID(r, "project_id"),
		Type:             queryString(r, "type"),
		Keyword:          queryString(r, "keyword"),
		SourceObjectType: queryString(r, "source_object_type"),
		SourceObjectID:   queryUUID(r, "source_object_id"),
		Page:             queryInt(r, "page", 1),
		PageSize:         queryInt(r, "page_size", 20),
	}

	memories, total, err := h.svc.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if memories == nil {
		memories = []model.Memory{}
	}

	writeList(w, memories, total, filter.Page, filter.PageSize)
}

func (h *MemoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid memory id")
		return
	}

	var req model.UpdateMemoryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	memory, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if memory == nil {
		writeError(w, http.StatusNotFound, "memory not found")
		return
	}

	writeData(w, http.StatusOK, memory)
}

func (h *MemoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid memory id")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeData(w, http.StatusOK, map[string]string{"status": "deleted"})
}
