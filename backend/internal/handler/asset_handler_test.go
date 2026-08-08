package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
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

// fakeAssetRepo is a stateful in-memory stand-in for the asset repository.
// Assets are a read-your-writes feature — upload, list, download, replace — so
// the tests need storage that behaves, not per-call stubs.
type fakeAssetRepo struct {
	assets  map[uuid.UUID][]model.IssueAsset
	content map[uuid.UUID][]byte
}

func newFakeAssetRepo() *fakeAssetRepo {
	return &fakeAssetRepo{
		assets:  map[uuid.UUID][]model.IssueAsset{},
		content: map[uuid.UUID][]byte{},
	}
}

func (f *fakeAssetRepo) Create(ctx context.Context, issueID uuid.UUID, req model.PutAssetRequest) (*model.IssueAsset, error) {
	if existing, _ := f.GetByFilename(ctx, issueID, req.Filename); existing != nil {
		return nil, repository.ErrAssetExists
	}
	asset := model.IssueAsset{
		ID:        uuid.New(),
		IssueID:   issueID,
		Filename:  req.Filename,
		MimeType:  req.MimeType,
		SizeBytes: int64(len(req.Content)),
		Checksum:  repository.Checksum(req.Content),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	f.assets[issueID] = append(f.assets[issueID], asset)
	f.content[asset.ID] = req.Content
	return &asset, nil
}

func (f *fakeAssetRepo) Replace(ctx context.Context, issueID uuid.UUID, filename string, req model.PutAssetRequest) (*model.IssueAsset, error) {
	for i, a := range f.assets[issueID] {
		if a.Filename == filename {
			a.MimeType = req.MimeType
			a.SizeBytes = int64(len(req.Content))
			a.Checksum = repository.Checksum(req.Content)
			a.UpdatedAt = time.Now()
			f.assets[issueID][i] = a
			f.content[a.ID] = req.Content
			return &a, nil
		}
	}
	return nil, repository.ErrAssetNotFound
}

func (f *fakeAssetRepo) ListByIssueID(ctx context.Context, issueID uuid.UUID) ([]model.IssueAsset, error) {
	return f.assets[issueID], nil
}

func (f *fakeAssetRepo) GetByFilename(ctx context.Context, issueID uuid.UUID, filename string) (*model.IssueAsset, error) {
	for _, a := range f.assets[issueID] {
		if a.Filename == filename {
			found := a
			return &found, nil
		}
	}
	return nil, nil
}

func (f *fakeAssetRepo) GetContent(ctx context.Context, assetID uuid.UUID) ([]byte, error) {
	return f.content[assetID], nil
}

func (f *fakeAssetRepo) Delete(ctx context.Context, issueID uuid.UUID, filename string) error {
	for i, a := range f.assets[issueID] {
		if a.Filename == filename {
			f.assets[issueID] = append(f.assets[issueID][:i], f.assets[issueID][i+1:]...)
			delete(f.content, a.ID)
			return nil
		}
	}
	return repository.ErrAssetNotFound
}

// assetTest is the router plus the fixtures the asset endpoints are exercised
// against.
type assetTest struct {
	t      *testing.T
	router chi.Router
	repo   *fakeAssetRepo
	issue  uuid.UUID
}

func newAssetTest(t *testing.T, maxBytes int64) *assetTest {
	t.Helper()

	issueID := uuid.New()
	issueRepo := &mocks.MockIssueRepo{
		GetByKeyFn: func(ctx context.Context, key string) (*model.Issue, error) {
			if key == "OZX-7" {
				return &model.Issue{ID: issueID, IssueKey: key}, nil
			}
			return nil, nil
		},
		// Delete reads the description to warn about references it leaves behind.
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
			return &model.Issue{ID: issueID, IssueKey: "OZX-7"}, nil
		},
	}
	projectRepo := &mocks.MockProjectRepo{}
	issueSvc := service.NewIssueService(issueRepo, projectRepo, &mocks.MockIssueHistoryRepo{})
	projectSvc := service.NewProjectService(projectRepo)
	resolver := NewIDResolver(projectSvc, issueSvc)

	repo := newFakeAssetRepo()
	h := NewAssetHandler(service.NewAssetService(repo, maxBytes), issueSvc, resolver)

	r := chi.NewRouter()
	r.Get("/issues/{id}/assets", h.List)
	r.Post("/issues/{id}/assets", h.Create)
	r.Get("/issues/{id}/assets/{filename}", h.Get)
	r.Put("/issues/{id}/assets/{filename}", h.Replace)
	r.Delete("/issues/{id}/assets/{filename}", h.Delete)

	return &assetTest{t: t, router: r, repo: repo, issue: issueID}
}

