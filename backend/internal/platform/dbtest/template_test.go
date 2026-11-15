package dbtest

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"sync"
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
	if again.backup != first.backup || !slices.Equal(again.tables, first.tables) || !slices.Equal(again.reference, first.reference) {
		t.Error("the second build read another template")
	}
	if !slices.Contains(again.tables, "goose_db_version") || !slices.Contains(again.reference, "session_event_kinds") || slices.Contains(again.reference, "goose_db_version") {
		t.Errorf("tables %v, reference %v", again.tables, again.reference)
	}
	if again.postgres != (again.backup == "") {
		t.Errorf("backup = %q on a template with postgres = %v: only a CockroachDB template is backed up", again.backup, again.postgres)
	}
}

// TestGooseIDsOfACopiedDatabase checks that goose_db_version of a test database uses the identity sequence the migrations made, whichever
// the server: a copy of the template keeps what the template has.
func TestGooseIDsOfACopiedDatabase(t *testing.T) {
	t.Parallel()
	pool := NewPool(t, "meurpg_dbtest_goose")
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	var used string
	if err := pool.QueryRow(ctx, `SELECT pg_get_serial_sequence('goose_db_version', 'id')`).Scan(&used); err != nil {
		t.Fatal(err)
	}
	if used != "public.goose_db_version_id_seq" {
		t.Errorf("goose_db_version uses the sequence %q, want public.goose_db_version_id_seq", used)
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
	query := map[string]string{
		"ROUTINES": `SELECT create_statement FROM [SHOW CREATE ALL ROUTINES]`,
		"TRIGGERS": `SELECT create_statement FROM [SHOW CREATE ALL TRIGGERS]`,
		"TABLES":   `SELECT create_statement FROM [SHOW CREATE ALL TABLES]`,
	}[what]
	if query == "" {
		t.Fatalf("showAll does not read %q", what)
	}
	rows, err := conn.QueryContext(t.Context(), query)
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

// TestARestoredTemplateEqualsTheTemplate compares a database made by RESTORE of the template's backup with the template itself: the
// tables with their indexes, constraints and sequences, the functions, the triggers, the reference rows and goose's table.
func TestARestoredTemplateEqualsTheTemplate(t *testing.T) {
	rawURL := testenv.DatabaseURL(t)
	templateOnce.Do(func() { template, errTemplate = buildTemplate(rawURL) })
	if errTemplate != nil {
		t.Fatalf("build the template: %v", errTemplate)
	}
	if template.postgres {
		t.Skip("PostgreSQL copies the template natively")
	}
	restored := open(t, mustDSN(t, rawURL, NewPool(t, "meurpg_dbtest_restored").Config().ConnConfig.Database))
	tpl := open(t, mustDSN(t, rawURL, template.name))

	for _, what := range []string{"TABLES", "ROUTINES", "TRIGGERS"} {
		got, want := showAll(t, restored, what), showAll(t, tpl, what)
		if len(want) == 0 && what != "TRIGGERS" {
			t.Fatalf("the template has no %s: the comparison proves nothing", what)
		}
		if !slices.Equal(got, want) {
			t.Errorf("SHOW CREATE ALL %s differs between the restored database and the template:\nonly restored: %q\nonly template: %q", what, only(got, want), only(want, got))
		}
	}
	for _, table := range append(slices.Clone(template.reference), "goose_db_version") {
		if got, want := rowsDigest(t, restored, table), rowsDigest(t, tpl, table); got != want {
			t.Errorf("the rows of %s differ: restored %s, template %s", table, got, want)
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

// TestConcurrentTemplateBuildsTakeOneBackup starts several builds of one new template at once, as the packages of a shard do on a
// server that has none: the process that migrates the template takes its one backup, and the others return once it is finished, so
// the collection holds one backup and a copy of the template restores from it.
func TestConcurrentTemplateBuildsTakeOneBackup(t *testing.T) {
	rawURL := testenv.DatabaseURL(t)
	admin := open(t, rawURL)
	postgres, err := migrations.IsPostgres(t.Context(), admin)
	if err != nil {
		t.Fatal(err)
	}
	if postgres {
		t.Skip("PostgreSQL copies the template natively, with no backup")
	}
	name := fmt.Sprintf("meurpg_tpl_concurrent_%d", time.Now().UnixNano())
	t.Cleanup(func() { mustExec(t, admin, migrations.DropDatabaseSQL(false, name)) })

	const builders = 4
	built := make([]templateDB, builders)
	errs := make([]error, builders)
	var wg sync.WaitGroup
	for i := range builders {
		wg.Go(func() { built[i], errs[i] = buildTemplateNamed(rawURL, name) })
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("builder %d: %v", i, err)
		}
	}

	var backups int
	if err := admin.QueryRowContext(t.Context(), fmt.Sprintf("SELECT count(*) FROM [SHOW BACKUPS IN '%s']", backupURI(name))).Scan(&backups); err != nil {
		t.Fatalf("list the backups: %v", err)
	}
	if backups != 1 {
		t.Errorf("the collection holds %d backups, want 1", backups)
	}
	for i, tpl := range built {
		copyName := fmt.Sprintf("%s_copy%d", name, i)
		t.Cleanup(func() { mustExec(t, admin, migrations.DropDatabaseSQL(false, copyName)) })
		if _, err := admin.ExecContext(t.Context(), restoreSQL(tpl, copyName)); err != nil {
			t.Errorf("restore for builder %d: %v", i, err)
		}
	}
}
