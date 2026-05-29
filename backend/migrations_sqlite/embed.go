// Package sqlitemigrations embeds the SQLite migration files for the
// standalone (single-binary) build.
package sqlitemigrations

import "embed"

// FS holds the embedded SQLite migration SQL files.
//
//go:embed *.sql
var FS embed.FS
