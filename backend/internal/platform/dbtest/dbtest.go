// Package dbtest gives integration tests a real, freshly migrated
// CockroachDB database. Only tests import it.
//
// The tests need MEURPG_TEST_DATABASE_URL; without it they skip (or fail, with
// MEURPG_REQUIRE_DB=1, which CI sets), so
// `go test ./...` works anywhere. To run them, start a throwaway CockroachDB
// (`make up` does, on port 26257) and run:
//
//	MEURPG_TEST_DATABASE_URL='postgresql://root@localhost:26257/defaultdb?sslmode=disable' make test
//
// Migrating a brand-new database for every test took about 25 seconds on
// CockroachDB (each schema change is a job, and Etapa 6 brought 46
// migrations), and the play package alone makes dozens of databases: CI hit
// its 10-minute limit (PR #59). So the migrations run once per set of
// migration files, into a template database named after their fingerprint
// (meurpg_tpl_<hash>), and a test database copies the template's tables.
// Even that copy took about 5 seconds, and dropping the database 1.5 more
// (04/10/2026, 32 tables), for each of some 400 tests; so a package makes
// only as many databases as it runs tests at once and reuses them: emptying
// one with DELETE takes about 10 milliseconds. The schema is the one the
// migrations build; the migrations package still tests the migrations
// themselves, from scratch.
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
	"runtime"
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
	"github.com/PuraFome/meuRPG/backend/internal/platform/testenv"
	"github.com/PuraFome/meuRPG/backend/migrations"
)

// EnvVar names the variable with the test server's connection string.
const EnvVar = testenv.DatabaseEnv

// counter makes database names unique even when two tests start in the same
// nanosecond.
var counter atomic.Int64

// The template this process clones, built once.
var (
	templateOnce sync.Once
	template     templateDB
	templateErr  error
)

// The databases this process made, and the ones free for the next test: a
// test that ends gives its database back, and the next test with the same
// prefix empties it (DELETE, a few milliseconds) instead of building a new
// one (CREATE DATABASE and every table, seconds on CockroachDB).
var (
	freeMu sync.Mutex
	free   = map[string][]string{}
	made   []string
)

// templateDB is a migrated database and the statements that copy it.
type templateDB struct {
	name string
	// tables are the template's tables, each after the ones it references;
	// emptying a database deletes them in reverse.
	tables []string
	// ddl creates the template's sequences and tables, each table after the
	// ones it references.
	ddl []string
}

// Enabled reports whether a test database is configured.
func Enabled() bool { return os.Getenv(EnvVar) != "" }

// NewPool gives the test an empty database on the test server with every
// migration's schema and returns a pool connected to it. The database is
// the test's alone while it runs, so tests can run in parallel without
// seeing each other's rows; when the test ends it goes back to the package,
// and the next test with the same prefix empties it and uses it again (the
// package's TestMain drops them all at the end, see Main). prefix names the
// databases, to tell packages apart in CockroachDB's console.
//
// Without MEURPG_TEST_DATABASE_URL, NewPool skips the test (it fails the test
// when MEURPG_REQUIRE_DB=1, which CI sets: see package testenv).
//
// The pool has one connection (see guardPool). A test that races transactions
// needs them to overlap: it asks for a bigger pool with NewPoolConns.
func NewPool(t testing.TB, prefix string) *pgxpool.Pool {
	t.Helper()
	wantedMu.Lock()
	conns, ok := wanted[t]
	wantedMu.Unlock()
	if !ok {
		conns = 1
	}
	return NewPoolConns(t, prefix, conns)
}

// The pool sizes tests asked for with PoolSize.
var (
	wantedMu sync.Mutex
	wanted   = map[testing.TB]int32{}
)

// PoolSize makes the pool this test gets from NewPool (through any harness) have
// conns connections instead of one. Call it first in a test that races
// transactions, before the harness is built, with conns at least the number of
// racers: on one connection they would run one after the other and prove
// nothing. The guard against nested acquisitions stays on. It works with
// t.Parallel, which the process-wide MEURPG_TEST_POOL_MAX_CONNS does not (t.Setenv
// forbids it).
func PoolSize(t testing.TB, conns int32) {
	t.Helper()
	wantedMu.Lock()
	wanted[t] = conns
	wantedMu.Unlock()
	t.Cleanup(func() {
		wantedMu.Lock()
		delete(wanted, t)
		wantedMu.Unlock()
	})
}

