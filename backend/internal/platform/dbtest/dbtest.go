// Package dbtest gives integration tests a real, freshly migrated
// CockroachDB database. Only tests import it.
//
// The tests need MEURPG_TEST_DATABASE_URL; without it they skip, so
// `go test ./...` works anywhere. To run them, start a throwaway CockroachDB
// (`make up` does, on port 26257) and run:
//
//	MEURPG_TEST_DATABASE_URL='postgresql://root@localhost:26257/defaultdb?sslmode=disable' make test
//
// Migrating a brand-new database for every test took about 25 seconds on
// CockroachDB (each schema change is a job, and Etapa 6 brought 46
// migrations), and the play package alone makes dozens of databases: CI hit
// its 10-minute limit (PR #59). So the migrations run once per set of migration files, into a
// template database named after their fingerprint (meurpg_tpl_<hash>), and
// each test's database copies the template's tables in about 3 seconds. The
// schema is the one the migrations build; the migrations package still tests
// the migrations themselves, from scratch.
//
// Every test process shares the template: the first one to create it
// migrates it, and the others wait until it has every migration. A template
// left half migrated (a run killed midway) is dropped and built again.
// Templates of older migration sets stay on the test server; they are empty
// and safe to drop.
package dbtest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// The template this process clones, built once.
var (
	templateOnce sync.Once
	template     templateDB
	templateErr  error
)

// templateDB is a migrated database and the statements that copy it.
type templateDB struct {
	name string
	// ddl creates the template's sequences and tables, each table after the
	// ones it references.
	ddl []string
}

// Enabled reports whether a test database is configured.
func Enabled() bool { return os.Getenv(EnvVar) != "" }

// NewPool creates a brand-new database on the test server with every
// migration's schema and returns a pool connected to it. The database is
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

	templateOnce.Do(func() { template, templateErr = buildTemplate(rawURL) })
	if templateErr != nil {
		t.Fatalf("build the test database template: %v", templateErr)
	}

	admin := open(t, rawURL)
	name := fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), counter.Add(1))
	mustExec(t, admin, "CREATE DATABASE "+name)
	t.Cleanup(func() { mustExec(t, admin, "DROP DATABASE IF EXISTS "+name+" CASCADE") })

	dsn, err := withDatabase(rawURL, name)
	if err != nil {
		t.Fatalf("parse %s: %v", EnvVar, err)
	}
	conn := open(t, dsn)
	for _, stmt := range template.ddl {
		if _, err := conn.ExecContext(t.Context(), stmt); err != nil {
			t.Fatalf("copy the template's schema: %v\n%s", err, stmt)
		}
	}
	// goose's own table too, so the database says which migrations it has.
	if _, err := conn.ExecContext(t.Context(), fmt.Sprintf(
		`INSERT INTO goose_db_version SELECT * FROM %s.public.goose_db_version`, template.name)); err != nil {
		t.Fatalf("copy the migration versions: %v", err)
	}
	if _, err := conn.ExecContext(t.Context(),
		`SELECT setval('goose_db_version_id_seq', (SELECT max(id) FROM goose_db_version))`); err != nil {
		t.Fatalf("move the migration version sequence: %v", err)
	}

	pool, err := db.NewPool(t.Context(), dsn)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// errStalled says a template has stopped getting migrations: whoever was
// migrating it is gone.
var errStalled = errors.New("the template stopped getting migrations")

// buildTemplate makes sure the template for these migrations exists and is
// complete, and reads the statements that copy it.
func buildTemplate(rawURL string) (templateDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	fingerprint, err := migrations.Fingerprint()
	if err != nil {
		return templateDB{}, fmt.Errorf("fingerprint the migrations: %w", err)
	}
	name := "meurpg_tpl_" + fingerprint
	dsn, err := withDatabase(rawURL, name)
	if err != nil {
		return templateDB{}, err
	}
	admin, err := sql.Open("pgx", rawURL)
	if err != nil {
		return templateDB{}, err
	}
	defer func() { _ = admin.Close() }()
	tpl, err := sql.Open("pgx", dsn)
	if err != nil {
		return templateDB{}, err
	}
	defer func() { _ = tpl.Close() }()
	provider, err := migrations.NewProvider(tpl)
	if err != nil {
		return templateDB{}, err
	}
	sources := provider.ListSources()
	latest := sources[len(sources)-1].Version

	for {
		_, err := admin.ExecContext(ctx, "CREATE DATABASE "+name)
		if err == nil {
			// This process creates it, so it migrates it.
			if _, err := provider.Up(ctx); err != nil {
				return templateDB{}, fmt.Errorf("migrate the template: %w", err)
			}
			break
		}
		if !isCode(err, "42P04") { // duplicate_database
			return templateDB{}, fmt.Errorf("create the template: %w", err)
		}
		// Another process made it: wait until it is complete.
		err = waitForVersion(ctx, tpl, latest)
		if errors.Is(err, errStalled) {
			if _, err := admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+name+" CASCADE"); err != nil {
				return templateDB{}, fmt.Errorf("drop a stalled template: %w", err)
			}
			continue
		}
		if err != nil {
			return templateDB{}, err
		}
		break
	}

	ddl, err := schemaDDL(ctx, tpl)
	if err != nil {
		return templateDB{}, err
	}
	return templateDB{name: name, ddl: ddl}, nil
}

