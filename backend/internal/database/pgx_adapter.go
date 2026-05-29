package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WrapPgx adapts a pgxpool.Pool to the DB interface. Used by the PostgreSQL
// server entrypoint.
func WrapPgx(pool *pgxpool.Pool) DB { return &pgxDB{pool: pool} }

type pgxDB struct{ pool *pgxpool.Pool }

func (d *pgxDB) Query(ctx context.Context, q string, args ...any) (Rows, error) {
	rows, err := d.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return pgxRows{rows}, nil
}

func (d *pgxDB) QueryRow(ctx context.Context, q string, args ...any) Row {
	return pgxRow{d.pool.QueryRow(ctx, q, args...)}
}

func (d *pgxDB) Exec(ctx context.Context, q string, args ...any) (Result, error) {
	ct, err := d.pool.Exec(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return rowsAffected(ct.RowsAffected()), nil
}

func (d *pgxDB) Begin(ctx context.Context) (Tx, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &pgxTx{tx}, nil
}

func (d *pgxDB) Dialect() string { return DialectPostgres }

func (d *pgxDB) Close() { d.pool.Close() }

type pgxTx struct{ tx pgx.Tx }

func (t *pgxTx) Query(ctx context.Context, q string, args ...any) (Rows, error) {
	rows, err := t.tx.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return pgxRows{rows}, nil
}

func (t *pgxTx) QueryRow(ctx context.Context, q string, args ...any) Row {
	return pgxRow{t.tx.QueryRow(ctx, q, args...)}
}

func (t *pgxTx) Exec(ctx context.Context, q string, args ...any) (Result, error) {
	ct, err := t.tx.Exec(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return rowsAffected(ct.RowsAffected()), nil
}

func (t *pgxTx) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *pgxTx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }

type pgxRows struct{ rows pgx.Rows }

func (r pgxRows) Next() bool             { return r.rows.Next() }
func (r pgxRows) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r pgxRows) Close()                 { r.rows.Close() }
func (r pgxRows) Err() error             { return r.rows.Err() }

type pgxRow struct{ row pgx.Row }

func (r pgxRow) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoRows
	}
	return err
}

// rowsAffected implements Result for both adapters.
type rowsAffected int64

func (n rowsAffected) RowsAffected() int64 { return int64(n) }