// upload posts a multipart body the way a browser would.
func (a *assetTest) upload(path, filename, contentType string, content []byte) *httptest.ResponseRecorder {
	a.t.Helper()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreatePart(map[string][]string{
		"Content-Disposition": {fmt.Sprintf(`form-data; name="file"; filename=%q`, filename)},
		"Content-Type":        {contentType},
	})
	if err != nil {
		a.t.Fatalf("create multipart part: %v", err)
	}
	part.Write(content)
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return a.do(req)
}

func (a *assetTest) do(req *http.Request) *httptest.ResponseRecorder {
	a.t.Helper()
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, req)
	return w
}

func (a *assetTest) requireStatus(w *httptest.ResponseRecorder, want int) {
	a.t.Helper()
	if w.Code != want {
		a.t.Fatalf("status = %d, want %d: %s", w.Code, want, w.Body.String())
	}
}

// The whole lifecycle over one issue, addressed by its key rather than its
// UUID, since that is how the CLI and the agent will call it.
func TestAssetLifecycle_Handler(t *testing.T) {
	a := newAssetTest(t, 0)
	base := "/issues/OZX-7/assets"

	png := []byte{0x89, 'P', 'N', 'G', 0x00, 0x0d, 0x0a, 0x1a, 0xff}
	w := a.upload(base, "reference-art.png", "image/png", png)
	a.requireStatus(w, http.StatusCreated)

	var created struct {
		Data model.IssueAsset `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Data.Filename != "reference-art.png" || created.Data.MimeType != "image/png" {
		t.Errorf("created = %+v", created.Data)
	}
	if created.Data.SizeBytes != int64(len(png)) {
		t.Errorf("size = %d, want %d", created.Data.SizeBytes, len(png))
	}

	// List carries metadata and no bytes.
	w = a.do(httptest.NewRequest(http.MethodGet, base, nil))
	a.requireStatus(w, http.StatusOK)
	if !strings.Contains(w.Body.String(), "reference-art.png") {
		t.Errorf("list missing the asset: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "content") {
		t.Errorf("list response leaked a content field: %s", w.Body.String())
	}

	// Download returns the exact bytes, typed and inline so a browser previews.
	w = a.do(httptest.NewRequest(http.MethodGet, base+"/reference-art.png", nil))
	a.requireStatus(w, http.StatusOK)
	if !bytes.Equal(w.Body.Bytes(), png) {
		t.Errorf("body = %v, want %v", w.Body.Bytes(), png)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "inline;") {
		t.Errorf("Content-Disposition = %q, want inline", cd)
	}

	// ?download=1 flips it to an attachment.
	w = a.do(httptest.NewRequest(http.MethodGet, base+"/reference-art.png?download=1", nil))
	if cd := w.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment;") {
		t.Errorf("Content-Disposition = %q, want attachment", cd)
	}

	// Replace keeps the id so any handed-out link still resolves.
	revised := []byte("final artwork")
	req := httptest.NewRequest(http.MethodPut, base+"/reference-art.png", bytes.NewReader(revised))
	req.Header.Set("Content-Type", "image/png")
	w = a.do(req)
	a.requireStatus(w, http.StatusOK)

	var replaced struct {
		Data model.IssueAsset `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &replaced)
	if replaced.Data.ID != created.Data.ID {
		t.Errorf("replace changed the id: %v → %v", created.Data.ID, replaced.Data.ID)
	}

	w = a.do(httptest.NewRequest(http.MethodGet, base+"/reference-art.png", nil))
	if !bytes.Equal(w.Body.Bytes(), revised) {
		t.Errorf("body after replace = %q, want %q", w.Body.Bytes(), revised)
	}

	// Delete, then the asset is gone.
	w = a.do(httptest.NewRequest(http.MethodDelete, base+"/reference-art.png", nil))
	a.requireStatus(w, http.StatusOK)
	w = a.do(httptest.NewRequest(http.MethodGet, base+"/reference-art.png", nil))
	a.requireStatus(w, http.StatusNotFound)
	w = a.do(httptest.NewRequest(http.MethodDelete, base+"/reference-art.png", nil))
	a.requireStatus(w, http.StatusNotFound)
}

