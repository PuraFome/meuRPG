package archtest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/archtest/beforereadylist"
)

// beforeReadyFile is the list `make before-ready` runs (docs/testing.md#before-ready).
const beforeReadyFile = "before_ready.txt"

// beforeReadyLine is one line of the list, as the parser the tool uses reads it.
type beforeReadyLine = beforereadylist.Line

// readBeforeReady parses the list with the parser `make before-ready` uses (beforereadylist).
func readBeforeReady(t *testing.T) []beforeReadyLine {
	t.Helper()
	f, err := os.Open(beforeReadyFile)
	if err != nil {
		t.Fatalf("open the before-ready list: %v", err)
	}
	defer f.Close()
	lines, err := beforereadylist.Parse(f)
	if err != nil {
		t.Fatalf("%s: %v", beforeReadyFile, err)
	}
	return lines
}

// parsedDir is the Go files of one directory, parsed once per run: three guards read each
// package, and they run in parallel.
type parsedDir struct {
	fset  *token.FileSet
	names []string // file names, in the directory's order
	files []*ast.File
}

var (
	parseMu    sync.Mutex
	parseCache = map[string]*parsedEntry{}
)

type parsedEntry struct {
	once sync.Once
	dir  *parsedDir
	err  error
}

// parseDir parses the .go files of dir (comments skipped), or returns the cached result.
func parseDir(t *testing.T, dir string) *parsedDir {
	t.Helper()
	parseMu.Lock()
	e := parseCache[dir]
	if e == nil {
		e = &parsedEntry{}
		parseCache[dir] = e
	}
	parseMu.Unlock()
	e.once.Do(func() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			e.err = fmt.Errorf("read %s: %w", dir, err)
			return
		}
		fset := token.NewFileSet()
		pd := &parsedDir{fset: fset}
		for _, ent := range entries {
			if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".go") {
				continue
			}
			file, err := parser.ParseFile(fset, filepath.Join(dir, ent.Name()), nil, 0)
			if err != nil {
				e.err = fmt.Errorf("parse %s: %w", ent.Name(), err)
				return
			}
			pd.names = append(pd.names, ent.Name())
			pd.files = append(pd.files, file)
		}
		e.dir = pd
	})
	if e.err != nil {
		t.Fatal(e.err)
	}
	return e.dir
}

// pinSignal says whether a node on its own shows that a function compares its
// result with a stored file: a literal naming a golden, a hash file or the
// fixtures, the shared golden package, or a package-level name that holds one of them (see pinNames).
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
	}
	return false
}

// pinNames returns the package-level names that stand for a stored file: a constant or variable
// whose value names a golden, a hash file or the fixtures, one named for the goldens or the
// frozen digests (frozenDigests), and a flag.Bool that turns the rewriting on. A test that
// reads one of them compares with a stored file.
func pinNames(files []*ast.File) map[string]bool {
	names := map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || (gd.Tok != token.CONST && gd.Tok != token.VAR) {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				pins := false
				for _, v := range vs.Values {
					ast.Inspect(v, func(n ast.Node) bool {
						pins = pins || pinSignal(n) || isFlagCall(n)
						return true
					})
				}
				for _, id := range vs.Names {
					lower := strings.ToLower(id.Name)
					if pins || strings.Contains(lower, "frozen") || strings.Contains(lower, "golden") {
						names[id.Name] = true
					}
				}
			}
		}
	}
	return names
}

// isFlagCall says whether n is flag.Bool(...) or the same call through an import alias ending in flag.
func isFlagCall(n ast.Node) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && strings.HasSuffix(id.Name, "flag")
}

