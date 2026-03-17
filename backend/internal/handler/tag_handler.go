package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
)

type TagHandler struct {
	repo repository.TagRepository
}

func NewTagHandler(repo repository.TagRepository) *TagHandler {
	return &TagHandler{repo: repo}
}

func (h *TagHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateTagRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "tag name is required")
		return
	}

	tag, err := h.repo.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeData(w, http.StatusCreated, tag)
}

func (h *TagHandler) List(w http.ResponseWriter, r *http.Request) {
	tags, err := h.repo.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if tags == nil {
		tags = []model.Tag{}
	}

	writeData(w, http.StatusOK, tags)
}

func (h *TagHandler) AddToIssue(w http.ResponseWriter, r *http.Request) {
	issueID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue id")
		return
	}

	var req model.AddTagRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.repo.AddToIssue(r.Context(), issueID, req.TagID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tags, err := h.repo.GetByIssueID(r.Context(), issueID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeData(w, http.StatusOK, tags)
}

func (h *TagHandler) RemoveFromIssue(w http.ResponseWriter, r *http.Request) {
	issueID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue id")
		return
	}

	tagID, err := parseUUID(chi.URLParam(r, "tagId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tag id")
		return
	}

	if err := h.repo.RemoveFromIssue(r.Context(), issueID, tagID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeData(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (h *TagHandler) AddToMemory(w http.ResponseWriter, r *http.Request) {
	memoryID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid memory id")
		return
	}

	var req model.AddTagRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.repo.AddToMemory(r.Context(), memoryID, req.TagID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tags, err := h.repo.GetByMemoryID(r.Context(), memoryID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeData(w, http.StatusOK, tags)
}

func (h *TagHandler) RemoveFromMemory(w http.ResponseWriter, r *http.Request) {
	memoryID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid memory id")
		return
	}

	tagID, err := parseUUID(chi.URLParam(r, "tagId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tag id")
		return
	}

	if err := h.repo.RemoveFromMemory(r.Context(), memoryID, tagID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeData(w, http.StatusOK, map[string]string{"status": "removed"})
}
