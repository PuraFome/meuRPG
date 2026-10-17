package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/PuraFome/meuRPG/backend/internal/platform/config"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
	"github.com/PuraFome/meuRPG/backend/internal/platform/testenv"
	"github.com/PuraFome/meuRPG/backend/migrations"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

// migrateLogLines turns the JSON log of run into one line per record:
// the level, the message and the attributes in order, with the time and the
// static attributes left out and the values that change from run to run
// (durations, times) masked.
func migrateLogLines(t *testing.T, log string) []string {
	t.Helper()
	var lines []string
	sc := bufio.NewScanner(strings.NewReader(log))
	for sc.Scan() {
		dec := json.NewDecoder(strings.NewReader(sc.Text()))
		if _, err := dec.Token(); err != nil {
			t.Fatalf("log line %q: %v", sc.Text(), err)
		}
		var parts []string
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				t.Fatalf("log line %q: %v", sc.Text(), err)
			}
			key, _ := keyTok.(string)
			var value any
			if err := dec.Decode(&value); err != nil {
				t.Fatalf("log line %q: %v", sc.Text(), err)
			}
			switch key {
			case "time", "service", "logging.googleapis.com/sourceLocation":
				continue
			case "duration_ms", "applied_at":
				value = "*"
			}
			parts = append(parts, fmt.Sprintf("%s=%v", key, value))
		}
		lines = append(lines, strings.Join(parts, " "))
	}
	return lines
}

// runMigrate calls run with a logger like main's, writing to a buffer.
func runMigrate(t *testing.T, cfg config.Config, args ...string) ([]string, error) {
	t.Helper()
	var buf strings.Builder
	err := run(logging.New(&buf, slog.LevelDebug), cfg, args)
	return migrateLogLines(t, buf.String()), err
}