// pinningTests returns the Test functions of one package directory that
// compare with a stored file, directly or through plain function calls of the
// same package (a method call is not followed: names collide between types,
// so those tests rely on the naming convention of the list), and every Test.
func pinningTests(t *testing.T, dir string, helpers []beforeReadyLine) (pins, all []string) {
	t.Helper()
	files := parseDir(t, dir).files
	names := pinNames(files)
	direct := map[string]bool{}
	calls := map[string][]string{}
	var tests []string
	for _, file := range files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil || fd.Recv != nil {
				continue
			}
			name := fd.Name.Name
			if strings.HasPrefix(name, "Test") {
				tests = append(tests, name)
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && names[id.Name] || pinSignal(n) {
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
	for _, h := range helpers {
		direct[h.Regex] = true
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
// with a golden, a hash or a fixture file is on the list, so a new one cannot be forgotten.
func TestBeforeReadyListIsWhole(t *testing.T) {
	t.Parallel()
	lines := readBeforeReady(t)
	byPkg := map[string][]beforeReadyLine{}
	for _, l := range lines {
		byPkg[l.Pkg] = append(byPkg[l.Pkg], l)
	}

	var problems []string
	pkgs := make([]string, 0, len(byPkg))
	for pkg := range byPkg {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)
	for _, pkg := range pkgs {
		ls := byPkg[pkg]
		dir := filepath.Join(backendRoot, pkg)
		_, all := pinningTests(t, dir, nil)
		problems = append(problems, undeclaredHelpers(ls, declaredFuncs(parseDir(t, dir).files))...)
		for _, l := range ls {
			if l.Kind == beforereadylist.Helper {
				continue
			}
			for _, part := range alternatives(l.Regex) {
				if !anyMatch(regexp.MustCompile(part), all) {
					problems = append(problems, fmt.Sprintf("%s:%d: %s matches no test of %s: the test was renamed or removed, fix the line", beforeReadyFile, l.N, part, pkg))
				}
			}
		}
	}

	dirs, err := testDirs(backendRoot)
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
				problems = append(problems, fmt.Sprintf("%s.%s compares with a golden, hash or fixture file but is not in %s: add a `run` line for %s (with the command that rewrites its file)", pkg, name, beforeReadyFile, pkg))
			}
		}
	}
	for _, p := range problems {
		t.Error(p)
	}
}

// testDirs returns the directories under root that hold a _test.go file, with repeats. It skips
// testdata, hidden directories and the generated protobuf package (root/gen) only: a directory
// named gen elsewhere (internal/maps/images/gen) holds code and tests of its own.
func testDirs(root string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && p != root && (d.Name() == "testdata" || p == filepath.Join(root, "gen") || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, "_test.go") {
			dirs = append(dirs, filepath.Dir(p))
		}
		return nil
	})
	return dirs, err
}

// groupRegex is a part with one group of plain names: ^TestLasting(Core|RPC), ^(TestA|TestB)$.
var groupRegex = regexp.MustCompile(`^([^()]*)\(([A-Za-z0-9_|]+)\)([^()]*)$`)

