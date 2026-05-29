package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRewriteSQLite(t *testing.T) {
	cases := []struct {
		name     string
		query    string
		args     []any
		wantSQL  string
		wantArgs []any
	}{
		{
			name:     "sequential placeholders",
			query:    "SELECT * FROM t WHERE a = $1 AND b = $2",
			args:     []any{"x", "y"},
			wantSQL:  "SELECT * FROM t WHERE a = ? AND b = ?",
			wantArgs: []any{"x", "y"},
		},
		{
			name:     "reused placeholder with ILIKE",
			query:    "WHERE (title ILIKE $1 OR description ILIKE $1) AND status = $2",
			args:     []any{"kw", "open"},
			wantSQL:  "WHERE (title LIKE ? OR description LIKE ?) AND status = ?",
			wantArgs: []any{"kw", "kw", "open"},
		},
		{
			name:     "now() rewrite",
			query:    "UPDATE t SET updated_at = now() WHERE id = $1",
			args:     []any{"id"},
			wantSQL:  "UPDATE t SET updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			wantArgs: []any{"id"},
		},
		{
			name:     "limit offset",
			query:    "SELECT a FROM t ORDER BY a LIMIT $1 OFFSET $2",
			args:     []any{10, 20},
			wantSQL:  "SELECT a FROM t ORDER BY a LIMIT ? OFFSET ?",
			wantArgs: []any{10, 20},
		},
		{
			name:     "literal with dollar untouched",
			query:    "SELECT '$1 literal' , col FROM t WHERE id = $1",
			args:     []any{"id"},
			wantSQL:  "SELECT '$1 literal' , col FROM t WHERE id = ?",
			wantArgs: []any{"id"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotSQL, gotArgs := rewriteSQLite(tc.query, tc.args)
			if gotSQL != tc.wantSQL {
				t.Errorf("sql:\n got: %q\nwant: %q", gotSQL, tc.wantSQL)
			}
			if len(gotArgs) != len(tc.wantArgs) {
				t.Fatalf("args len got %d want %d (%v)", len(gotArgs), len(tc.wantArgs), gotArgs)
			}
			for i := range gotArgs {
				if gotArgs[i] != tc.wantArgs[i] {
					t.Errorf("arg[%d] got %v want %v", i, gotArgs[i], tc.wantArgs[i])
				}
			}
		})
	}
}

// TestSQLiteRoundTrip validates the load-bearing assumptions: window functions,
// RETURNING, time.Time bind/scan against DATETIME columns, and ErrNoRows.
func TestSQLiteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	db, err := NewSQLite(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	_, err = db.Exec(ctx, `CREATE TABLE t (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT now()
	)`)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	id := uuid.New()
	var gotID uuid.UUID
	var created, updated time.Time
	// RETURNING + Go-supplied uuid + DB-defaulted timestamps.
	row := db.QueryRow(ctx,
		`INSERT INTO t (id, name) VALUES ($1, $2) RETURNING id, created_at, updated_at`,
		id, "hello")
	if err := row.Scan(&gotID, &created, &updated); err != nil {
		t.Fatalf("insert returning scan: %v", err)
	}
	if gotID != id {
		t.Errorf("id got %v want %v", gotID, id)
	}
	if created.IsZero() || updated.IsZero() {
		t.Errorf("timestamps not parsed to time.Time: created=%v updated=%v", created, updated)
	}

	// Bind a time.Time and scan it back.
	bound := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if _, err := db.Exec(ctx, `UPDATE t SET updated_at = $1 WHERE id = $2`, bound, id); err != nil {
		t.Fatalf("update time: %v", err)
	}
	var back time.Time
	if err := db.QueryRow(ctx, `SELECT updated_at FROM t WHERE id = $1`, id).Scan(&back); err != nil {
		t.Fatalf("scan time: %v", err)
	}
	if !back.Equal(bound) {
		t.Errorf("time round-trip got %v want %v", back, bound)
	}

	// Window function COUNT(*) OVER().
	var total int
	var name string
	rows, err := db.Query(ctx, `SELECT name, COUNT(*) OVER() AS total FROM t`)
	if err != nil {
		t.Fatalf("window query: %v", err)
	}
	for rows.Next() {
		if err := rows.Scan(&name, &total); err != nil {
			t.Fatalf("window scan: %v", err)
		}
	}
	rows.Close()
	if total != 1 {
		t.Errorf("window count got %d want 1", total)
	}

	// ErrNoRows normalization.
	err = db.QueryRow(ctx, `SELECT name FROM t WHERE id = $1`, uuid.New()).Scan(&name)
	if err != ErrNoRows {
		t.Errorf("missing row err got %v want ErrNoRows", err)
	}
}
