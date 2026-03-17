package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/warriorguo/memory_flow/backend/internal/service"
)

type ProgressHandler struct {
	svc *service.ProgressService
}

func NewProgressHandler(svc *service.ProgressService) *ProgressHandler {
	return &ProgressHandler{svc: svc}
}

func (h *ProgressHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseUUID(chi.URLParam(r, "projectId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	summary, err := h.svc.GetSummary(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeData(w, http.StatusOK, summary)
}

func (h *ProgressHandler) GetTrend(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseUUID(chi.URLParam(r, "projectId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	days := queryInt(r, "days", 30)

	trend, err := h.svc.GetTrend(r.Context(), projectID, days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeData(w, http.StatusOK, trend)
}