// NewPoolConns is NewPool with a pool of conns connections, for a test that
// races transactions (ten people accepting the last use of an invite): with
// one connection they would run one after the other and prove nothing. Pick
// conns at least as big as the number of racers. The guard against nested
// acquisitions stays on at any size; MEURPG_TEST_POOL_MAX_CONNS, which is
// process-wide, only changes the default of NewPool.
func NewPoolConns(t testing.TB, prefix string, conns int32) *pgxpool.Pool {
	t.Helper()
	rawURL := testenv.DatabaseURL(t)

	templateOnce.Do(func() { template, templateErr = buildTemplate(rawURL) })
	if templateErr != nil {
		t.Fatalf("build the test database template: %v", templateErr)
	}

	name, reused := take(prefix)
	if !reused {
		name = fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), counter.Add(1))
	}
	dsn, err := withDatabase(rawURL, name)
	if err != nil {
		t.Fatalf("parse %s: %v", EnvVar, err)
	}
	if reused {
		conn := open(t, dsn)
		if _, err := conn.ExecContext(t.Context(), template.emptyAll()); err != nil {
			t.Fatalf("empty the reused test database %s: %v", name, err)
		}
		return poolFor(t, prefix, name, dsn, conns)
	}
	admin := open(t, rawURL)
	mustExec(t, admin, "CREATE DATABASE "+name)
	freeMu.Lock()
	made = append(made, name)
	freeMu.Unlock()

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

	return poolFor(t, prefix, name, dsn, conns)
}

// Main runs a package's tests and then drops the databases this process
// made. Every package that uses NewPool calls it from its TestMain:
//
//	func TestMain(m *testing.M) { dbtest.Main(m) }
//
// Without it the databases stay on the test server after the run (harmless,
// but they pile up; they can be dropped by hand, see CONTRIBUTING.md).
func Main(m *testing.M) {
	code := m.Run()
	if url := os.Getenv(EnvVar); url != "" && len(made) > 0 {
		dropAll(url)
	}
	os.Exit(code)
}

// dropAll drops every database this process made, in one statement each, and
// gives up quietly on a server too busy to answer: the run is over.
func dropAll(rawURL string) {
	conn, err := sql.Open("pgx", rawURL)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()
	for _, name := range made {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, _ = conn.ExecContext(ctx, "DROP DATABASE IF EXISTS "+name+" CASCADE")
		cancel()
	}
}

