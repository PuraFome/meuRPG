// Package migrations holds the database schema history as numbered SQL files
// and embeds them into the binary, so the migrate command ships as a single
// file with no directory to copy around.
//
// Files are named NNNNN_description.sql and use goose annotations
// (-- +goose Up / -- +goose Down). Add a new file for every change; never
// edit one that has already been applied in production.
//
// A CockroachDB caveat: goose runs each migration inside a transaction, but
// CockroachDB (autocommit_before_ddl, on by default) commits the open
// transaction before every schema change (CREATE, ALTER, DROP...). So a
// migration with several DDL statements is NOT all-or-nothing: if the third
// statement fails, the first two stay applied, and goose does not record the
// version. To make re-running always safe:
//   - write statements idempotently (IF NOT EXISTS / IF EXISTS);
//   - prefer small migrations, ideally one schema change each.
package migrations

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var files embed.FS

// NewProvider returns a goose migration provider for the embedded files.
//
// CockroachDB speaks the PostgreSQL wire protocol and SQL dialect, so goose's
// Postgres dialect works for it. goose tracks applied versions in the
// goose_db_version table.
func NewProvider(db *sql.DB, opts ...goose.ProviderOption) (*goose.Provider, error) {
	return goose.NewProvider(goose.DialectPostgres, db, files, opts...)
}
