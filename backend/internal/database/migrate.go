package database

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	sqlitemigrate "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// MigrateSQLite runs the embedded SQLite migrations against the given *sql.DB.
// srcFS is the filesystem containing the *.sql migration files (e.g. the
// embedded sqlitemigrations.FS).
func MigrateSQLite(db *sql.DB, srcFS fs.FS) error {
	src, err := iofs.New(srcFS, ".")
	if err != nil {
		return fmt.Errorf("open migration source: %w", err)
	}

	driver, err := sqlitemigrate.WithInstance(db, &sqlitemigrate.Config{})
	if err != nil {
		return fmt.Errorf("create sqlite migrate driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run sqlite migrations: %w", err)
	}
	return nil
}
