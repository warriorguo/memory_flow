package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	"github.com/warriorguo/memory_flow/backend/internal/repository/mocks"
	"github.com/warriorguo/memory_flow/backend/internal/service"
)

// fakeCommentRepo is a stateful stand-in: comments are a read-your-writes
// feature — post, list, mark read, list again — so the tests need storage that
// behaves rather than per-call stubs.
type fakeCommentRepo struct {
	comments map[uuid.UUID][]model.IssueComment
	receipts map[string]bool // commentID + "\x00" + reader
	clock    time.Time
}

func newFakeCommentRepo() *fakeCommentRepo {
	return &fakeCommentRepo{
		comments: map[uuid.UUID][]model.IssueComment{},
		receipts: map[string]bool{},
		clock:    time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC),
	}
}

func receiptKey(commentID uuid.UUID, reader string) string {
	return commentID.String() + "\x00" + reader
}

func (f *fakeCommentRepo) Create(_ context.Context, issueID uuid.UUID, req model.CreateCommentRequest) (*model.IssueComment, error) {
	f.clock = f.clock.Add(time.Minute)
	comment := model.IssueComment{
		ID:        uuid.New(),
		IssueID:   issueID,
		AuthorID:  req.AuthorID,
		Body:      req.Body,
		CreatedAt: f.clock,
		UpdatedAt: f.clock,
	}
	f.comments[issueID] = append(f.comments[issueID], comment)
	return &comment, nil
}

func (f *fakeCommentRepo) ListByIssueID(_ context.Context, issueID uuid.UUID) ([]model.IssueComment, error) {
	return append([]model.IssueComment(nil), f.comments[issueID]...), nil
}

func (f *fakeCommentRepo) Delete(_ context.Context, issueID, commentID uuid.UUID) error {
	for i, c := range f.comments[issueID] {
		if c.ID == commentID {
			f.comments[issueID] = append(f.comments[issueID][:i], f.comments[issueID][i+1:]...)
			return nil
		}
	}
	return repository.ErrCommentNotFound
}

func (f *fakeCommentRepo) ReadIDs(_ context.Context, issueID uuid.UUID, reader string) (map[uuid.UUID]bool, error) {
	read := map[uuid.UUID]bool{}
	for _, c := range f.comments[issueID] {
		if f.receipts[receiptKey(c.ID, reader)] {
			read[c.ID] = true
		}
	}
	return read, nil
}

func (f *fakeCommentRepo) MarkRead(_ context.Context, issueID uuid.UUID, reader string) error {
	for _, c := range f.comments[issueID] {
		f.receipts[receiptKey(c.ID, reader)] = true
	}
	return nil
}

type commentTest struct {
	t      *testing.T
	router chi.Router
	issue  uuid.UUID
}

func newCommentTest(t *testing.T) *commentTest {
	t.Helper()

	issueID := uuid.New()
	issueRepo := &mocks.MockIssueRepo{
		GetByKeyFn: func(_ context.Context, key string) (*model.Issue, error) {
			if key != "OZX-7" {
				return nil, nil
			}
			return &model.Issue{ID: issueID, IssueKey: "OZX-7"}, nil
		},
	}
	projectRepo := &mocks.MockProjectRepo{}
	issueSvc := service.NewIssueService(issueRepo, projectRepo, &mocks.MockIssueHistoryRepo{})
	resolver := NewIDResolver(service.NewProjectService(projectRepo), issueSvc)

	h := NewCommentHandler(service.NewCommentService(newFakeCommentRepo()), resolver)

	r := chi.NewRouter()
	r.Get("/issues/{id}/comments", h.List)
	r.Post("/issues/{id}/comments", h.Create)
	r.Post("/issues/{id}/comments/read", h.MarkRead)
	r.Delete("/issues/{id}/comments/{commentId}", h.Delete)

	return &commentTest{t: t, router: r, issue: issueID}
}

func (c *commentTest) do(method, path, body string) *httptest.ResponseRecorder {
	c.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	c.router.ServeHTTP(w, req)
	return w
}

func (c *commentTest) requireStatus(w *httptest.ResponseRecorder, want int) {
	c.t.Helper()
	if w.Code != want {
		c.t.Fatalf("status = %d, want %d: %s", w.Code, want, w.Body.String())
	}
}

