// Package testenv reads the environment of the database tests. It is a leaf
// package (it imports nothing of ours), so every test package can use it,
// including the ones dbtest itself imports (db, migrations).
package testenv

import (
	"os"
	"testing"
)

const (
	// DatabaseEnv names the variable with the test server's connection string.
	DatabaseEnv = "MEURPG_TEST_DATABASE_URL"
	// RequireEnv names the variable that, set to "1", turns a missing DatabaseEnv from a skip into a
	// failure. CI sets it on the job that runs the integration tests: if the
	// database URL ever goes missing there, the job must go red instead of
	// skipping about a thousand tests and staying green.
	RequireEnv = "MEURPG_REQUIRE_DB"
)

// DatabaseURL returns the test server's connection string. Without it the
// test is skipped, so `go test ./...` works on a machine with no database;
// with MEURPG_REQUIRE_DB=1 it fails instead.
func DatabaseURL(t testing.TB) string {
	t.Helper()
	url := os.Getenv(DatabaseEnv)
	if url != "" {
		return url
	}
	if os.Getenv(RequireEnv) == "1" {
		t.Fatalf("%s is not set, but %s=1 says the database tests must run", DatabaseEnv, RequireEnv)
	}
	t.Skip(DatabaseEnv + " is not set; skipping database test")
	return ""
}
