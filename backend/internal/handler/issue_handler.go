package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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

	// Resolve tags before the issue exists, so a bad tag id fails the whole
	// request instead of leaving an issue behind without its tags.
	tagIDs, err := h.resolveCreateTags(r, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	issue, err := h.svc.Create(r.Context(), projectID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(tagIDs) > 0 {
		for _, tagID := range tagIDs {
			if err := h.tagRepo.AddToIssue(r.Context(), issue.ID, tagID); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		if issue.Tags, err = h.tagRepo.GetByIssueID(r.Context(), issue.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	writeData(w, http.StatusCreated, issue)
}

// resolveCreateTags turns a create request's tag_ids and tag names into tag
// ids: ids must exist, names are found case-insensitively or created.
func (h *IssueHandler) resolveCreateTags(r *http.Request, req model.CreateIssueRequest) ([]uuid.UUID, error) {
	if len(req.TagIDs) == 0 && len(req.Tags) == 0 {
		return nil, nil
	}
	ids, err := h.checkTagIDs(r, req.TagIDs)
	if err != nil {
		return nil, err
	}
	for _, name := range req.Tags {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		tag, err := h.tagRepo.GetByName(r.Context(), name)
		if err != nil {
			return nil, err
		}
		if tag == nil {
			if tag, err = h.tagRepo.Create(r.Context(), model.CreateTagRequest{Name: name}); err != nil {
				return nil, err
			}
		}
		ids = append(ids, tag.ID)
	}
	return dedupeIDs(ids), nil
}

// checkTagIDs verifies every id names an existing tag.
func (h *IssueHandler) checkTagIDs(r *http.Request, ids []uuid.UUID) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		tag, err := h.tagRepo.GetByID(r.Context(), id)
		if err != nil {
			return nil, err
		}
		if tag == nil {
			return nil, fmt.Errorf("tag %s not found", id)
		}
		out = append(out, id)
	}
	return dedupeIDs(out), nil
}

func dedupeIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(ids))
	out := ids[:0]
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// resolveTagFilter maps `tag` query values — tag ids or names, repeatable —
// to tag ids. ok is false when some named tag does not exist: no issue can
// carry it, so the (AND) filter matches nothing.
func (h *IssueHandler) resolveTagFilter(r *http.Request) (ids []uuid.UUID, ok bool, err error) {
	for _, v := range r.URL.Query()["tag"] {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if id, perr := uuid.Parse(v); perr == nil {
			ids = append(ids, id)
			continue
		}
		tag, err := h.tagRepo.GetByName(r.Context(), v)
		if err != nil {
			return nil, false, err
		}
		if tag == nil {
			return nil, false, nil
		}
		ids = append(ids, tag.ID)
	}
	return dedupeIDs(ids), true, nil
}

// attachTags fills in every listed issue's tags with one query.
func (h *IssueHandler) attachTags(r *http.Request, issues []model.Issue) error {
	if len(issues) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(issues))
	for i := range issues {
		ids[i] = issues[i].ID
	}
	byIssue, err := h.tagRepo.GetByIssueIDs(r.Context(), ids)
	if err != nil {
		return err
	}
	for i := range issues {
		issues[i].Tags = byIssue[issues[i].ID]
	}
	return nil
}

// setIssueTags makes the issue's tags exactly want.
func (h *IssueHandler) setIssueTags(r *http.Request, issueID uuid.UUID, want []uuid.UUID) error {
	current, err := h.tagRepo.GetByIssueID(r.Context(), issueID)
	if err != nil {
		return err
	}
	keep := make(map[uuid.UUID]bool, len(want))
	for _, id := range want {
		keep[id] = true
	}
	have := make(map[uuid.UUID]bool, len(current))
	for _, t := range current {
		have[t.ID] = true
		if !keep[t.ID] {
			if err := h.tagRepo.RemoveFromIssue(r.Context(), issueID, t.ID); err != nil {
				return err
			}
		}
	}
	for _, id := range want {
		if !have[id] {
			if err := h.tagRepo.AddToIssue(r.Context(), issueID, id); err != nil {
				return err
			}
		}
	}
	return nil
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
		Page:       queryInt(r, "page", 1),
		PageSize:   queryInt(r, "page_size", 20),
	}

	tagIDs, ok, err := h.resolveTagFilter(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeList(w, []model.Issue{}, 0, filter.Page, filter.PageSize)
		return
	}
	filter.TagIDs = tagIDs

	issues, total, err := h.svc.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if issues == nil {
		issues = []model.Issue{}
	}
	if err := h.attachTags(r, issues); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
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

	var tagIDs []uuid.UUID
	if req.TagIDs != nil {
		if tagIDs, err = h.checkTagIDs(r, *req.TagIDs); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	issue, err := h.svc.Update(r.Context(), id, req, operatorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.TagIDs != nil {
		if err := h.setIssueTags(r, issue.ID, tagIDs); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if issue.Tags, err = h.tagRepo.GetByIssueID(r.Context(), issue.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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