// waitForVersion waits until the template has the latest migration. It
// returns errStalled when no migration lands for a minute and a half (one
// takes a few seconds), so a run killed while migrating does not leave the
// next ones waiting forever.
func waitForVersion(ctx context.Context, tpl *sql.DB, latest int64) error {
	last, lastChange := int64(-1), time.Now()
	for {
		var version int64
		err := tpl.QueryRowContext(ctx, `SELECT coalesce(max(version_id), 0) FROM goose_db_version`).Scan(&version)
		if isCode(err, "42P01") { // undefined_table: goose has not started yet
			err, version = nil, 0
		}
		if err != nil {
			return fmt.Errorf("read the template's version: %w", err)
		}
		if version >= latest {
			return nil
		}
		if version != last {
			last, lastChange = version, time.Now()
		} else if time.Since(lastChange) > 90*time.Second {
			return errStalled
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// references finds the tables a CREATE TABLE statement points at.
var references = regexp.MustCompile(`REFERENCES public\.(\w+)`)

// schemaDDL returns the statements that recreate the template's sequences
// and tables, with their indexes, CHECKs, foreign keys and options inline,
// as CockroachDB writes them (SHOW CREATE). A table comes after the ones it
// references, so every foreign key finds its table: creating a table with
// its foreign keys inline is quick, where adding them afterwards, as a
// migration does, costs a schema change job each.
func schemaDDL(ctx context.Context, tpl *sql.DB) ([]string, error) {
	sequences, err := names(ctx, tpl, `SELECT sequence_name FROM information_schema.sequences WHERE sequence_schema = 'public' ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	tables, err := names(ctx, tpl, `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE' ORDER BY 1`)
	if err != nil {
		return nil, err
	}

	var ddl []string
	for _, s := range sequences {
		stmt, err := showCreate(ctx, tpl, "SEQUENCE", s)
		if err != nil {
			return nil, err
		}
		ddl = append(ddl, stmt)
	}
	create := make(map[string]string, len(tables))
	for _, table := range tables {
		if create[table], err = showCreate(ctx, tpl, "TABLE", table); err != nil {
			return nil, err
		}
	}
	// Each round adds the tables whose references are all created.
	done := map[string]bool{}
	for len(done) < len(tables) {
		progress := false
		for _, table := range tables {
			if done[table] {
				continue
			}
			ready := true
			for _, m := range references.FindAllStringSubmatch(create[table], -1) {
				if ref := m[1]; ref != table && !done[ref] {
					ready = false
				}
			}
			if ready {
				ddl, done[table], progress = append(ddl, create[table]), true, true
			}
		}
		if !progress {
			left := slices.DeleteFunc(slices.Clone(tables), func(t string) bool { return done[t] })
			return nil, fmt.Errorf("foreign keys in a cycle among %s", strings.Join(left, ", "))
		}
	}
	return ddl, nil
}

func names(ctx context.Context, conn *sql.DB, query string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list the template's objects: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// showCreate asks CockroachDB how to create the template's sequence or table
// name. The name comes from the template's own catalog; it is quoted anyway.
func showCreate(ctx context.Context, conn *sql.DB, kind, name string) (string, error) {
	var object, stmt string
	query := "SHOW CREATE " + kind + " " + pgx.Identifier{"public", name}.Sanitize()
	//nolint:gosec // a quoted identifier read from the template's own catalog, in test code
	if err := conn.QueryRowContext(ctx, query).Scan(&object, &stmt); err != nil {
		return "", fmt.Errorf("show create %s %s: %w", kind, name, err)
	}
	return stmt, nil
}

func isCode(err error, code string) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == code
}

// withDatabase points a connection string at another database.
func withDatabase(rawURL, name string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	u.Path = "/" + name
	return u.String(), nil
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
