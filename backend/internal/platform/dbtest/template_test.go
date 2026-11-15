package dbtest

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/PuraFome/meuRPG/backend/internal/platform/testenv"
	"github.com/PuraFome/meuRPG/backend/migrations"
)

// TestMain drops the databases this package's own tests take from NewPool.
func TestMain(m *testing.M) { Main(m) }

// scratchDatabase creates an empty database with these statements run in it,
// and returns a connection to it. It is dropped when the test ends.
func scratchDatabase(t *testing.T, label string, stmts ...string) *sql.DB {
	t.Helper()
	rawURL := testenv.DatabaseURL(t)
	admin := open(t, rawURL)
	name := fmt.Sprintf("meurpg_dbtest_%s_%d", label, time.Now().UnixNano())
	mustExec(t, admin, "CREATE DATABASE "+name)
	t.Cleanup(func() { mustExec(t, admin, migrations.DropDatabaseSQL(template.postgres, name)) })
	dsn, err := withDatabase(rawURL, name)
	if err != nil {
		t.Fatal(err)
	}
	conn := open(t, dsn)
	for _, stmt := range stmts {
		mustExec(t, conn, stmt)
	}
	return conn
}

// TestSchemaDDLOrdersTablesAfterTheirReferences checks schemaDDL: the
// sequences first, then the tables in rounds, each round taking (in name
// order) the tables whose references are all created; a table that points
// at itself is ready at once.
func TestSchemaDDLOrdersTablesAfterTheirReferences(t *testing.T) {
	t.Parallel()
	conn := scratchDatabase(t, "ddl",
		`CREATE SEQUENCE s_b`,
		`CREATE SEQUENCE s_a`,
		`CREATE TABLE z_root (id INT PRIMARY KEY)`,
		`CREATE TABLE b_parent (id INT PRIMARY KEY, root INT REFERENCES z_root (id))`,
		`CREATE TABLE c_child (id INT PRIMARY KEY, parent INT REFERENCES b_parent (id), root INT REFERENCES z_root (id))`,
		`CREATE TABLE a_self (id INT PRIMARY KEY, up INT REFERENCES a_self (id))`,
		`CREATE VIEW v_roots AS SELECT id FROM z_root`,
	)
	ddl, order, err := schemaDDL(t.Context(), conn)
	if err != nil {
		t.Fatalf("schemaDDL() error = %v", err)
	}
	wantOrder := []string{"a_self", "z_root", "b_parent", "c_child"}
	if !slices.Equal(order, wantOrder) {
		t.Errorf("order = %v, want %v", order, wantOrder)
	}
	wantPrefixes := []string{
		"CREATE SEQUENCE public.s_a ",
		"CREATE SEQUENCE public.s_b ",
		"CREATE TABLE public.a_self (",
		"CREATE TABLE public.z_root (",
		"CREATE TABLE public.b_parent (",
		"CREATE TABLE public.c_child (",
	}
	if len(ddl) != len(wantPrefixes) {
		t.Fatalf("ddl has %d statements, want %d:\n%s", len(ddl), len(wantPrefixes), strings.Join(ddl, "\n"))
	}
	for i, prefix := range wantPrefixes {
		if !strings.HasPrefix(ddl[i], prefix) {
			t.Errorf("ddl[%d] = %q, want it to start with %q", i, ddl[i], prefix)
		}
	}
	if !strings.Contains(ddl[5], "REFERENCES public.b_parent(id)") || !strings.Contains(ddl[5], "REFERENCES public.z_root(id)") {
		t.Errorf("the foreign keys are not inline: %s", ddl[5])
	}
}

// TestSchemaDDLRefusesACycle checks the error for foreign keys in a cycle:
// it names, in name order, every table left once no round makes progress
// (the tables that only depend on the cycle, too).
func TestSchemaDDLRefusesACycle(t *testing.T) {
	t.Parallel()
	conn := scratchDatabase(t, "cycle",
		`CREATE TABLE free (id INT PRIMARY KEY)`,
		`CREATE TABLE x_cyc (id INT PRIMARY KEY, y INT)`,
		`CREATE TABLE y_cyc (id INT PRIMARY KEY, x INT REFERENCES x_cyc (id))`,
		`ALTER TABLE x_cyc ADD CONSTRAINT x_y FOREIGN KEY (y) REFERENCES y_cyc (id)`,
		`CREATE TABLE after_cyc (id INT PRIMARY KEY, x INT REFERENCES x_cyc (id))`,
	)
	ddl, order, err := schemaDDL(t.Context(), conn)
	if want := "foreign keys in a cycle among after_cyc, x_cyc, y_cyc"; err == nil || err.Error() != want {
		t.Errorf("schemaDDL() error = %v, want %q", err, want)
	}
	if ddl != nil || order != nil {
		t.Errorf("schemaDDL() = %v, %v with its error", ddl, order)
	}
}

