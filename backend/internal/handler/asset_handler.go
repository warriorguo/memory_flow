package handler

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	"github.com/warriorguo/memory_flow/backend/internal/service"
)

type AssetHandler struct {
	svc      *service.AssetService
	resolver *IDResolver
}

func NewAssetHandler(svc *service.AssetService, resolver *IDResolver) *AssetHandler {
	return &AssetHandler{svc: svc, resolver: resolver}
}

// resolveIssue maps the {id} path parameter — an issue key like OZX-12 or a
// UUID — onto the issue's UUID, writing a 404 and reporting false if it does
// not resolve.
func (h *AssetHandler) resolveIssue(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := h.resolver.ResolveIssueID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return uuid.Nil, false
	}
	return id, true
}

// pathFilename reads the {filename} path parameter. chi hands back the raw
// path segment when the URL was percent-encoded, which it is for any name with
// a space or a non-ASCII character, so decode it before matching on it.
func pathFilename(r *http.Request) string {
	raw := chi.URLParam(r, "filename")
	if decoded, err := url.PathUnescape(raw); err == nil {
		return decoded
	}
	return raw
}

// readUpload pulls the bytes out of the request. A multipart form field named
// "file" is the browser path; anything else is treated as a raw body, which is
// what a `curl --data-binary` or a scripted PUT sends. The declared filename
// comes from the multipart part, the `filename` query parameter, or the URL.
func (h *AssetHandler) readUpload(w http.ResponseWriter, r *http.Request, urlFilename string) (model.PutAssetRequest, bool) {
	limit := h.svc.MaxBytes()
	// One byte of headroom so a body exactly at the limit still reads whole and
	// anything larger is detectable rather than silently truncated.
	r.Body = http.MaxBytesReader(w, r.Body, limit+1)

	var req model.PutAssetRequest

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		file, header, err := r.FormFile("file")
		if err != nil {
			if isTooLarge(err) {
				h.writeTooLarge(w, limit)
				return req, false
			}
			writeError(w, http.StatusBadRequest, "expected a multipart field named 'file'")
			return req, false
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			writeError(w, http.StatusBadRequest, "could not read the uploaded file")
			return req, false
		}
		req.Content = content
		req.Filename = header.Filename
		req.MimeType = header.Header.Get("Content-Type")
	} else {
		content, err := io.ReadAll(r.Body)
		if err != nil {
			if isTooLarge(err) {
				h.writeTooLarge(w, limit)
				return req, false
			}
			writeError(w, http.StatusBadRequest, "could not read the request body")
			return req, false
		}
		req.Content = content
		req.MimeType = r.Header.Get("Content-Type")
	}

	if name := r.URL.Query().Get("filename"); name != "" {
		req.Filename = name
	}
	if req.Filename == "" {
		req.Filename = urlFilename
	}
	if int64(len(req.Content)) > limit {
		h.writeTooLarge(w, limit)
		return req, false
	}
	return req, true
}

func isTooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr) || strings.Contains(err.Error(), "request body too large")
}

func (h *AssetHandler) writeTooLarge(w http.ResponseWriter, limit int64) {
	writeError(w, http.StatusRequestEntityTooLarge,
		fmt.Sprintf("asset exceeds the %d byte limit", limit))
}

// writeAssetError maps the service and repository sentinels onto status codes.
func writeAssetError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrAssetExists):
		writeError(w, http.StatusConflict, "an asset with that filename already exists — replace it with PUT instead")
	case errors.Is(err, repository.ErrAssetNotFound):
		writeError(w, http.StatusNotFound, "asset not found")
	case errors.Is(err, service.ErrAssetTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, service.ErrInvalidFilename):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

// Create handles POST /issues/{id}/assets. An existing filename is a 409 so an
// upload cannot quietly overwrite someone else's file; ?overwrite=true opts in.
func (h *AssetHandler) Create(w http.ResponseWriter, r *http.Request) {
	issueID, ok := h.resolveIssue(w, r)
	if !ok {
		return
	}
	req, ok := h.readUpload(w, r, "")
	if !ok {
		return
	}

	create := h.svc.Create
	if r.URL.Query().Get("overwrite") == "true" {
		create = h.svc.Upsert
	}

	asset, err := create(r.Context(), issueID, req)
	if err != nil {
		writeAssetError(w, err)
		return
	}
	writeData(w, http.StatusCreated, asset)
}

// List handles GET /issues/{id}/assets. Metadata only — content never rides a
// list response.
func (h *AssetHandler) List(w http.ResponseWriter, r *http.Request) {
	issueID, ok := h.resolveIssue(w, r)
	if !ok {
		return
	}
	assets, err := h.svc.List(r.Context(), issueID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if assets == nil {
		assets = []model.IssueAsset{}
	}
	writeList(w, assets, len(assets), 1, len(assets))
}

// Get handles GET /issues/{id}/assets/{filename}, serving the bytes.
//
// http.ServeContent does the heavy lifting: Range requests (so video and audio
// scrub instead of downloading whole), If-None-Match against the checksum ETag,
// and 206/416 handling.
func (h *AssetHandler) Get(w http.ResponseWriter, r *http.Request) {
	issueID, ok := h.resolveIssue(w, r)
	if !ok {
		return
	}
	filename := pathFilename(r)

	asset, content, err := h.svc.GetContent(r.Context(), issueID, filename)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if asset == nil {
		writeError(w, http.StatusNotFound, "asset not found")
		return
	}

	w.Header().Set("Content-Type", asset.MimeType)
	w.Header().Set("ETag", `"`+asset.Checksum+`"`)
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", contentDisposition("attachment", asset.Filename))
	} else {
		w.Header().Set("Content-Disposition", contentDisposition("inline", asset.Filename))
	}

	http.ServeContent(w, r, asset.Filename, asset.UpdatedAt, bytes.NewReader(content))
}

// contentDisposition builds the header with both the plain and the UTF-8
// filename forms, so non-ASCII names survive the trip.
func contentDisposition(kind, filename string) string {
	ascii := strings.Map(func(r rune) rune {
		if r < 32 || r > 126 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, filename)
	return fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, kind, ascii, urlEscape(filename))
}

func urlEscape(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

// Replace handles PUT /issues/{id}/assets/{filename}, keeping the asset's id so
// links and asset: references stay valid.
func (h *AssetHandler) Replace(w http.ResponseWriter, r *http.Request) {
	issueID, ok := h.resolveIssue(w, r)
	if !ok {
		return
	}
	filename := pathFilename(r)
	req, ok := h.readUpload(w, r, filename)
	if !ok {
		return
	}

	asset, err := h.svc.Replace(r.Context(), issueID, filename, req)
	if err != nil {
		writeAssetError(w, err)
		return
	}
	writeData(w, http.StatusOK, asset)
}

// Delete handles DELETE /issues/{id}/assets/{filename}.
func (h *AssetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	issueID, ok := h.resolveIssue(w, r)
	if !ok {
		return
	}
	filename := pathFilename(r)

	if err := h.svc.Delete(r.Context(), issueID, filename); err != nil {
		writeAssetError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]string{"status": "deleted", "filename": filename})
}
