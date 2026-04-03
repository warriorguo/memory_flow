package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/service"
)

type DependencyHandler struct {
	svc *service.DependencyService
}

func NewDependencyHandler(svc *service.DependencyService) *DependencyHandler {
	return &DependencyHandler{svc: svc}
}

func (h *DependencyHandler) Create(w http.ResponseWriter, r *http.Request) {
	issueID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue id")
		return
	}

	var req model.CreateDependencyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	dep, err := h.svc.Create(r.Context(), issueID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeData(w, http.StatusCreated, dep)
}

func (h *DependencyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	depID, err := parseUUID(chi.URLParam(r, "depId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid dependency id")
		return
	}

	if err := h.svc.Delete(r.Context(), depID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *DependencyHandler) List(w http.ResponseWriter, r *http.Request) {
	issueID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue id")
		return
	}

	deps, err := h.svc.ListByIssueID(r.Context(), issueID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if deps == nil {
		deps = []model.IssueDependency{}
	}

	writeData(w, http.StatusOK, deps)
}

func (h *DependencyHandler) GetTree(w http.ResponseWriter, r *http.Request) {
	issueID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue id")
		return
	}

	tree, err := h.svc.GetDependencyTree(r.Context(), issueID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeData(w, http.StatusOK, tree)
}

func (h *DependencyHandler) GetEffectivePriority(w http.ResponseWriter, r *http.Request) {
	issueID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue id")
		return
	}

	priority, err := h.svc.GetEffectivePriority(r.Context(), issueID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeData(w, http.StatusOK, map[string]string{"effective_priority": priority})
}