// TestSchemaDDLErrors checks the error of a catalog that cannot be read.
func TestSchemaDDLErrors(t *testing.T) {
	t.Parallel()
	conn := scratchDatabase(t, "closed")
	_ = conn.Close()
	_, _, err := schemaDDL(t.Context(), conn)
	if want := "list the template's objects: sql: database is closed"; err == nil || err.Error() != want {
		t.Errorf("schemaDDL() error = %v, want %q", err, want)
	}
}

// TestBuildTemplateErrors checks buildTemplate's refusals: a URL that does
// not parse, and a server that does not answer.
func TestBuildTemplateErrors(t *testing.T) {
	t.Parallel()
	if _, err := buildTemplate("postgresql://[::1"); err == nil || err.Error() != `parse "postgresql://[::1": missing ']' in host` {
		t.Errorf("buildTemplate(bad URL) error = %v", err)
	}
	_, err := buildTemplate("postgresql://root@127.0.0.1:1/none?sslmode=disable&connect_timeout=5")
	want := "create the template: failed to connect to `user=root database=none`: 127.0.0.1:1 (127.0.0.1): dial error: dial tcp 127.0.0.1:1: connect: connection refused"
	if err == nil || err.Error() != want {
		t.Errorf("buildTemplate(unreachable) error = %v, want %q", err, want)
	}
}

// TestBuildTemplateFindsTheMigratedTemplate checks the path of every process
// but the first: the template exists (CREATE DATABASE says 42P04) and is
// complete, so it is read, not migrated again, and gives what the first
// process got.
func TestBuildTemplateFindsTheMigratedTemplate(t *testing.T) {
	t.Parallel()
	rawURL := testenv.DatabaseURL(t)
	first, err := buildTemplate(rawURL)
	if err != nil {
		t.Fatalf("buildTemplate() error = %v", err)
	}
	again, err := buildTemplate(rawURL)
	if err != nil {
		t.Fatalf("second buildTemplate() error = %v", err)
	}
	fingerprint, err := migrations.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if again.name != "meurpg_tpl_"+fingerprint || again.name != first.name {
		t.Errorf("template name = %q, want meurpg_tpl_%s", again.name, fingerprint)
	}
	if !slices.Equal(again.ddl, first.ddl) || !slices.Equal(again.tables, first.tables) || !slices.Equal(again.reference, first.reference) {
		t.Error("the second build read another template")
	}
	if !slices.Contains(again.tables, "goose_db_version") || !slices.Contains(again.reference, "session_event_kinds") || slices.Contains(again.reference, "goose_db_version") {
		t.Errorf("tables %v, reference %v", again.tables, again.reference)
	}
	if again.postgres {
		if len(again.ddl) != 0 {
			t.Errorf("a PostgreSQL template keeps no DDL (it is copied natively), got %d statements", len(again.ddl))
		}
	} else if len(again.ddl) == 0 || !strings.HasPrefix(again.ddl[0], "CREATE SEQUENCE public.goose_db_version_id_seq ") {
		t.Errorf("ddl = %q, want goose's sequence first", again.ddl)
	}
}

