package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

// assetColumns is every column except content — list and lookup queries must
// never drag a blob along.
const assetColumns = `id, issue_id, filename, mime_type, size_bytes, checksum, created_at, updated_at`

type AssetRepo struct {
	db    database.DB
	store ContentStore
}

func NewAssetRepo(db database.DB, store ContentStore) *AssetRepo {
	return &AssetRepo{db: db, store: store}
}

// ErrAssetExists is returned by Create when the issue already has an asset
// under that filename. Replacing is a separate, explicit call so an upload
// cannot silently overwrite someone else's file.
var ErrAssetExists = fmt.Errorf("asset already exists")

// ErrAssetNotFound is returned when the issue has no asset under that filename.
var ErrAssetNotFound = fmt.Errorf("asset not found")

func scanAsset(row database.Row) (*model.IssueAsset, error) {
	var a model.IssueAsset
	err := row.Scan(&a.ID, &a.IssueID, &a.Filename, &a.MimeType, &a.SizeBytes, &a.Checksum, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// Checksum is the content digest recorded on every asset. Sync compares it to
// decide which blobs actually need transferring, and the API serves it as an
// ETag.
func Checksum(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// Create attaches a new asset to an issue. Metadata and bytes are written in
// one transaction so a reader can never observe a row whose content has not
// landed yet.
func (r *AssetRepo) Create(ctx context.Context, issueID uuid.UUID, req model.PutAssetRequest) (*model.IssueAsset, error) {
	existing, err := r.GetByFilename(ctx, issueID, req.Filename)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrAssetExists
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin asset create: %w", err)
	}
	defer tx.Rollback(ctx)

	query := fmt.Sprintf(`
		INSERT INTO issue_assets (id, issue_id, filename, mime_type, size_bytes, checksum)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING %s`, assetColumns)

	row := tx.QueryRow(ctx, query, uuid.New(), issueID, req.Filename, req.MimeType,
		int64(len(req.Content)), Checksum(req.Content))
	asset, err := scanAsset(row)
	if err != nil {
		return nil, fmt.Errorf("create asset: %w", err)
	}
	if err := r.store.Put(ctx, tx, asset.ID, req.Content); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit asset create: %w", err)
	}
	return asset, nil
}

// Replace overwrites the content of an existing asset, keeping its id so
// asset: references in descriptions and any recorded link stay valid.
func (r *AssetRepo) Replace(ctx context.Context, issueID uuid.UUID, filename string, req model.PutAssetRequest) (*model.IssueAsset, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin asset replace: %w", err)
	}
	defer tx.Rollback(ctx)

	query := fmt.Sprintf(`
		UPDATE issue_assets
		SET mime_type = $1, size_bytes = $2, checksum = $3, updated_at = now()
		WHERE issue_id = $4 AND filename = $5
		RETURNING %s`, assetColumns)

	row := tx.QueryRow(ctx, query, req.MimeType, int64(len(req.Content)), Checksum(req.Content), issueID, filename)
	asset, err := scanAsset(row)
	if err != nil {
		if err == database.ErrNoRows {
			return nil, ErrAssetNotFound
		}
		return nil, fmt.Errorf("replace asset: %w", err)
	}
	if err := r.store.Put(ctx, tx, asset.ID, req.Content); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit asset replace: %w", err)
	}
	return asset, nil
}

// ListByIssueID returns the issue's assets, oldest first, without their bytes.
func (r *AssetRepo) ListByIssueID(ctx context.Context, issueID uuid.UUID) ([]model.IssueAsset, error) {
	query := fmt.Sprintf(`SELECT %s FROM issue_assets WHERE issue_id = $1 ORDER BY created_at, filename`, assetColumns)
	rows, err := r.db.Query(ctx, query, issueID)
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	defer rows.Close()

	var assets []model.IssueAsset
	for rows.Next() {
		var a model.IssueAsset
		if err := rows.Scan(&a.ID, &a.IssueID, &a.Filename, &a.MimeType, &a.SizeBytes, &a.Checksum, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan asset: %w", err)
		}
		assets = append(assets, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate assets: %w", err)
	}
	return assets, nil
}

// GetByFilename returns the asset metadata, or nil when the issue has no asset
// under that name.
func (r *AssetRepo) GetByFilename(ctx context.Context, issueID uuid.UUID, filename string) (*model.IssueAsset, error) {
	query := fmt.Sprintf(`SELECT %s FROM issue_assets WHERE issue_id = $1 AND filename = $2`, assetColumns)
	asset, err := scanAsset(r.db.QueryRow(ctx, query, issueID, filename))
	if err != nil {
		if err == database.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get asset: %w", err)
	}
	return asset, nil
}

// GetContent returns the bytes of an asset by id.
func (r *AssetRepo) GetContent(ctx context.Context, assetID uuid.UUID) ([]byte, error) {
	return r.store.Get(ctx, r.db, assetID)
}

// Delete removes an asset and its bytes.
func (r *AssetRepo) Delete(ctx context.Context, issueID uuid.UUID, filename string) error {
	asset, err := r.GetByFilename(ctx, issueID, filename)
	if err != nil {
		return err
	}
	if asset == nil {
		return ErrAssetNotFound
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin asset delete: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM issue_assets WHERE id = $1`, asset.ID); err != nil {
		return fmt.Errorf("delete asset: %w", err)
	}
	if err := r.store.Delete(ctx, tx, asset.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
