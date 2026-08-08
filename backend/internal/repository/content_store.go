package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/database"
)

// ContentStore persists the bytes of an asset, keyed by the asset's id. It is
// the seam that lets the blob backend change without touching the metadata
// schema: the default implementation keeps the bytes in the issue_assets row
// itself, and an S3/MinIO implementation would write them elsewhere and leave
// the column NULL.
//
// Every method takes the [database.Querier] the caller is working through, so
// the default store can join the caller's transaction and commit bytes and
// metadata together. A remote store ignores it.
type ContentStore interface {
	Put(ctx context.Context, q database.Querier, assetID uuid.UUID, content []byte) error
	Get(ctx context.Context, q database.Querier, assetID uuid.UUID) ([]byte, error)
	Delete(ctx context.Context, q database.Querier, assetID uuid.UUID) error
}

// DBContentStore keeps asset bytes in the issue_assets.content column.
//
// Blobs in the database are the default because the server pod has no
// persistent volume — files written to its filesystem vanish on redeploy —
// and because it keeps the Postgres server and the SQLite standalone on one
// code path. The per-file size cap enforced above this layer is what keeps the
// tradeoff honest.
type DBContentStore struct{}

func NewDBContentStore() *DBContentStore { return &DBContentStore{} }

func (s *DBContentStore) Put(ctx context.Context, q database.Querier, assetID uuid.UUID, content []byte) error {
	_, err := q.Exec(ctx, `UPDATE issue_assets SET content = $1 WHERE id = $2`, content, assetID)
	if err != nil {
		return fmt.Errorf("store asset content: %w", err)
	}
	return nil
}

func (s *DBContentStore) Get(ctx context.Context, q database.Querier, assetID uuid.UUID) ([]byte, error) {
	var content []byte
	err := q.QueryRow(ctx, `SELECT content FROM issue_assets WHERE id = $1`, assetID).Scan(&content)
	if err != nil {
		if err == database.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("load asset content: %w", err)
	}
	return content, nil
}

// Delete is a no-op for the DB store: the bytes live in the metadata row, so
// deleting the row already removes them.
func (s *DBContentStore) Delete(ctx context.Context, q database.Querier, assetID uuid.UUID) error {
	return nil
}