// TestGooseIDsOfACopiedDatabase documents a known defect, not fixed: the template's
// DDL creates goose's identity sequence (goose_db_version_id_seq) as a
// sequence of its own, before goose_db_version, whose identity column then
// makes another one. NewPool's setval moves the copy, not the sequence the
// table uses, so the next row goose inserts in a test database (after a
// down, say) collides with a copied id. No test runs goose on a test
// database today.
func TestGooseIDsOfACopiedDatabase(t *testing.T) {
	t.Parallel()
	pool := NewPool(t, "meurpg_dbtest_goose")
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	var used string
	if err := pool.QueryRow(ctx, `SELECT pg_get_serial_sequence('goose_db_version', 'id')`).Scan(&used); err != nil {
		t.Fatal(err)
	}
	var version string
	if err := pool.QueryRow(ctx, `SELECT version()`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	// A PostgreSQL copy is native, so the table keeps the sequence it was made with; the DDL of
	// CockroachDB's copy is where the defect is.
	if postgres := !strings.Contains(version, "CockroachDB"); postgres != (used == "public.goose_db_version_id_seq") {
		t.Errorf("goose_db_version uses %q on %q: the finding changed, update this test", used, version)
	}
}

// showAll reads what SHOW CREATE ALL <what> says of the database the connection is on, sorted, with the database's own name left out
// (a trigger's table is written with it).
func showAll(t *testing.T, conn *sql.DB, what string) []string {
	t.Helper()
	var database string
	if err := conn.QueryRowContext(t.Context(), `SELECT current_database()`).Scan(&database); err != nil {
		t.Fatalf("name the database: %v", err)
	}
	rows, err := conn.QueryContext(t.Context(), `SELECT create_statement FROM [SHOW CREATE ALL `+what+`]`)
	if err != nil {
		t.Fatalf("SHOW CREATE ALL %s: %v", what, err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var stmt string
		if err := rows.Scan(&stmt); err != nil {
			t.Fatalf("read a statement: %v", err)
		}
		out = append(out, strings.ReplaceAll(stmt, database, "<database>"))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read the statements: %v", err)
	}
	slices.Sort(out)
	return out
}

// rowsDigest is a digest of the rows of a table, in a fixed order.
func rowsDigest(t *testing.T, conn *sql.DB, table string) string {
	t.Helper()
	var digest sql.NullString
	if err := conn.QueryRowContext(t.Context(), `SELECT md5(string_agg(x::text, ',' ORDER BY x::text)) FROM `+table+` x`).Scan(&digest); err != nil {
		t.Fatalf("digest of %s: %v", table, err)
	}
	return digest.String
}

// TestARestoredTemplateEqualsTheReplayedOne compares a database made by RESTORE of the template's backup with the template itself
// (the tables with their indexes, constraints and sequences, the functions, the triggers, the reference rows and goose's table), and
// with one made by replaying the template's DDL and copying its reference rows: the rows must be the same in the three, and the
// schema of the replayed one is logged where it differs.
func TestARestoredTemplateEqualsTheReplayedOne(t *testing.T) {
	rawURL := testenv.DatabaseURL(t)
	templateOnce.Do(func() { template, errTemplate = buildTemplate(rawURL) })
	if errTemplate != nil {
		t.Fatalf("build the template: %v", errTemplate)
	}
	if template.postgres {
		t.Skip("PostgreSQL copies the template natively")
	}
	restored := open(t, mustDSN(t, rawURL, NewPool(t, "meurpg_dbtest_restored").Config().ConnConfig.Database))
	replayed := scratchDatabase(t, "replayed")
	copyCockroachTemplate(t, replayed)
	tpl := open(t, mustDSN(t, rawURL, template.name))

	for _, what := range []string{"TABLES", "ROUTINES", "TRIGGERS"} {
		got, want, replay := showAll(t, restored, what), showAll(t, tpl, what), showAll(t, replayed, what)
		if len(want) == 0 && what != "TRIGGERS" {
			t.Fatalf("the template has no %s: the comparison proves nothing", what)
		}
		if !slices.Equal(got, want) {
			t.Errorf("SHOW CREATE ALL %s differs between the restored database and the template:\nonly restored: %q\nonly template: %q", what, only(got, want), only(want, got))
		}
		// What the replay makes that the template does not have, or lacks.
		if extra, lacks := only(replay, want), only(want, replay); len(extra)+len(lacks) > 0 {
			t.Logf("SHOW CREATE ALL %s: the replayed database has %q and lacks %q", what, extra, lacks)
		}
	}
	for _, table := range append(slices.Clone(template.reference), "goose_db_version") {
		if got, want, replay := rowsDigest(t, restored, table), rowsDigest(t, tpl, table), rowsDigest(t, replayed, table); got != want || replay != want {
			t.Errorf("the rows of %s differ: restored %s, template %s, replayed %s", table, got, want, replay)
		}
	}
	// Nothing but the reference rows: every other table is empty.
	for _, table := range template.tables {
		if slices.Contains(template.reference, table) || table == "goose_db_version" {
			continue
		}
		var has bool
		if err := restored.QueryRowContext(t.Context(), `SELECT EXISTS (SELECT 1 FROM `+table+`)`).Scan(&has); err != nil || has {
			t.Errorf("the restored table %s has rows (%v, error %v)", table, has, err)
		}
	}
}

func mustDSN(t *testing.T, rawURL, database string) string {
	t.Helper()
	dsn, err := withDatabase(rawURL, database)
	if err != nil {
		t.Fatalf("parse the URL: %v", err)
	}
	return dsn
}

// only returns the elements of a that b lacks.
func only(a, b []string) []string {
	var out []string
	for _, x := range a {
		if !slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}
