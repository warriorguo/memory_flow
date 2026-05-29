package database

import (
	"context"
	"errors"
)

// ErrNoRows is returned by Row.Scan / QueryRow when no row exists. Both the pgx
// and database/sql adapters normalize their native "no rows" sentinel to this
// value so repositories can compare against a single error regardless of the
// underlying driver.
var ErrNoRows = errors.New("database: no rows in result set")

// DB is the connection-pool abstraction used by all repositories. It is
// implemented by a pgx adapter (PostgreSQL server) and a database/sql adapter
// backed by modernc.org/sqlite (standalone binary).
type DB interface {
	Querier
	Begin(ctx context.Context) (Tx, error)
	// Dialect reports the backend: DialectPostgres or DialectSQLite.
	Dialect() string
	Close()
}

// Backend dialect identifiers returned by DB.Dialect.
const (
	DialectPostgres = "postgres"
	DialectSQLite   = "sqlite"
)

// Querier is the set of query methods shared by DB and Tx. The method shapes
// mirror pgx so existing repository call sites need no changes.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
	Exec(ctx context.Context, sql string, args ...any) (Result, error)
}

// Tx is a database transaction. Commit/Rollback take a context to match pgx's
// signature (the database/sql adapter ignores the context value).
type Tx interface {
	Querier
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Rows is an iterator over a multi-row result. Close() returns nothing to match
// how repositories already call `defer rows.Close()`.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

// Row is a single-row result. Scan returns ErrNoRows when the result is empty.
type Row interface {
	Scan(dest ...any) error
}

// Result reports how many rows an Exec affected.
type Result interface {
	RowsAffected() int64
}
