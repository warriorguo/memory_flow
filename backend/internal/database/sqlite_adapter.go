package database

import (
	"context"
	"database/sql"
	"errors"
)

// sqliteDB adapts a *sql.DB (modernc.org/sqlite driver) to the DB interface,
// rewriting each query into SQLite dialect before execution.
type sqliteDB struct{ db *sql.DB }

// WrapSQLite adapts an already-open *sql.DB to the DB interface.
func WrapSQLite(db *sql.DB) DB { return &sqliteDB{db: db} }

// SQL returns the underlying *sql.DB (used to run migrations).
func (d *sqliteDB) SQL() *sql.DB { return d.db }

func (d *sqliteDB) Query(ctx context.Context, q string, args ...any) (Rows, error) {
	q, args = rewriteSQLite(q, args)
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return &sqlRows{rows: rows}, nil
}

func (d *sqliteDB) QueryRow(ctx context.Context, q string, args ...any) Row {
	q, args = rewriteSQLite(q, args)
	return sqlRow{d.db.QueryRowContext(ctx, q, args...)}
}

func (d *sqliteDB) Exec(ctx context.Context, q string, args ...any) (Result, error) {
	q, args = rewriteSQLite(q, args)
	res, err := d.db.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	return rowsAffected(n), nil
}

func (d *sqliteDB) Begin(ctx context.Context) (Tx, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &sqlTx{tx}, nil
}

func (d *sqliteDB) Dialect() string { return DialectSQLite }

func (d *sqliteDB) Close() { _ = d.db.Close() }

type sqlTx struct{ tx *sql.Tx }

func (t *sqlTx) Query(ctx context.Context, q string, args ...any) (Rows, error) {
	q, args = rewriteSQLite(q, args)
	rows, err := t.tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return &sqlRows{rows: rows}, nil
}

func (t *sqlTx) QueryRow(ctx context.Context, q string, args ...any) Row {
	q, args = rewriteSQLite(q, args)
	return sqlRow{t.tx.QueryRowContext(ctx, q, args...)}
}

func (t *sqlTx) Exec(ctx context.Context, q string, args ...any) (Result, error) {
	q, args = rewriteSQLite(q, args)
	res, err := t.tx.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	return rowsAffected(n), nil
}

func (t *sqlTx) Commit(ctx context.Context) error   { return t.tx.Commit() }
func (t *sqlTx) Rollback(ctx context.Context) error { return t.tx.Rollback() }

type sqlRows struct {
	rows *sql.Rows
	err  error
}

func (r *sqlRows) Next() bool             { return r.rows.Next() }
func (r *sqlRows) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *sqlRows) Close()                 { _ = r.rows.Close() }
func (r *sqlRows) Err() error {
	if r.err != nil {
		return r.err
	}
	return r.rows.Err()
}

type sqlRow struct{ row *sql.Row }

func (r sqlRow) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoRows
	}
	return err
}
