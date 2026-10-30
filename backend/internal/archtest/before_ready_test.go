package archtest

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// beforeReadyFile is the list `make before-ready` runs (docs/testing.md#before-ready).
const beforeReadyFile = "before_ready.txt"

// beforeReadyLine is one line of the list: `run` (tests to run), `skip` (tests that pin a file
// but need no run, with the reason) or `helper` (a function whose callers pin a file).
type beforeReadyLine struct {
	kind  string // "run", "skip" or "helper"
	pkg   string // directory under backend/
	re    *regexp.Regexp
	regex string
	last  string // the regenerate command (run), or the reason (skip, helper)
	n     int    // line number, for messages
}

// readBeforeReady parses the list: tab-separated `kind pkg regex last` lines, `#` comments and blank lines.
func readBeforeReady(t *testing.T) []beforeReadyLine {
	t.Helper()
	f, err := os.Open(beforeReadyFile)
	if err != nil {
		t.Fatalf("open the before-ready list: %v", err)
	}
	defer f.Close()
	var lines []beforeReadyLine
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		text := sc.Text()
		if strings.TrimSpace(text) == "" || strings.HasPrefix(text, "#") {
			continue
		}
		cols := strings.Split(text, "\t")
		const wantCols = 4
		if len(cols) != wantCols || (cols[0] != "run" && cols[0] != "skip" && cols[0] != "helper") || strings.TrimSpace(cols[3]) == "" {
			t.Fatalf("%s:%d: want `run|skip<TAB>package<TAB>regex<TAB>regenerate command (run) or reason (skip)`, got %q", beforeReadyFile, n, text)
		}
		re, err := regexp.Compile(cols[2])
		if err != nil {
			t.Fatalf("%s:%d: regex %q: %v", beforeReadyFile, n, cols[2], err)
		}
		lines = append(lines, beforeReadyLine{kind: cols[0], pkg: cols[1], re: re, regex: cols[2], last: cols[3], n: n})
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read the before-ready list: %v", err)
	}
	return lines
}

// pinSignal says whether a node on its own shows that a function compares its
// result with a stored file: a literal naming a golden, a hash file or the
// fixtures, the shared golden package, or an `-update` flag read.
func pinSignal(n ast.Node) bool {
	switch v := n.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return false
		}
		for _, mark := range []string{".golden", ".sha256", "testdata", "UPDATE_GOLDEN"} {
			if strings.Contains(v.Value, mark) {
				return true
			}
		}
	case *ast.SelectorExpr:
		id, ok := v.X.(*ast.Ident)
		return ok && id.Name == "goldentest" && (v.Sel.Name == "Compare" || v.Sel.Name == "JSON")
	case *ast.StarExpr:
		id, ok := v.X.(*ast.Ident)
		return ok && strings.HasPrefix(strings.ToLower(id.Name), "update")
	}
	return false
}

// pinningTests returns the Test functions of one package directory that
// compare with a stored file, directly or through plain function calls of the
// same package (a method call is not followed: names collide between types,
// so those tests rely on the naming convention of the list), and every Test.
func pinningTests(t *testing.T, dir string, helpers []beforeReadyLine) (pins, all []string) {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var files []*ast.File
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		files = append(files, file)
	}
	direct := map[string]bool{}
	calls := map[string][]string{}
	var tests []string
	{
		for _, file := range files {
			for _, decl := range file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil || fd.Recv != nil {
					continue
				}
				name := fd.Name.Name
				if strings.HasPrefix(name, "Test") && strings.HasSuffix(fset.Position(fd.Pos()).Filename, "_test.go") {
					tests = append(tests, name)
				}
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					if pinSignal(n) {
						direct[name] = true
					}
					if call, ok := n.(*ast.CallExpr); ok {
						if id, ok := call.Fun.(*ast.Ident); ok {
							calls[name] = append(calls[name], id.Name)
						}
					}
					return true
				})
			}
		}
	}
	for _, h := range helpers {
		direct[h.regex] = true
	}
	// A function pins when it has the signal or calls one that does.
	for changed := true; changed; {
		changed = false
		for fn, callees := range calls {
			if direct[fn] {
				continue
			}
			for _, c := range callees {
				if direct[c] {
					direct[fn], changed = true, true
					break
				}
			}
		}
	}
	for _, name := range tests {
		if direct[name] {
			pins = append(pins, name)
		}
	}
	sort.Strings(tests)
	sort.Strings(pins)
	return pins, tests
}

// TestBeforeReadyListIsWhole keeps the list `make before-ready` runs honest:
// every line names a package and tests that exist, and a test that compares
// with a golden, a hash or a fixture file is on the list (or skipped with the
// reason), so a new one cannot be forgotten.
func TestBeforeReadyListIsWhole(t *testing.T) {
	t.Parallel()
	lines := readBeforeReady(t)
	byPkg := map[string][]beforeReadyLine{}
	for _, l := range lines {
		byPkg[l.pkg] = append(byPkg[l.pkg], l)
	}

	for pkg, ls := range byPkg {
		dir := filepath.Join(backendRoot, pkg)
		_, all := pinningTests(t, dir, nil)
		for _, l := range ls {
			if l.kind == "helper" {
				continue
			}
			for _, part := range alternatives(l.regex) {
				if !anyMatch(regexp.MustCompile(part), all) {
					t.Errorf("%s:%d: %s matches no test of %s: the test was renamed or removed, fix the line", beforeReadyFile, l.n, part, pkg)
				}
			}
		}
	}

	var dirs []string
	err := filepath.WalkDir(backendRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != backendRoot && (d.Name() == "testdata" || d.Name() == "gen" || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(path, "_test.go") {
			dirs = append(dirs, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", backendRoot, err)
	}
	seen := map[string]bool{}
	for _, dir := range dirs {
		if seen[dir] {
			continue
		}
		seen[dir] = true
		pkg, err := filepath.Rel(backendRoot, dir)
		if err != nil {
			t.Fatal(err)
		}
		pkg = filepath.ToSlash(pkg)
		pins, _ := pinningTests(t, dir, helpersOf(byPkg[pkg]))
		for _, name := range pins {
			if !listed(byPkg[pkg], name) {
				t.Errorf("%s compares with a golden, hash or fixture file but is not in %s: add a `run` line for %s (with the command that rewrites its file) or a `skip` line with the reason",
					fmt.Sprintf("%s.%s", pkg, name), beforeReadyFile, pkg)
			}
		}
	}
}

// namesRegex is a regex that only lists names: ^(TestA|TestB)$.
var namesRegex = regexp.MustCompile(`^\^\(([A-Za-z0-9_|]+)\)\$$`)

// alternatives splits a regex that lists names into each name, so a renamed test is
// found even when the others of its line still match; any other regex is one part.
func alternatives(regex string) []string {
	if m := namesRegex.FindStringSubmatch(regex); m != nil {
		var out []string
		for _, name := range strings.Split(m[1], "|") {
			out = append(out, "^"+name+"$")
		}
		return out
	}
	return []string{regex}
}

func anyMatch(re *regexp.Regexp, names []string) bool {
	for _, n := range names {
		if re.MatchString(n) {
			return true
		}
	}
	return false
}

func helpersOf(ls []beforeReadyLine) []beforeReadyLine {
	var out []beforeReadyLine
	for _, l := range ls {
		if l.kind == "helper" {
			out = append(out, l)
		}
	}
	return out
}

func listed(ls []beforeReadyLine, name string) bool {
	for _, l := range ls {
		if l.kind != "helper" && l.re.MatchString(name) {
			return true
		}
	}
	return false
}