// listComments decodes the list envelope, including its unread summary.
func (c *commentTest) listComments(reader string) ([]model.IssueComment, model.CommentSummary) {
	c.t.Helper()
	path := "/issues/OZX-7/comments"
	if reader != "" {
		path += "?reader=" + reader
	}
	w := c.do(http.MethodGet, path, "")
	c.requireStatus(w, http.StatusOK)

	var env struct {
		Data    []model.IssueComment `json:"data"`
		Total   int                  `json:"total"`
		Summary model.CommentSummary `json:"summary"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		c.t.Fatalf("decode list: %v\n%s", err, w.Body.String())
	}
	if env.Total != len(env.Data) {
		c.t.Errorf("total = %d, want %d", env.Total, len(env.Data))
	}
	return env.Data, env.Summary
}

func TestCommentPostListAndMarkRead(t *testing.T) {
	c := newCommentTest(t)

	if _, summary := c.listComments("bob"); summary.Total != 0 || summary.Unread != 0 {
		t.Errorf("empty thread summary = %+v", summary)
	}

	w := c.do(http.MethodPost, "/issues/OZX-7/comments", `{"body":"Repro'd on 1.4.2","author_id":"alice"}`)
	c.requireStatus(w, http.StatusCreated)
	c.do(http.MethodPost, "/issues/OZX-7/comments", `{"body":"Fix is in the loader","author_id":"alice"}`)

	// Bob has read nothing.
	comments, summary := c.listComments("bob")
	if summary.Total != 2 || summary.Unread != 2 {
		t.Errorf("summary = %+v, want 2 total / 2 unread", summary)
	}
	if !comments[0].Unread {
		t.Errorf("comment 0 should be unread for bob")
	}
	if summary.LastAuthor == nil || *summary.LastAuthor != "alice" {
		t.Errorf("last author = %v, want alice", summary.LastAuthor)
	}

	// Alice wrote them, so nothing is new to her.
	if _, summary := c.listComments("alice"); summary.Unread != 0 {
		t.Errorf("author's own unread = %d, want 0", summary.Unread)
	}

	// The reader can come from the query string alone — no body required.
	w = c.do(http.MethodPost, "/issues/OZX-7/comments/read?reader=bob", "")
	c.requireStatus(w, http.StatusOK)
	if _, summary := c.listComments("bob"); summary.Unread != 0 {
		t.Errorf("unread after mark read = %d, want 0", summary.Unread)
	}

	// A later comment is new again.
	c.do(http.MethodPost, "/issues/OZX-7/comments", `{"body":"Shipped","author_id":"carol"}`)
	if _, summary := c.listComments("bob"); summary.Unread != 1 {
		t.Errorf("unread after a new comment = %d, want 1", summary.Unread)
	}

	// Without a reader nobody is asking, so nothing is reported as unread.
	if _, summary := c.listComments(""); summary.Total != 3 || summary.Unread != 0 {
		t.Errorf("anonymous summary = %+v, want 3 total / 0 unread", summary)
	}
}

func TestCommentDeleteAndErrors(t *testing.T) {
	c := newCommentTest(t)

	w := c.do(http.MethodPost, "/issues/OZX-7/comments", `{"body":"oops","author_id":"alice"}`)
	c.requireStatus(w, http.StatusCreated)
	var created struct {
		Data model.IssueComment `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &created)

	c.requireStatus(c.do(http.MethodDelete, "/issues/OZX-7/comments/"+created.Data.ID.String(), ""), http.StatusOK)
	c.requireStatus(c.do(http.MethodDelete, "/issues/OZX-7/comments/"+created.Data.ID.String(), ""), http.StatusNotFound)
	c.requireStatus(c.do(http.MethodDelete, "/issues/OZX-7/comments/not-a-uuid", ""), http.StatusBadRequest)

	// An unknown issue is a 404, not a comment created against nothing.
	c.requireStatus(c.do(http.MethodGet, "/issues/OZX-99/comments", ""), http.StatusNotFound)
	c.requireStatus(c.do(http.MethodPost, "/issues/OZX-99/comments", `{"body":"hi"}`), http.StatusNotFound)

	// A blank body, and a body over the cap.
	c.requireStatus(c.do(http.MethodPost, "/issues/OZX-7/comments", `{"body":"   "}`), http.StatusBadRequest)
	long, _ := json.Marshal(map[string]string{"body": strings.Repeat("x", service.MaxCommentBytes+1)})
	c.requireStatus(c.do(http.MethodPost, "/issues/OZX-7/comments", string(long)), http.StatusRequestEntityTooLarge)

	// Marking read is inherently per-reader, so it needs one.
	c.requireStatus(c.do(http.MethodPost, "/issues/OZX-7/comments/read", ""), http.StatusBadRequest)
}
