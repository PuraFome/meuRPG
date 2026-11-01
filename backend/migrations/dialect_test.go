package migrations

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// lastCockroachOnlyMigration is the newest migration written before the tests
// ran on PostgreSQL. The ones up to it may use what only CockroachDB accepts
// (postgres.go adapts them); a newer one, and every query the application
// sends, must run on both.
const lastCockroachOnlyMigration = 288

// cockroachOnly are the spellings that only CockroachDB accepts.
var cockroachOnly = []struct {
	re   *regexp.Regexp
	what string
}{
	{regexp.MustCompile(`(?i)\bSTRING\b`), "the STRING type (write TEXT)"},
	{regexp.MustCompile(`(?i)\bunique_rowid\s*\(`), "unique_rowid() (write a sequence or an identity column)"},
	{regexp.MustCompile(`(?i)\bttl_[a-z_]+`), "row-level TTL (sweep the rows in the application)"},
	{regexp.MustCompile(`(?i)\bcrdb_internal\b`), "crdb_internal"},
	{regexp.MustCompile(`(?i)\bSHOW\s+CREATE\b`), "SHOW CREATE"},
	{regexp.MustCompile(`(?i)\bAS\s+OF\s+SYSTEM\s+TIME\b`), "AS OF SYSTEM TIME"},
	{regexp.MustCompile(`(?i)\bUSING\s+HASH\b`), "USING HASH"},
	{regexp.MustCompile(`(?i)\bSTORING\b`), "STORING (write INCLUDE)"},
	{regexp.MustCompile(`(?i)\bSET\s+TRANSACTION\s+PRIORITY\b`), "SET TRANSACTION PRIORITY"},
}

var sqlComment = regexp.MustCompile(`(?m)--.*$`)

// dialectProblems names what a SQL text uses that only CockroachDB accepts.
func dialectProblems(sql string) []string {
	sql = sqlComment.ReplaceAllString(sql, "")
	var out []string
	for _, c := range cockroachOnly {
		if c.re.MatchString(sql) {
			out = append(out, c.what)
		}
	}
	return out
}

func TestDialectProblemsFindsWhatOnlyCockroachDBAccepts(t *testing.T) {
	t.Parallel()
	for sql, want := range map[string]int{
		"CREATE TABLE t (a STRING)":                              1,
		"CREATE TABLE t (a TEXT) WITH (ttl_expire_after = '1h')": 1,
		"SELECT unique_rowid()":                                  1,
		"SELECT 1 -- STRING is cut in a comment":                 0,
		"CREATE TABLE t (a TEXT, b BIGINT)":                      0,
		"SELECT x FROM t AS OF SYSTEM TIME '-1s'":                1,
	} {
		if got := dialectProblems(sql); len(got) != want {
			t.Errorf("dialectProblems(%q) = %v, want %d", sql, got, want)
		}
	}
}

func TestNewMigrationsAndQueriesRunOnBothDatabases(t *testing.T) {
	t.Parallel()
	check := func(path string, body []byte) {
		for _, p := range dialectProblems(string(body)) {
			t.Errorf("%s uses %s, which only CockroachDB accepts; write SQL both databases run (docs/data/README.md)", path, p)
		}
	}
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, e := range entries {
		n, err := strconv.Atoi(strings.SplitN(e.Name(), "_", 2)[0])
		if err != nil || n <= lastCockroachOnlyMigration {
			continue
		}
		body, err := fs.ReadFile(files, e.Name())
		if err != nil {
			t.Fatal(err)
		}
		check(e.Name(), body)
		checked++
	}
	queries, err := filepath.Glob(filepath.Join("..", "internal", "*", "queries.sql"))
	if err != nil || len(queries) == 0 {
		t.Fatalf("find the queries.sql files: %v (found %d)", err, len(queries))
	}
	for _, q := range queries {
		body, err := fs.ReadFile(os.DirFS(".."), filepath.ToSlash(strings.TrimPrefix(q, ".."+string(filepath.Separator))))
		if err != nil {
			t.Fatal(err)
		}
		check(q, body)
	}
	t.Logf("checked %d new migrations and %d query files", checked, len(queries))
}
