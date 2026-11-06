package archtest

import (
	_ "embed"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// pgErrorsExceptions is the list of files in identity and authz that still
// read a database error's code themselves; it can only shrink.
//
//go:embed pg_errors_exceptions.txt
var pgErrorsExceptions string

// pgErrorModules are the modules that translate database errors only through
// the common helper; their sqlc packages are generated and skipped.
var (
	pgErrorModules = []string{"internal/identity", "internal/authz"}
	pgErrorSkip    = []string{"internal/identity/identitydb", "internal/authz/authzdb"}
)

var sqlStateLiteral = regexp.MustCompile(`^"\d{5}"$`)

// TestIdentityAuthzTranslateDBErrorsViaHelper fails for a non-test file of
// internal/identity or internal/authz that imports pgconn or uses PgError,
// SQLState or a SQLSTATE literal, and that pg_errors_exceptions.txt does not
// list; and for a listed file that no longer does.
func TestIdentityAuthzTranslateDBErrorsViaHelper(t *testing.T) {
	root := os.DirFS(backendRoot)
	found := map[string]string{}
	files := 0
	for _, dir := range pgErrorModules {
		err := walkGoFiles(root, dir, func(path string) error {
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			for _, skip := range pgErrorSkip {
				if strings.HasPrefix(path, skip+"/") {
					return nil
				}
			}
			files++
			src, err := fs.ReadFile(root, path)
			if err != nil {
				return err
			}
			what, err := pgErrorUses(path, src)
			if err != nil {
				return err
			}
			if len(what) > 0 {
				found[path] = fmt.Sprintf("%s reads a database error's code itself (%s): use db.IsUniqueViolation "+
					"(internal/platform/db) for a unique violation and rpcerr (internal/platform/rpcerr) for the RPC error; "+
					"errors.Is(err, pgx.ErrNoRows) is fine", path, strings.Join(what, ", "))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("reading %s: %v", dir, err)
		}
	}
	if files == 0 {
		t.Fatal("found no Go file: is backendRoot right?")
	}
	failProblems(t, compareExceptions("pg_errors_exceptions.txt", parseExceptions(pgErrorsExceptions), found))
	t.Logf("%d non-test Go files of identity and authz checked for pgconn, PgError, SQLState and SQLSTATE literals; %d known exceptions", files, len(found))
}

// pgErrorUses returns what in one file reads a database error's code: the
// pgconn import, the names PgError and SQLState, a 5-digit string literal.
func pgErrorUses(path string, src []byte) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var what []string
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			what = append(what, s)
		}
	}
	for _, imp := range f.Imports {
		if p, err := strconv.Unquote(imp.Path.Value); err == nil && p == "github.com/jackc/pgx/v5/pgconn" {
			add("imports pgconn")
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			if n.Name == "PgError" || n.Name == "SQLState" {
				add("uses " + n.Name)
			}
		case *ast.BasicLit:
			if n.Kind == token.STRING && sqlStateLiteral.MatchString(n.Value) {
				add("literal " + n.Value)
			}
		}
		return true
	})
	return what, nil
}

func TestPgErrorUses(t *testing.T) {
	cases := []struct {
		name, src string
		want      string
	}{
		{"pgconn import", "package x\nimport \"github.com/jackc/pgx/v5/pgconn\"\nvar _ = pgconn.Timeout", "[imports pgconn]"},
		{"PgError", "package x\nvar e *struct{ PgError int }", "[uses PgError]"},
		{"SQLState compare", "package x\nfunc f(e interface{ SQLState() string }) bool { return e.SQLState() == \"23505\" }", "[uses SQLState literal \"23505\"]"},
		{"ErrNoRows is fine", "package x\nimport (\"errors\"; \"github.com/jackc/pgx/v5\")\nvar _ = errors.Is(nil, pgx.ErrNoRows)", "[]"},
		{"other number string", "package x\nvar s = \"123456\"; var t = \"12\"", "[]"},
	}
	for _, c := range cases {
		got, err := pgErrorUses("internal/identity/a.go", []byte(c.src))
		if err != nil || fmt.Sprint(got) != c.want {
			t.Errorf("%s: got %v (err %v), want %s", c.name, got, err, c.want)
		}
	}
}
