package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

// The list endpoint takes tag names or ids (repeatable, ANDed), answers an
// unknown name with an empty page, and returns each issue's tags inline.
func TestListIssues_TagFilterAndInlineTags(t *testing.T) {
	h, issueRepo, projectRepo, _, tagRepo := newTestIssueHandler()
	projectID := uuid.New()
	roomID, aiID := uuid.New(), uuid.New()
	issueID := uuid.New()
	color := "#d73a4a"

	projectRepo.GetByKeyFn = func(ctx context.Context, key string) (*model.Project, error) {
		return &model.Project{ID: projectID, Key: key}, nil
	}
	tagRepo.GetByNameFn = func(ctx context.Context, name string) (*model.Tag, error) {
		switch strings.ToLower(name) {
		case "room":
			return &model.Tag{ID: roomID, Name: "room"}, nil
		case "enemy-ai":
			return &model.Tag{ID: aiID, Name: "enemy-ai"}, nil
		}
		return nil, nil
	}
	var gotFilter model.IssueFilter
	issueRepo.ListFn = func(ctx context.Context, f model.IssueFilter) ([]model.Issue, int, error) {
		gotFilter = f
		return []model.Issue{{ID: issueID, IssueKey: "OZX-1", Title: "t"}}, 1, nil
	}
	tagRepo.GetByIssueIDsFn = func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]model.Tag, error) {
		return map[uuid.UUID][]model.Tag{issueID: {{ID: roomID, Name: "room", Color: &color}}}, nil
	}

	do := func(q string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/OZX/issues?"+q, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("projectId", "OZX")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.ListByProject(w, req)
		return w
	}

	w := do("tag=Room&tag=" + aiID.String())
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if len(gotFilter.TagIDs) != 2 || gotFilter.TagIDs[0] != roomID || gotFilter.TagIDs[1] != aiID {
		t.Errorf("filter tag ids = %v", gotFilter.TagIDs)
	}
	var env struct {
		Data []struct {
			Tags []model.Tag `json:"tags"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 1 || len(env.Data[0].Tags) != 1 || env.Data[0].Tags[0].Name != "room" {
		t.Errorf("inline tags = %s", w.Body.String())
	}

	gotFilter = model.IssueFilter{}
	w = do("tag=room&tag=no-such-tag")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"total":0`) {
		t.Errorf("unknown tag: %d %s", w.Code, w.Body.String())
	}
	if gotFilter.ProjectID != nil {
		t.Error("unknown tag should short-circuit before querying issues")
	}
}

// Creating an issue with tag names finds existing tags and creates missing ones.
func TestCreateIssue_WithTagNames(t *testing.T) {
	h, issueRepo, projectRepo, _, tagRepo := newTestIssueHandler()
	projectID := uuid.New()
	uiID := uuid.New()
	var created []string
	attached := map[uuid.UUID]bool{}

	projectRepo.IncrementIssueNumberFn = func(ctx context.Context, tx database.Tx, id uuid.UUID) (int, string, error) {
		return 5, "MF", nil
	}
	issueRepo.BeginTxFn = func(ctx context.Context) (database.Tx, error) { return newMockTxForHandler(), nil }
	issueRepo.CreateFn = func(ctx context.Context, tx database.Tx, key string, pid uuid.UUID, req model.CreateIssueRequest) (*model.Issue, error) {
		return &model.Issue{ID: uuid.New(), IssueKey: key, Type: req.Type, Title: req.Title}, nil
	}
	tagRepo.GetByNameFn = func(ctx context.Context, name string) (*model.Tag, error) {
		if name == "ui" {
			return &model.Tag{ID: uiID, Name: "ui"}, nil
		}
		return nil, nil
	}
	tagRepo.CreateFn = func(ctx context.Context, req model.CreateTagRequest) (*model.Tag, error) {
		created = append(created, req.Name)
		return &model.Tag{ID: uuid.New(), Name: req.Name}, nil
	}
	tagRepo.AddToIssueFn = func(ctx context.Context, issueID, tagID uuid.UUID) error {
		attached[tagID] = true
		return nil
	}
	tagRepo.GetByIssueIDFn = func(ctx context.Context, issueID uuid.UUID) ([]model.Tag, error) {
		return []model.Tag{{Name: "P3-延后"}, {Name: "ui"}}, nil
	}

	body := `{"type":"bug","title":"x","tags":["ui","P3-延后","ui"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/issues", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("projectId", projectID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if len(created) != 1 || created[0] != "P3-延后" {
		t.Errorf("created tags = %v", created)
	}
	if len(attached) != 2 || !attached[uiID] {
		t.Errorf("attached = %v", attached)
	}
	if !strings.Contains(w.Body.String(), `"tags":[`) {
		t.Errorf("response lacks tags: %s", w.Body.String())
	}
}
