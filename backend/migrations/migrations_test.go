package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"slices"
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

	wantTables := []string{"auth_sessions", "character_join_tokens", "characters", "maps", "oauth_handshakes", "users"}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	if got := tables(t, db); !slices.Equal(got, wantTables) {
		t.Errorf("tables after Up = %v, want %v", got, wantTables)
	}

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
