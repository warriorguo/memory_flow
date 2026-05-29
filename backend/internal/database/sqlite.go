package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// SQLiteDSN builds the connection string for a SQLite database file, enabling
// foreign-key enforcement (off by default in SQLite, but our schema relies on
// ON DELETE CASCADE) and a busy timeout.
func SQLiteDSN(path string) string {
	return fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", path)
}

// OpenSQLiteDB opens a raw *sql.DB against the SQLite file. Callers that need
// the DB abstraction should use NewSQLite; the raw handle is exposed for
// running migrations.
func OpenSQLiteDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", SQLiteDSN(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite is a single-writer engine; constrain the pool to avoid SQLITE_BUSY
	// under concurrent writes for the single-user standalone workload.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return db, nil
}

// NewSQLite opens a SQLite-backed DB at the given file path.
func NewSQLite(path string) (DB, error) {
	db, err := OpenSQLiteDB(path)
	if err != nil {
		return nil, err
	}
	return WrapSQLite(db), nil
}
