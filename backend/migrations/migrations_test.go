package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestMigrationsUpDownUp applies every migration to a brand-new database,
// rolls them all back, and applies them again. That proves each Down really
// undoes its Up, and that Up is safe to repeat.
//
// It needs MEURPG_TEST_DATABASE_URL (see internal/platform/db for how to
// start a local CockroachDB) and creates, then drops, its own database.
func TestMigrationsUpDownUp(t *testing.T) {
	t.Parallel()
	db := freshDatabase(t)
	ctx := t.Context()

	provider, err := NewProvider(db)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	// Whatever the migrations create today; the checks below don't need a
	// hard-coded list, so adding a migration never requires editing this test.
	created := tables(t, db)

	// Running Up again with nothing pending must be a no-op.
	if results, err := provider.Up(ctx); err != nil || len(results) != 0 {
		t.Fatalf("second Up() = %d results, %v; want 0, nil", len(results), err)
	}

	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("DownTo(0) error = %v", err)
	}
	if got := tables(t, db); len(got) != 0 {
		t.Errorf("tables after Down = %v, want none", got)
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("Up() after Down error = %v", err)
	}
	if got := tables(t, db); !slices.Equal(got, created) {
		t.Errorf("tables after Down then Up = %v, want %v", got, created)
	}
	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		t.Fatalf("GetDBVersion() error = %v", err)
	}
	if sources := provider.ListSources(); version != sources[len(sources)-1].Version {
		t.Errorf("database version = %d, want the latest migration %d", version, sources[len(sources)-1].Version)
	}
}

// freshDatabase creates an empty database on the test server and returns a
// connection to it. The database is dropped when the test ends.
func freshDatabase(t *testing.T) *sql.DB {
	t.Helper()

	rawURL := os.Getenv("MEURPG_TEST_DATABASE_URL")
	if rawURL == "" {
		t.Skip("MEURPG_TEST_DATABASE_URL is not set; skipping database test")
	}

	admin := open(t, rawURL)
	name := fmt.Sprintf("meurpg_migrations_test_%d", time.Now().UnixNano())
	exec(t, admin, "CREATE DATABASE "+name)
	t.Cleanup(func() { exec(t, admin, "DROP DATABASE IF EXISTS "+name+" CASCADE") })

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse MEURPG_TEST_DATABASE_URL: %v", err)
	}
	u.Path = "/" + name
	return open(t, u.String())
}

func open(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func exec(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, query); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// tables lists the application's tables, leaving out goose's own.
func tables(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		  AND table_name <> 'goose_db_version'
		ORDER BY table_name`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("list tables: %v", err)
	}
	return names
}

// TestMigrationsAreSafeToRerun runs every migration's Up a second time over
// a migrated database, as happens when a migration fails halfway and is run
// again (CockroachDB commits before each DDL statement, see migrations.go).
// The second run must succeed and leave the schema exactly as it was: no
// "already exists" error and no duplicated constraint.
func TestMigrationsAreSafeToRerun(t *testing.T) {
	t.Parallel()
	db := freshDatabase(t)
	ctx := t.Context()

	provider, err := NewProvider(db)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	before := schema(t, db)

	// Make goose forget every migration, so Up applies them all again.
	exec(t, db, "DELETE FROM goose_db_version WHERE version_id > 0")
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("second Up() error = %v", err)
	}
	if after := schema(t, db); after != before {
		t.Errorf("schema changed when the migrations ran again.\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// schema returns the CREATE statements of every table, goose's excluded.
func schema(t *testing.T, db *sql.DB) string {
	t.Helper()
	var b strings.Builder
	for _, table := range tables(t, db) {
		var name, create string
		if err := db.QueryRowContext(t.Context(), "SHOW CREATE TABLE "+table).Scan(&name, &create); err != nil {
			t.Fatalf("SHOW CREATE TABLE %s: %v", table, err)
		}
		b.WriteString(create + "\n")
	}
	return b.String()
}
