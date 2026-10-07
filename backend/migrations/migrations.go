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
//
// TestMigrationsAreSafeToRerun checks this: it runs every Up twice.
//
// The numbers 00118 and 00119 do not exist (see docs/data.md): goose accepts
// gaps, and no new migration may take them.
//
// goose has no lock for CockroachDB, so cmd/migrate takes a lease lock before
// `up` and `down` (lock.go): a second run waits for the first.
//
// sqlc reads these files too (backend/sqlc.yaml), with a PostgreSQL parser,
// to learn the schema. So write SQL that both PostgreSQL and CockroachDB
// accept: an index gets its own CREATE INDEX IF NOT EXISTS migration instead
// of an INDEX line inside CREATE TABLE, and covering columns are INCLUDE
// (CockroachDB's STORING). CockroachDB-only table options in WITH (...),
// such as row-level TTL, are fine.
package migrations

import (
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"slices"

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

// Fingerprint is a short hash of every migration file, names and contents.
// Two databases built from the same fingerprint have the same schema;
// internal/platform/dbtest names the template database it builds for the
// tests after it, so a new or changed migration gets a new template.
func Fingerprint() (string, error) {
	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return "", err
	}
	slices.Sort(names)
	h := sha256.New()
	for _, name := range names {
		body, err := files.ReadFile(name)
		if err != nil {
			return "", err
		}
		// The name and the length go in first, so moving bytes from one
		// file to the next changes the hash.
		fmt.Fprintf(h, "%s\x00%d\x00", name, len(body))
		h.Write(body)
	}
	return hex.EncodeToString(h.Sum(nil))[:12], nil
}
