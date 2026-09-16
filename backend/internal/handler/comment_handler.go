package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	"github.com/warriorguo/memory_flow/backend/internal/service"
)

type CommentHandler struct {
	svc      *service.CommentService
	resolver *IDResolver
}

func NewCommentHandler(svc *service.CommentService, resolver *IDResolver) *CommentHandler {
	return &CommentHandler{svc: svc, resolver: resolver}
}

// resolveIssue maps the {id} path parameter — an issue key like MF-1 or a UUID
// — onto the issue's UUID, writing a 404 and reporting false if it does not
// resolve.
func (h *CommentHandler) resolveIssue(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := h.resolver.ResolveIssueID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return uuid.Nil, false
	}
	return id, true
}

// reader identifies who is asking, so the response can say which comments are
// new to them. It is a free-form string — a username, an agent name — matching
// the convention assignee_id and creator_id already follow. There is no auth on
// this API, so the caller names itself.
func reader(r *http.Request) string {
	return r.URL.Query().Get("reader")
}

func writeCommentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrEmptyComment), errors.Is(err, service.ErrReaderRequired):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrCommentTooLong):
		writeError(w, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, repository.ErrCommentNotFound):
		writeError(w, http.StatusNotFound, "comment not found")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

// List handles GET /issues/{id}/comments[?reader=X]. With a reader, every
// comment carries an `unread` flag and the envelope carries a summary.
func (h *CommentHandler) List(w http.ResponseWriter, r *http.Request) {
	issueID, ok := h.resolveIssue(w, r)
	if !ok {
		return
	}

	comments, err := h.svc.List(r.Context(), issueID, reader(r))
	if err != nil {
		writeCommentError(w, err)
		return
	}
	if comments == nil {
		comments = []model.IssueComment{}
	}
	writeJSON(w, http.StatusOK, commentListResponse{
		listResponse: listResponse{
			Data:     comments,
			Total:    len(comments),
			Page:     1,
			PageSize: len(comments),
		},
		Summary: service.SummarizeComments(comments),
	})
}

// commentListResponse is the list envelope plus the unread summary, so a client
// rendering "3 comments, 2 unread" needs one request rather than two.
type commentListResponse struct {
	listResponse
	Summary *model.CommentSummary `json:"summary"`
}

// Create handles POST /issues/{id}/comments.
func (h *CommentHandler) Create(w http.ResponseWriter, r *http.Request) {
	issueID, ok := h.resolveIssue(w, r)
	if !ok {
		return
	}

	var req model.CreateCommentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	comment, err := h.svc.Create(r.Context(), issueID, req)
	if err != nil {
		writeCommentError(w, err)
		return
	}
	writeData(w, http.StatusCreated, comment)
}

// MarkRead handles POST /issues/{id}/comments/read, moving the reader's marker
// to the newest comment. The reader comes from the body or the query string.
func (h *CommentHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	issueID, ok := h.resolveIssue(w, r)
	if !ok {
		return
	}

	// The body is optional: ?reader=X alone is a complete request, which is what
	// a "mark read" beacon from a browser or a shell one-liner sends.
	var req model.MarkCommentsReadRequest
	_ = decodeJSON(r, &req)
	if req.Reader == "" {
		req.Reader = reader(r)
	}

	summary, err := h.svc.MarkRead(r.Context(), issueID, req.Reader)
	if err != nil {
		writeCommentError(w, err)
		return
	}
	writeData(w, http.StatusOK, summary)
}

// Delete handles DELETE /issues/{id}/comments/{commentId}.
func (h *CommentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	issueID, ok := h.resolveIssue(w, r)
	if !ok {
		return
	}
	commentID, err := parseUUID(chi.URLParam(r, "commentId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid comment id")
		return
	}

	if err := h.svc.Delete(r.Context(), issueID, commentID); err != nil {
		writeCommentError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]string{"status": "deleted", "id": commentID.String()})
}
