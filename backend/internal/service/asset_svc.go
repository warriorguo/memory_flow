package service

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
)

// DefaultMaxAssetBytes caps a single asset. Asset bytes live in the database,
// so this is what keeps that tradeoff honest — big enough for reference art,
// short clips, and logs; small enough that no single row wrecks the database.
const DefaultMaxAssetBytes int64 = 32 << 20 // 32MB

// ErrAssetTooLarge is returned when an upload exceeds the configured cap.
var ErrAssetTooLarge = errors.New("asset too large")

// ErrInvalidFilename is returned for names that could escape their issue or
// confuse a client saving the file to disk.
var ErrInvalidFilename = errors.New("invalid filename")

type AssetService struct {
	repo     repository.AssetRepository
	maxBytes int64
}

// NewAssetService builds the service. A maxBytes of 0 or less means
// DefaultMaxAssetBytes.
func NewAssetService(repo repository.AssetRepository, maxBytes int64) *AssetService {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxAssetBytes
	}
	return &AssetService{repo: repo, maxBytes: maxBytes}
}

// MaxBytes reports the per-file cap, so handlers can reject an oversized body
// before reading all of it into memory.
func (s *AssetService) MaxBytes() int64 { return s.maxBytes }

// SanitizeFilename validates the name an asset will be addressed by. Names are
// the public handle for an asset — descriptions reference them and clients
// write them to disk — so anything that could escape the issue's namespace or
// a download directory is rejected outright rather than silently rewritten.
func SanitizeFilename(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%w: filename is required", ErrInvalidFilename)
	}
	if len(name) > 255 {
		return "", fmt.Errorf("%w: longer than 255 characters", ErrInvalidFilename)
	}
	if strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("%w: must not contain a path separator", ErrInvalidFilename)
	}
	if name == "." || name == ".." {
		return "", fmt.Errorf("%w: %q is not a file name", ErrInvalidFilename, name)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("%w: contains a control character", ErrInvalidFilename)
		}
	}
	return name, nil
}

// DetectMimeType falls back to the extension when the client sent nothing
// useful. Browsers omit the type for plenty of formats, and a wrong
// application/octet-stream would stop the UI from previewing the file.
func DetectMimeType(declared, filename string) string {
	declared = strings.TrimSpace(declared)
	if declared != "" && declared != "application/octet-stream" {
		return declared
	}
	if byExt := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename))); byExt != "" {
		return byExt
	}
	if declared != "" {
		return declared
	}
	return "application/octet-stream"
}

func (s *AssetService) validate(filename string, content []byte) (string, error) {
	clean, err := SanitizeFilename(filename)
	if err != nil {
		return "", err
	}
	if int64(len(content)) > s.maxBytes {
		return "", fmt.Errorf("%w: %d bytes exceeds the %d byte limit", ErrAssetTooLarge, len(content), s.maxBytes)
	}
	return clean, nil
}

// Create attaches a new asset. An existing filename is a conflict, not an
// overwrite — see [repository.ErrAssetExists].
func (s *AssetService) Create(ctx context.Context, issueID uuid.UUID, req model.PutAssetRequest) (*model.IssueAsset, error) {
	clean, err := s.validate(req.Filename, req.Content)
	if err != nil {
		return nil, err
	}
	req.Filename = clean
	req.MimeType = DetectMimeType(req.MimeType, clean)
	return s.repo.Create(ctx, issueID, req)
}

// Replace overwrites an existing asset's content in place.
func (s *AssetService) Replace(ctx context.Context, issueID uuid.UUID, filename string, req model.PutAssetRequest) (*model.IssueAsset, error) {
	clean, err := s.validate(filename, req.Content)
	if err != nil {
		return nil, err
	}
	req.Filename = clean
	req.MimeType = DetectMimeType(req.MimeType, clean)
	return s.repo.Replace(ctx, issueID, clean, req)
}

// Upsert creates the asset, or replaces it when the filename is already taken.
// Used by clients that have explicitly chosen to overwrite.
func (s *AssetService) Upsert(ctx context.Context, issueID uuid.UUID, req model.PutAssetRequest) (*model.IssueAsset, error) {
	asset, err := s.Create(ctx, issueID, req)
	if errors.Is(err, repository.ErrAssetExists) {
		return s.Replace(ctx, issueID, req.Filename, req)
	}
	return asset, err
}

func (s *AssetService) List(ctx context.Context, issueID uuid.UUID) ([]model.IssueAsset, error) {
	return s.repo.ListByIssueID(ctx, issueID)
}

func (s *AssetService) Get(ctx context.Context, issueID uuid.UUID, filename string) (*model.IssueAsset, error) {
	return s.repo.GetByFilename(ctx, issueID, filename)
}

// GetContent returns the metadata and the bytes together, or a nil asset when
// the issue has nothing under that name.
func (s *AssetService) GetContent(ctx context.Context, issueID uuid.UUID, filename string) (*model.IssueAsset, []byte, error) {
	asset, err := s.repo.GetByFilename(ctx, issueID, filename)
	if err != nil || asset == nil {
		return nil, nil, err
	}
	content, err := s.repo.GetContent(ctx, asset.ID)
	if err != nil {
		return nil, nil, err
	}
	return asset, content, nil
}

func (s *AssetService) Delete(ctx context.Context, issueID uuid.UUID, filename string) error {
	return s.repo.Delete(ctx, issueID, filename)
}