// TestRunRefusals checks the refusals that need no database, in order: the
// arguments first, then DATABASE_URL.
func TestRunRefusals(t *testing.T) {
	t.Parallel()
	withURL := config.Config{DatabaseURL: "postgresql://root@127.0.0.1:1/none?sslmode=disable"}
	tests := []struct {
		name string
		cfg  config.Config
		args []string
		want string
	}{
		{"no argument", withURL, nil, "usage: migrate <up|down|status>"},
		{"two arguments", withURL, []string{"up", "down"}, "usage: migrate <up|down|status>"},
		{"no argument and no database", config.Config{}, nil, "usage: migrate <up|down|status>"},
		{"no database", config.Config{}, []string{"up"}, "DATABASE_URL is required"},
		{"no database, unknown command", config.Config{}, []string{"sideways"}, "DATABASE_URL is required"},
		{"broken database URL", config.Config{DatabaseURL: "postgresql://h:99999999/x"}, []string{"status"}, "DATABASE_URL is not a valid PostgreSQL connection string"},
		// The command is checked after the connection is opened (lazily: nothing
		// is sent), so an unknown one needs a valid URL to be named.
		{"unknown command", withURL, []string{"sideways"}, `unknown command "sideways"; usage: migrate <up|down|status>`},
		{"command in upper case", withURL, []string{"UP"}, `unknown command "UP"; usage: migrate <up|down|status>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lines, err := runMigrate(t, tt.cfg, tt.args...)
			if err == nil || err.Error() != tt.want {
				t.Errorf("run() error = %v, want %q", err, tt.want)
			}
			if len(lines) != 0 {
				t.Errorf("run() logged %q", lines)
			}
		})
	}
}

// TestRunOnADatabase checks the log of each command on a migrated database:
// status lists every migration, up with nothing pending says so, down rolls
// the latest back, status shows it pending, and up applies it again.
func TestRunOnADatabase(t *testing.T) {
	t.Parallel()
	pool := dbtest.NewPool(t, "migratecmd")
	cfg := config.Config{DatabaseURL: config.Secret(pool.Config().ConnString())}

	sources := migrationSources(t, pool.Config().ConnString())
	latest := sources[len(sources)-1]
	latestLine := func(prefix string) string {
		return fmt.Sprintf("%sversion=%d file=%s", prefix, latest.Version, latest.Path)
	}

	status := func(lastState string) {
		t.Helper()
		lines, err := runMigrate(t, cfg, "status")
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		if len(lines) != len(sources) {
			t.Fatalf("status logged %d lines for %d migrations", len(lines), len(sources))
		}
		for i, s := range sources {
			state, appliedAt := "applied", " applied_at=*"
			if i == len(sources)-1 && lastState == "pending" {
				state, appliedAt = "pending", ""
			}
			want := fmt.Sprintf("severity=INFO message=migration version=%d file=%s state=%s%s", s.Version, s.Path, state, appliedAt)
			if lines[i] != want {
				t.Fatalf("status line %d = %q, want %q", i, lines[i], want)
			}
		}
	}
	upOrDown := func(command string, want ...string) {
		t.Helper()
		lines, err := runMigrate(t, cfg, command)
		if err != nil {
			t.Fatalf("%s: %v", command, err)
		}
		if !slices.Equal(lines, want) {
			t.Fatalf("%s logged\n%s\nwant\n%s", command, strings.Join(lines, "\n"), strings.Join(want, "\n"))
		}
	}

	status("applied")
	upOrDown("up", "severity=INFO message=database is up to date applied=0")
	upOrDown("down", latestLine("severity=INFO message=migration rolled back "))
	status("pending")
	// A database from dbtest continues goose's ids on a copy of the template's
	// identity sequence, not the one the table uses (a finding of dbtest, see
	// its schemaDDL test), so the next insert would collide: point the table's
	// own sequence past the rows, as on a database migrated from scratch.
	if _, err := pool.Exec(t.Context(), `SELECT setval(pg_get_serial_sequence('goose_db_version', 'id'), (SELECT max(id) FROM goose_db_version))`); err != nil {
		t.Fatal(err)
	}
	upOrDown("up",
		latestLine("severity=INFO message=migration applied ")+" duration_ms=*",
		"severity=INFO message=database is up to date applied=1")
	status("applied")
}

// migrationSource is one embedded migration, as the log names it.
type migrationSource struct {
	Version int64
	Path    string
}

func migrationSources(t *testing.T, dsn string) []migrationSource {
	t.Helper()
	db, err := sql.Open("pgx", dsn) // lazy: listing the sources sends nothing
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	provider, err := migrations.NewProvider(db)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	var out []migrationSource
	for _, s := range provider.ListSources() {
		out = append(out, migrationSource{s.Version, s.Path})
	}
	return out
}

// TestRunErrorsOfTheDatabase checks the errors of each command when the
// database fails it: unreachable (the lock comes first for up and down), or
// empty, where down has nothing to roll back and status lists every migration
// as pending.
func TestRunErrorsOfTheDatabase(t *testing.T) {
	t.Parallel()
	unreachable := config.Config{DatabaseURL: "postgresql://root@127.0.0.1:1/none?sslmode=disable&connect_timeout=5"}
	refused := "failed to connect to `user=root database=none`: 127.0.0.1:1 (127.0.0.1): dial error: dial tcp 127.0.0.1:1: connect: connection refused"
	for command, want := range map[string]string{
		"up":     "lock: create the migration lock table: " + refused,
		"down":   "lock: create the migration lock table: " + refused,
		"status": "status: failed to initialize: " + refused,
	} {
		lines, err := runMigrate(t, unreachable, command)
		if err == nil || err.Error() != want || len(lines) != 0 {
			t.Errorf("%s on an unreachable database: %v, %q; want %q and no log", command, err, lines, want)
		}
	}
	empty := config.Config{DatabaseURL: config.Secret(emptyDatabase(t))}
	if lines, err := runMigrate(t, empty, "down"); err == nil || err.Error() != "down: no next version found" || len(lines) != 0 {
		t.Errorf("down on an empty database: %v, %q", err, lines)
	}
	lines, err := runMigrate(t, empty, "status")
	if err != nil {
		t.Fatal(err)
	}
	sources := migrationSources(t, empty.DatabaseURL.Reveal())
	if len(lines) != len(sources) {
		t.Fatalf("status logged %d lines for %d migrations", len(lines), len(sources))
	}
	for i, s := range sources {
		if want := fmt.Sprintf("severity=INFO message=migration version=%d file=%s state=pending", s.Version, s.Path); lines[i] != want {
			t.Fatalf("status line %d = %q, want %q", i, lines[i], want)
		}
	}
}

// emptyDatabase creates a database with no table and returns its URL. It is
// dropped when the test ends.
func emptyDatabase(t *testing.T) string {
	t.Helper()
	rawURL := testenv.DatabaseURL(t)
	admin, err := sql.Open("pgx", rawURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	name := fmt.Sprintf("meurpg_migratecmd_test_empty_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(t.Context(), "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+name+" CASCADE")
	})
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}
