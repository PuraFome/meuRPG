// Package dbtest gives integration tests a real, freshly migrated
// CockroachDB database. Only tests import it.
//
// The tests need MEURPG_TEST_DATABASE_URL; without it they skip, so
// `go test ./...` works anywhere. To run them, start a throwaway CockroachDB
// (`make up` does, on port 26257) and run:
//
//	MEURPG_TEST_DATABASE_URL='postgresql://root@localhost:26257/defaultdb?sslmode=disable' make test
package dbtest

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // the "pgx" driver for database/sql, which goose needs

	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/migrations"
)

// EnvVar names the variable with the test server's connection string.
const EnvVar = "MEURPG_TEST_DATABASE_URL"

// counter makes database names unique even when two tests start in the same
// nanosecond.
var counter atomic.Int64

// Enabled reports whether a test database is configured.
func Enabled() bool { return os.Getenv(EnvVar) != "" }

// NewPool creates a brand-new database on the test server, applies every
// migration to it and returns a pool connected to it. The database is
// dropped when the test ends, so tests can run in parallel without seeing
// each other's rows. prefix names the database, to tell tests apart in
// CockroachDB's console.
//
// Without MEURPG_TEST_DATABASE_URL, NewPool skips the test.
func NewPool(t testing.TB, prefix string) *pgxpool.Pool {
	t.Helper()
	rawURL := os.Getenv(EnvVar)
	if rawURL == "" {
		t.Skip(EnvVar + " is not set; skipping database test")
	}

	admin := open(t, rawURL)
	name := fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), counter.Add(1))
	mustExec(t, admin, "CREATE DATABASE "+name)
	t.Cleanup(func() { mustExec(t, admin, "DROP DATABASE IF EXISTS "+name+" CASCADE") })

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse %s: %v", EnvVar, err)
	}
	u.Path = "/" + name

	provider, err := migrations.NewProvider(open(t, u.String()))
	if err != nil {
		t.Fatalf("migrations.NewProvider() error = %v", err)
	}
	if _, err := provider.Up(t.Context()); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	pool, err := db.NewPool(t.Context(), u.String())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func open(t testing.TB, dsn string) *sql.DB {
	t.Helper()
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func mustExec(t testing.TB, conn *sql.DB, query string) {
	t.Helper()
	// Not t.Context(): cleanups run after it is canceled.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := conn.ExecContext(ctx, query); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