// alternatives splits a regex into the alternatives to check one by one, so a renamed or
// deleted sibling is found even when the others of its line still match: the parts at its
// top-level bars, each with its one group of names expanded (^Test(A|B)$ is ^TestA$ and ^TestB$).
func alternatives(regex string) []string {
	var parts []string
	depth, start := 0, 0
	for i, r := range regex {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case '|':
			if depth == 0 {
				parts = append(parts, regex[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, regex[start:])
	var out []string
	for _, part := range parts {
		m := groupRegex.FindStringSubmatch(part)
		if m == nil {
			out = append(out, part)
			continue
		}
		for _, name := range strings.Split(m[2], "|") {
			out = append(out, m[1]+name+m[3])
		}
	}
	return out
}

func anyMatch(re *regexp.Regexp, names []string) bool {
	for _, n := range names {
		if re.MatchString(n) {
			return true
		}
	}
	return false
}

// declaredFuncs returns the names of the plain functions the files declare: pinningTests follows
// plain calls only (names of methods collide between types), so a helper line cannot name a method.
func declaredFuncs(files []*ast.File) map[string]bool {
	out := map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv == nil {
				out[fd.Name.Name] = true
			}
		}
	}
	return out
}

// undeclaredHelpers returns a message for each helper line whose plain function no file of
// its package declares: a helper that was renamed marks nothing, and the tests that call it
// would silently leave the guard.
func undeclaredHelpers(lines []beforeReadyLine, declared map[string]bool) []string {
	var out []string
	for _, l := range lines {
		if l.Kind == beforereadylist.Helper && !declared[l.Regex] {
			out = append(out, fmt.Sprintf("%s:%d: the helper %s is declared by no plain function of %s (a method cannot be a helper): it was renamed or removed, fix the line", beforeReadyFile, l.N, l.Regex, l.Pkg))
		}
	}
	return out
}

func helpersOf(ls []beforeReadyLine) []beforeReadyLine {
	var out []beforeReadyLine
	for _, l := range ls {
		if l.Kind == beforereadylist.Helper {
			out = append(out, l)
		}
	}
	return out
}

func listed(ls []beforeReadyLine, name string) bool {
	for _, l := range ls {
		if l.Kind != beforereadylist.Helper && l.Re.MatchString(name) {
			return true
		}
	}
	return false
}

// TestBeforeReadyDetector keeps the guard's eyes honest on a fixture package
// (testdata/pinfixture): what compares with a stored file is found through a
// constant, a flag, the frozen digests and a helper, and a test that merely
// dereferences a variable that is not a flag (counter) is not.
func TestBeforeReadyDetector(t *testing.T) {
	t.Parallel()
	pins, all := pinningTests(t, filepath.Join("testdata", "pinfixture"), nil)
	wantPins := []string{"TestThroughAConst", "TestThroughAFlag", "TestThroughAHelper", "TestThroughTheDigests"}
	if !slices.Equal(pins, wantPins) {
		t.Errorf("pins = %v, want %v", pins, wantPins)
	}
	if len(all) != len(wantPins)+2 {
		t.Errorf("tests = %v, want the %d pins and 2 more", all, len(wantPins))
	}
	if got := alternatives(`AuthorizationMatrix$|^TestEvery$`); !slices.Equal(got, []string{`AuthorizationMatrix$`, `^TestEvery$`}) {
		t.Errorf("alternatives = %q", got)
	}
	want := []string{`Characterization$`, `Golden$`, `^TestX$`}
	if got := alternatives(`(Characterization|Golden)$|^TestX$`); !slices.Equal(got, want) {
		t.Errorf("alternatives = %q, want %q", got, want)
	}
	want = []string{`Frozen`, `^TestLastingCore`, `^TestLastingRPC`, `^TestA$`, `^TestB$`}
	if got := alternatives(`Frozen|^TestLasting(Core|RPC)|^(TestA|TestB)$`); !slices.Equal(got, want) {
		t.Errorf("alternatives = %q, want %q", got, want)
	}
}

// TestBeforeReadyInputsAreComplete keeps the inputs column honest: the default run of
// `make before-ready` runs a line when the change touches its package, a package it imports or
// one of its inputs, so a listed test that reads a file outside those must name it. Every path
// with a ".." segment that a listed test reaches (in its body, through the functions of its
// package it calls, and through the package-level names it uses) must be inside an input of
// the line, or a path the guard cannot resolve fails it. That every input exists is checked by
// beforereadylist's tests, which read the whole repository.
func TestBeforeReadyInputsAreComplete(t *testing.T) {
	t.Parallel()
	lines := readBeforeReady(t)
	byPkg := map[string][]beforeReadyLine{}
	for _, l := range lines {
		if l.Kind == beforereadylist.Run {
			byPkg[l.Pkg] = append(byPkg[l.Pkg], l)
		}
	}
	for pkg, ls := range byPkg {
		reads := upwardReads(t, filepath.Join(backendRoot, pkg))
		for _, l := range ls {
			for test, r := range reads {
				if !l.Re.MatchString(test) {
					continue
				}
				for _, problem := range r.problems {
					t.Errorf("%s:%d: %s: %s", beforeReadyFile, l.N, test, problem)
				}
				for _, p := range r.paths {
					resolved := resolveFromRepo(pkg, p)
					if inside := "backend/" + pkg; resolved == inside || strings.HasPrefix(resolved, inside+"/") {
						continue // it comes back into the package
					}
					if !beforereadylist.Covered(resolved, l.Inputs) {
						t.Errorf("%s:%d: %s reads %q (%s from the repository root) but the line does not name it in its inputs: add it", beforeReadyFile, l.N, test, p, resolved)
					}
				}
			}
		}
	}
}

// TestBeforeReadyWalkSkipRule exercises the walk of TestBeforeReadyListIsWhole on a fixture tree
// (testdata/walkfixture): the generated package at the root (gen), testdata and hidden
// directories are skipped, and a directory named gen deeper down is not.
func TestBeforeReadyWalkSkipRule(t *testing.T) {
	t.Parallel()
	root := filepath.Join("testdata", "walkfixture")
	dirs, err := testDirs(root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range dirs {
		rel, _ := filepath.Rel(root, d)
		got = append(got, filepath.ToSlash(rel))
	}
	sort.Strings(got)
	if want := []string{".", "a/gen", "plain"}; !slices.Equal(got, want) {
		t.Errorf("directories with tests = %q, want %q", got, want)
	}
}

// TestBeforeReadyHelpersMustExist: a helper line names a plain function the package
// declares, as a run line names a test that exists. The fixture (testdata/pinfixture) declares
// readIt; a helper line that still says the old name after a rename must fail the guard.
func TestBeforeReadyHelpersMustExist(t *testing.T) {
	t.Parallel()
	declared := declaredFuncs(parseDir(t, filepath.Join("testdata", "pinfixture")).files)
	lines, err := beforereadylist.Parse(strings.NewReader("helper\ttestdata/pinfixture\treadIt\tpins\nhelper\ttestdata/pinfixture\treadItOldName\tpins\nrun\ttestdata/pinfixture\t^TestPlain$\tregen\t-\n"))
	if err != nil {
		t.Fatal(err)
	}
	got := undeclaredHelpers(lines, declared)
	if len(got) != 1 || !strings.Contains(got[0], "readItOldName") || !strings.Contains(got[0], "before_ready.txt:2") {
		t.Errorf("problems = %q, want one naming readItOldName at line 2", got)
	}
	// A method is declared, but the guard follows plain calls only, so the line would mark nothing.
	lines, err = beforereadylist.Parse(strings.NewReader("helper\ttestdata/pinfixture\tload\tpins\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := undeclaredHelpers(lines, declared); len(got) != 1 || !strings.Contains(got[0], "load") {
		t.Errorf("a helper line naming a method: problems = %q, want one", got)
	}
}
