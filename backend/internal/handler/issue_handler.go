package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/warriorguo/memory_flow/backend/internal/middleware"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	"github.com/warriorguo/memory_flow/backend/internal/service"
)

type IssueHandler struct {
	svc      *service.IssueService
	tagRepo  repository.TagRepository
	resolver *IDResolver
}

func NewIssueHandler(svc *service.IssueService, tagRepo repository.TagRepository, resolver *IDResolver) *IssueHandler {
	return &IssueHandler{svc: svc, tagRepo: tagRepo, resolver: resolver}
}

func (h *IssueHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID, err := h.resolver.ResolveProjectID(r.Context(), chi.URLParam(r, "projectId"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req model.CreateIssueRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Set creator from auth context if not provided
	if req.CreatorID == nil {
		userID := middleware.GetUserID(r.Context())
		if userID != "" {
			req.CreatorID = &userID
		}
	}

	issue, err := h.svc.Create(r.Context(), projectID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeData(w, http.StatusCreated, issue)
}

func (h *IssueHandler) Search(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'key' is required")
		return
	}

	issue, err := h.svc.GetByKey(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if issue == nil {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}

	tags, err := h.tagRepo.GetByIssueID(r.Context(), issue.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	issue.Tags = tags

	writeData(w, http.StatusOK, issue)
}

func (h *IssueHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolver.ResolveIssueID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	issue, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if issue == nil {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}

	tags, err := h.tagRepo.GetByIssueID(r.Context(), issue.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	issue.Tags = tags

	writeData(w, http.StatusOK, issue)
}

func (h *IssueHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := h.resolver.ResolveProjectID(r.Context(), chi.URLParam(r, "projectId"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	filter := model.IssueFilter{
		ProjectID:  &projectID,
		Type:       queryString(r, "type"),
		Status:     queryString(r, "status"),
		Priority:   queryString(r, "priority"),
		AssigneeID: queryString(r, "assignee_id"),
		CreatorID:  queryString(r, "creator_id"),
		Keyword:    queryString(r, "keyword"),
		TagID:      queryString(r, "tag"),
		Page:       queryInt(r, "page", 1),
		PageSize:   queryInt(r, "page_size", 20),
	}

	issues, total, err := h.svc.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if issues == nil {
		issues = []model.Issue{}
	}

	writeList(w, issues, total, filter.Page, filter.PageSize)
}

func (h *IssueHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolver.ResolveIssueID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req model.UpdateIssueRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := middleware.GetUserID(r.Context())
	var operatorID *string
	if userID != "" {
		operatorID = &userID
	}

	issue, err := h.svc.Update(r.Context(), id, req, operatorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeData(w, http.StatusOK, issue)
}

func (h *IssueHandler) TransitionStatus(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolver.ResolveIssueID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req model.TransitionStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Status == "" {
		writeError(w, http.StatusBadRequest, "status is required")
		return
	}

	userID := middleware.GetUserID(r.Context())
	var operatorID *string
	if userID != "" {
		operatorID = &userID
	}

	issue, err := h.svc.TransitionStatus(r.Context(), id, req.Status, operatorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeData(w, http.StatusOK, issue)
}

func (h *IssueHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolver.ResolveIssueID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	history, err := h.svc.GetHistory(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if history == nil {
		history = []model.IssueHistory{}
	}

	writeData(w, http.StatusOK, history)
}