// Without Range support a video player cannot seek, so this is load-bearing
// for the "assets can be video" half of the requirement.
func TestAssetGetServesRangeRequests(t *testing.T) {
	a := newAssetTest(t, 0)
	base := "/issues/OZX-7/assets"

	frames := []byte("0123456789abcdef")
	a.requireStatus(a.upload(base, "clip.mp4", "video/mp4", frames), http.StatusCreated)

	req := httptest.NewRequest(http.MethodGet, base+"/clip.mp4", nil)
	req.Header.Set("Range", "bytes=4-9")
	w := a.do(req)

	a.requireStatus(w, http.StatusPartialContent)
	if got := w.Body.String(); got != "456789" {
		t.Errorf("range body = %q, want %q", got, "456789")
	}
	if cr := w.Header().Get("Content-Range"); cr != "bytes 4-9/16" {
		t.Errorf("Content-Range = %q, want bytes 4-9/16", cr)
	}
	if ar := w.Header().Get("Accept-Ranges"); ar != "bytes" {
		t.Errorf("Accept-Ranges = %q, want bytes", ar)
	}
}

func TestAssetGetHonorsETag(t *testing.T) {
	a := newAssetTest(t, 0)
	base := "/issues/OZX-7/assets"

	a.requireStatus(a.upload(base, "spec.md", "text/markdown", []byte("# spec")), http.StatusCreated)

	w := a.do(httptest.NewRequest(http.MethodGet, base+"/spec.md", nil))
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag on the download response")
	}

	req := httptest.NewRequest(http.MethodGet, base+"/spec.md", nil)
	req.Header.Set("If-None-Match", etag)
	w = a.do(req)
	if w.Code != http.StatusNotModified {
		t.Errorf("status = %d, want 304", w.Code)
	}
}

// A repeat upload must not quietly clobber the earlier file — that is how a
// second person's artwork disappears.
func TestAssetCreateConflictsOnDuplicateFilename(t *testing.T) {
	a := newAssetTest(t, 0)
	base := "/issues/OZX-7/assets"

	a.requireStatus(a.upload(base, "enemy.png", "image/png", []byte("v1")), http.StatusCreated)

	w := a.upload(base, "enemy.png", "image/png", []byte("v2"))
	a.requireStatus(w, http.StatusConflict)
	if !strings.Contains(w.Body.String(), "PUT") {
		t.Errorf("conflict message should point at replace: %s", w.Body.String())
	}

	// Opting in overwrites.
	w = a.upload(base+"?overwrite=true", "enemy.png", "image/png", []byte("v2"))
	a.requireStatus(w, http.StatusCreated)

	w = a.do(httptest.NewRequest(http.MethodGet, base+"/enemy.png", nil))
	if got := w.Body.String(); got != "v2" {
		t.Errorf("body = %q, want v2", got)
	}
}

func TestAssetUploadRejectsOversizedBody(t *testing.T) {
	a := newAssetTest(t, 16)
	w := a.upload("/issues/OZX-7/assets", "big.bin", "application/octet-stream", bytes.Repeat([]byte("x"), 64))
	a.requireStatus(w, http.StatusRequestEntityTooLarge)
}

func TestAssetUploadRejectsUnsafeFilenames(t *testing.T) {
	a := newAssetTest(t, 0)
	base := "/issues/OZX-7/assets"

	for _, name := range []string{"../escape.txt", "dir/nested.txt", "", ".."} {
		w := a.do(func() *http.Request {
			req := httptest.NewRequest(http.MethodPost, base+"?filename="+name, strings.NewReader("x"))
			req.Header.Set("Content-Type", "text/plain")
			return req
		}())
		if w.Code != http.StatusBadRequest {
			t.Errorf("filename %q: status = %d, want 400 (%s)", name, w.Code, w.Body.String())
		}
	}
}