// poolFor connects to a test database and gives it back when the test ends:
// the pool closes first (cleanups run last-in, first-out), then the database
// is free for the next test with this prefix.
func poolFor(t testing.TB, prefix, name, dsn string, conns int32) *pgxpool.Pool {
	t.Helper()
	t.Cleanup(func() {
		freeMu.Lock()
		free[prefix] = append(free[prefix], name)
		freeMu.Unlock()
	})
	pool, err := db.NewPoolWith(t.Context(), dsn, guardPool(t, conns))
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Guard against nested pool acquisition: a request that takes a second pool
// connection while its transaction holds the first.
//
// A db.InTx closure that reads through the pool instead of the transaction
// (a cross-module call with no tx, a pool-bound s.queries) takes a second
// connection while the transaction keeps the first. With the pool's small
// size and a few concurrent requests, every connection ends up held by a
// transaction waiting for another one: a deadlock that lasts until the
// requests' contexts end (PR #120: CombatService.MoveCombatant waited 176 s).
// Alone, in a test, the same code works, which is why it slipped through.
// Two guards, on every test pool:
//
//   - The pool tracer looks at the stack of each Acquire. One made from inside
//     a db.InTx closure is a bug whatever the pool's size: the test fails at
//     once with the stack, which names the path, and the Acquire itself is
//     canceled (errNestedAcquire), so nothing waits.
//   - The pool has ONE connection (MaxConns = 1; NewPoolConns asks for more),
//     so a nested acquisition that the stack check cannot see (another goroutine
//     started by the closure) deadlocks, and after nestedAcquireTimeout a panic
//     prints the waiting goroutine's stack. That panic ends the whole package's
//     test binary, not just the test.
//
// MEURPG_TEST_POOL_MAX_CONNS overrides the size, for a test that really needs
// several connections at once (or to list every violation in one run instead
// of stopping at the first deadlock).
const poolMaxConnsEnv = "MEURPG_TEST_POOL_MAX_CONNS"

// nestedAcquireTimeout is how long an Acquire may wait before it is called a
// deadlock. It is the second line of defense, for the nested acquisition that
// the stack check cannot see, so it is long: on a one-connection pool the
// requests of a test queue for the connection, and under the race detector and
// a busy test server a queue of them waits whole seconds (a request builds its
// answer from dozens of queries).
const nestedAcquireTimeout = 90 * time.Second

// inTxClosure is the frame of the function crdbpgxv5.ExecuteTx calls with the
// transaction: db.InTx's fn runs under it, and nothing else does.
//
// The name is cockroach-go's: if a new version renames it, the guard goes
// blind and TestGuardCatchesANestedAcquisition (it needs the database, so it
// runs in CI's go-db job) fails.
const inTxClosure = "crdbpgxv5.ExecuteTx.func1"

// errNestedAcquire is the cause of the canceled Acquire of a nested read.
var errNestedAcquire = errors.New("dbtest: nested pool acquisition inside a db.InTx closure")

// guardPool is the tweak NewPoolWith applies to every test pool.
func guardPool(t testing.TB, conns int32) func(*pgxpool.Config) {
	return func(cfg *pgxpool.Config) {
		cfg.MaxConns = conns
		if v := os.Getenv(poolMaxConnsEnv); v != "" && conns == 1 {
			var n int32
			if _, err := fmt.Sscan(v, &n); err == nil && n > 0 {
				cfg.MaxConns = n
			}
		}
		w := &acquireWatch{t: t}
		t.Cleanup(w.finish)
		cfg.ConnConfig.Tracer = w
	}
}

type acquireWatchKey struct{}

// acquireWatch implements pgxpool.AcquireTracer (and pgx.QueryTracer, which pgx
// requires of the tracer, without tracing any query).
type acquireWatch struct {
	t testing.TB
	// mu guards done: a request that outlives the test must not touch t.
	mu   sync.Mutex
	done bool
}

func (w *acquireWatch) finish() {
	w.mu.Lock()
	w.done = true
	w.mu.Unlock()
}

type acquireWait struct{ timer *time.Timer }

func (w *acquireWatch) TraceAcquireStart(ctx context.Context, pool *pgxpool.Pool, _ pgxpool.TraceAcquireStartData) context.Context {
	// Only the program counters are taken, and the frames are scanned for the
	// transaction's closure: the stack is formatted only when it is flagged, or
	// when the backstop timer fires. The normal path pays for the scan alone.
	pcs := make([]uintptr, 256)
	pcs = pcs[:runtime.Callers(2, pcs)]
	nested := false
	frames := runtime.CallersFrames(pcs)
	for {
		f, more := frames.Next()
		if strings.Contains(f.Function, inTxClosure) {
			nested = true
			break
		}
		if !more {
			break
		}
	}
	if nested {
		w.mu.Lock()
		if !w.done {
			w.t.Errorf("nested pool acquisition: a query went through the pool inside a db.InTx closure, "+
				"which takes a second connection while the transaction holds the first and deadlocks under load "+
				"(inside a transaction use only queries.WithTx(tx), or a function that takes the pgx.Tx). The stack:\n%s", formatStack(pcs))
		}
		w.mu.Unlock()
		// Fail the Acquire at once: nothing waits for a connection the
		// transaction would never give back.
		ctx, cancel := context.WithCancelCause(ctx)
		cancel(errNestedAcquire)
		return ctx
	}
	wait := &acquireWait{}
	wait.timer = time.AfterFunc(nestedAcquireTimeout, func() {
		panic(fmt.Sprintf("dbtest: no pool connection for %v (pool of %d): a read through the pool "+
			"inside a db.InTx closure, probably. The goroutine that waits:\n%s",
			nestedAcquireTimeout, pool.Config().MaxConns, formatStack(pcs)))
	})
	return context.WithValue(ctx, acquireWatchKey{}, wait)
}

// formatStack prints the frames of a captured stack, one function and one
// file:line each.
func formatStack(pcs []uintptr) string {
	var b strings.Builder
	frames := runtime.CallersFrames(pcs)
	for {
		f, more := frames.Next()
		fmt.Fprintf(&b, "\t%s\n\t\t%s:%d\n", f.Function, f.File, f.Line)
		if !more {
			return b.String()
		}
	}
}

func (*acquireWatch) TraceAcquireEnd(ctx context.Context, _ *pgxpool.Pool, _ pgxpool.TraceAcquireEndData) {
	if w, ok := ctx.Value(acquireWatchKey{}).(*acquireWait); ok {
		w.timer.Stop()
	}
}

func (*acquireWatch) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	return ctx
}
func (*acquireWatch) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// SideConnection returns a pool of its own on the database of pool, for a test
// that must hold a transaction open while the service under test works: the
// service's pool has one connection (see guardPool), so a transaction begun on
// it would leave the service with none. Begin on the side pool (side.Begin):
// do not wrap service calls in db.InTx(ctx, side, ...), the guard would flag
// them as nested. The side pool is closed when the test
// ends and is not guarded: it is the test's, not the service's.
func SideConnection(t testing.TB, pool *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	side, err := pgxpool.New(t.Context(), pool.Config().ConnString())
	if err != nil {
		t.Fatalf("open a side connection: %v", err)
	}
	t.Cleanup(side.Close)
	return side
}