// A raw body with ?filename= is the scripted path — what the CLI uses when it
// streams a file up.
func TestAssetUploadFromRawBody(t *testing.T) {
	a := newAssetTest(t, 0)

	req := httptest.NewRequest(http.MethodPost, "/issues/OZX-7/assets?filename=notes.txt", strings.NewReader("hello"))
	req.Header.Set("Content-Type", "text/plain")
	a.requireStatus(a.do(req), http.StatusCreated)

	w := a.do(httptest.NewRequest(http.MethodGet, "/issues/OZX-7/assets/notes.txt", nil))
	if got := w.Body.String(); got != "hello" {
		t.Errorf("body = %q, want hello", got)
	}
}

// The mime type is inferred from the extension when the client did not send a
// useful one, or the UI would refuse to preview perfectly previewable files.
func TestAssetMimeInferredFromExtension(t *testing.T) {
	a := newAssetTest(t, 0)

	w := a.upload("/issues/OZX-7/assets", "sound.wav", "application/octet-stream", []byte("RIFF"))
	a.requireStatus(w, http.StatusCreated)

	var created struct {
		Data model.IssueAsset `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &created)
	if !strings.Contains(created.Data.MimeType, "wav") {
		t.Errorf("mime = %q, want something wav-ish", created.Data.MimeType)
	}
}

// Deleting an asset the description still points at must warn — the reference
// does not disappear with the file.
func TestAssetDeleteWarnsWhenStillReferenced(t *testing.T) {
	description := "参考图见 asset:enemy_ref.png，日志 asset:crash.log"
	issueID := uuid.New()
	issueRepo := &mocks.MockIssueRepo{
		GetByKeyFn: func(ctx context.Context, key string) (*model.Issue, error) {
			return &model.Issue{ID: issueID, IssueKey: key, Description: &description}, nil
		},
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Issue, error) {
			return &model.Issue{ID: issueID, IssueKey: "OZX-7", Description: &description}, nil
		},
	}
	projectRepo := &mocks.MockProjectRepo{}
	issueSvc := service.NewIssueService(issueRepo, projectRepo, &mocks.MockIssueHistoryRepo{})
	resolver := NewIDResolver(service.NewProjectService(projectRepo), issueSvc)

	repo := newFakeAssetRepo()
	h := NewAssetHandler(service.NewAssetService(repo, 0), issueSvc, resolver)
	r := chi.NewRouter()
	r.Post("/issues/{id}/assets", h.Create)
	r.Delete("/issues/{id}/assets/{filename}", h.Delete)

	a := &assetTest{t: t, router: r, repo: repo, issue: issueID}
	base := "/issues/OZX-7/assets"
	a.requireStatus(a.upload(base, "enemy_ref.png", "image/png", []byte("art")), http.StatusCreated)
	a.requireStatus(a.upload(base, "unused.png", "image/png", []byte("art")), http.StatusCreated)

	// Referenced: warn, but still delete.
	w := a.do(httptest.NewRequest(http.MethodDelete, base+"/enemy_ref.png", nil))
	a.requireStatus(w, http.StatusOK)
	if !strings.Contains(w.Body.String(), "still references") {
		t.Errorf("no warning for a referenced asset: %s", w.Body.String())
	}

	// Unreferenced: no warning to give.
	w = a.do(httptest.NewRequest(http.MethodDelete, base+"/unused.png", nil))
	a.requireStatus(w, http.StatusOK)
	if strings.Contains(w.Body.String(), "warning") {
		t.Errorf("unexpected warning for an unreferenced asset: %s", w.Body.String())
	}
}

func TestAssetUnknownIssueIs404(t *testing.T) {
	a := newAssetTest(t, 0)
	w := a.do(httptest.NewRequest(http.MethodGet, "/issues/NOPE-1/assets", nil))
	a.requireStatus(w, http.StatusNotFound)
}