// take returns a free database with this prefix, if there is one.
func take(prefix string) (string, bool) {
	freeMu.Lock()
	defer freeMu.Unlock()
	names := free[prefix]
	if len(names) == 0 {
		return "", false
	}
	name := names[len(names)-1]
	free[prefix] = names[:len(names)-1]
	return name, true
}

// emptyAll deletes every row of every table, children before parents.
func (tpl templateDB) emptyAll() string {
	var b strings.Builder
	for i := len(tpl.tables) - 1; i >= 0; i-- {
		if tpl.tables[i] == "goose_db_version" {
			continue // the migrations it records stay
		}
		b.WriteString("DELETE FROM " + pgx.Identifier{tpl.tables[i]}.Sanitize() + ";")
	}
	return b.String()
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

	ddl, tables, err := schemaDDL(ctx, tpl)
	if err != nil {
		return templateDB{}, err
	}
	return templateDB{name: name, ddl: ddl, tables: tables}, nil
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
func schemaDDL(ctx context.Context, tpl *sql.DB) ([]string, []string, error) {
	sequences, err := names(ctx, tpl, `SELECT sequence_name FROM information_schema.sequences WHERE sequence_schema = 'public' ORDER BY 1`)
	if err != nil {
		return nil, nil, err
	}
	tables, err := names(ctx, tpl, `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE' ORDER BY 1`)
	if err != nil {
		return nil, nil, err
	}

	var ddl, order []string
	for _, s := range sequences {
		stmt, err := showCreate(ctx, tpl, "SEQUENCE", s)
		if err != nil {
			return nil, nil, err
		}
		ddl = append(ddl, stmt)
	}
	create := make(map[string]string, len(tables))
	for _, table := range tables {
		if create[table], err = showCreate(ctx, tpl, "TABLE", table); err != nil {
			return nil, nil, err
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
				order = append(order, table)
			}
		}
		if !progress {
			left := slices.DeleteFunc(slices.Clone(tables), func(t string) bool { return done[t] })
			return nil, nil, fmt.Errorf("foreign keys in a cycle among %s", strings.Join(left, ", "))
		}
	}
	return ddl, order, nil
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
