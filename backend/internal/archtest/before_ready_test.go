package archtest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
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
		pd := &parsedDir{}
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
				problems = append(problems, fmt.Sprintf("%s.%s compares with a golden, hash or fixture file but is not in %s: add a `run` line for %s (with the command that rewrites its file)", pkg, name, beforeReadyFile, pkg))
			}
		}
	}
	for _, p := range problems {
		t.Error(p)
	}
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
// the line, and every input must exist.
func TestBeforeReadyInputsAreComplete(t *testing.T) {
	t.Parallel()
	lines := readBeforeReady(t)
	for _, l := range lines {
		if l.Kind != beforereadylist.Run {
			continue
		}
		for _, in := range l.Inputs {
			if in == "" || strings.HasPrefix(in, "/") || strings.Contains(in, "..") {
				t.Errorf("%s:%d: input %q must be a path from the repository root", beforeReadyFile, l.N, in)
				continue
			}
			if _, err := os.Stat(filepath.Join(backendRoot, "..", filepath.FromSlash(in))); err != nil {
				t.Errorf("%s:%d: input %q does not exist: %v", beforeReadyFile, l.N, in, err)
			}
		}
	}
	byPkg := map[string][]beforeReadyLine{}
	for _, l := range lines {
		if l.Kind == beforereadylist.Run {
			byPkg[l.Pkg] = append(byPkg[l.Pkg], l)
		}
	}
	for pkg, ls := range byPkg {
		reads := upwardPaths(t, filepath.Join(backendRoot, pkg))
		for _, l := range ls {
			for test, paths := range reads {
				if !l.Re.MatchString(test) {
					continue
				}
				for _, p := range paths {
					if resolved := path.Join("backend", pkg, p); !beforereadylist.Covered(resolved, l.Inputs) {
						t.Errorf("%s:%d: %s reads %q (%s from the repository root) but the line does not name it in its inputs: add it", beforeReadyFile, l.N, test, p, resolved)
					}
				}
			}
		}
	}
}

// upwardPaths returns, for each Test function of the package in dir, the paths with a ".."
// segment that it reaches, as written (relative to dir) and cut before a wildcard: the string
// literals of its body, of the functions and methods (by name) it calls, in test and non-test
// files of the package, and of the package-level names it uses, and a filepath.Join("..", ...)
// as one path. Its limits: a path built at run time, read in another package or through an
// embed, or in a file a build tag leaves out, is not seen.
func upwardPaths(t *testing.T, dir string) map[string][]string {
	t.Helper()
	pd := parseDir(t, dir)
	own := map[string][]string{}  // node -> the upward paths it holds itself
	uses := map[string][]string{} // node -> the nodes (functions, package-level names) it mentions
	var tests []string
	for i, file := range pd.files {
		fileName := pd.names[i]
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Body == nil {
					continue
				}
				// A method is known by its name alone, whatever its receiver: two types
				// with the same method name share a node, which attributes more, never less.
				if d.Recv == nil && strings.HasPrefix(d.Name.Name, "Test") && strings.HasSuffix(fileName, "_test.go") {
					tests = append(tests, d.Name.Name)
				}
				o, u := scanUpward(d.Body)
				own[d.Name.Name] = append(own[d.Name.Name], o...)
				uses[d.Name.Name] = append(uses[d.Name.Name], u...)
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for _, id := range vs.Names {
							for _, v := range vs.Values {
								o, u := scanUpward(v)
								own[id.Name] = append(own[id.Name], o...)
								uses[id.Name] = append(uses[id.Name], u...)
							}
						}
					}
				}
			}
		}
	}
	out := map[string][]string{}
	for _, test := range tests {
		seen := map[string]bool{}
		for stack := []string{test}; len(stack) > 0; {
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if seen[n] {
				continue
			}
			seen[n] = true
			out[test] = append(out[test], own[n]...)
			stack = append(stack, uses[n]...)
		}
		sort.Strings(out[test])
		out[test] = slices.Compact(out[test])
	}
	return out
}

// upSegment matches a path that goes up: "..", "../x", "a/../b", "/../x".
var upSegment = regexp.MustCompile(`(^|/)\.\.(/|$)`)

// scanUpward returns the upward paths written in a node and the identifiers it mentions.
func scanUpward(n ast.Node) (paths, idents []string) {
	joined := map[ast.Node]bool{} // the ".." arguments of a Join, already counted as one path
	ast.Inspect(n, func(x ast.Node) bool {
		switch v := x.(type) {
		case *ast.Ident:
			idents = append(idents, v.Name)
		case *ast.CallExpr:
			if p := joinedUpward(v); p != "" {
				paths = append(paths, p)
				for _, a := range v.Args[:strings.Count(p, "..")] {
					joined[a] = true
				}
			}
		case *ast.BasicLit:
			if v.Kind != token.STRING || joined[v] {
				return true
			}
			if s, err := strconv.Unquote(v.Value); err == nil && upSegment.MatchString(s) {
				paths = append(paths, cutAtWildcard(s))
			}
		}
		return true
	})
	return paths, idents
}

// joinedUpward returns "../.." for filepath.Join("..", "..", "x"): the leading ".." arguments
// as the directory they reach, which is what the rest is read under. "" for any other call.
func joinedUpward(c *ast.CallExpr) string {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Join" {
		return ""
	}
	var ups []string
	for _, a := range c.Args {
		lit, ok := a.(*ast.BasicLit)
		if !ok || lit.Value != `".."` {
			break
		}
		ups = append(ups, "..")
	}
	return strings.Join(ups, "/")
}

// cutAtWildcard keeps the directories of a pattern before the first wildcard.
func cutAtWildcard(p string) string {
	i := strings.IndexAny(p, "*?[")
	if i < 0 {
		return p
	}
	return path.Dir(p[:i] + "x")
}

// TestBeforeReadyUpwardPathsDetector keeps the eyes of TestBeforeReadyInputsAreComplete honest
// on a fixture package (testdata/upfixture): a path that goes up is found written in the test,
// through a constant, through a helper, cut before a wildcard and as the arguments of Join, and
// a path inside the package's own directory is not one.
func TestBeforeReadyUpwardPathsDetector(t *testing.T) {
	t.Parallel()
	got := upwardPaths(t, filepath.Join("testdata", "upfixture"))
	want := map[string][]string{
		"TestDirect":              {"../data/a.txt"},
		"TestThroughAConst":       {"../../proto/x.json"},
		"TestThroughAHelper":      {"../../NOTICE"},
		"TestThroughAGlob":        {"../../migrations"},
		"TestThroughAJoin":        {"../.."},
		"TestThroughAMethod":      {"../../web/m.json"},
		"TestThroughANonTestFile": {"../../LICENSE"},
		"TestOwnDirectory":        nil,
	}
	for name, paths := range want {
		if !slices.Equal(got[name], paths) {
			t.Errorf("%s reaches %q, want %q", name, got[name], paths)
		}
	}
	if !beforereadylist.Covered("backend/migrations/0001.sql", []string{"backend/migrations"}) || beforereadylist.Covered("backend/migrations2/x", []string{"backend/migrations"}) {
		t.Error("covered must take a directory and what is under it, not a name that starts the same")
	}
}

// hintPath matches a repository path written in a regenerate command or a reason: under a
// top-level directory of the backend module or of the repository.
var hintPath = regexp.MustCompile(`(?:\./)?(?:internal|cmd|backend|proto|docs|migrations)/[A-Za-z0-9_./-]+`)

// TestBeforeReadyHintsNamePathsThatExist keeps the hints honest: a failure prints the fourth
// column next to the failed test, and a path in it that moved (a file named under a directory
// it is not in) sends the reader to nothing. A path is read from backend/ or from the
// repository root.
func TestBeforeReadyHintsNamePathsThatExist(t *testing.T) {
	t.Parallel()
	for _, l := range readBeforeReady(t) {
		for _, p := range hintPath.FindAllString(l.Last, -1) {
			p = strings.TrimPrefix(strings.TrimRight(p, ".,)"), "./")
			if _, err := os.Stat(filepath.Join(backendRoot, filepath.FromSlash(p))); err == nil {
				continue
			}
			if _, err := os.Stat(filepath.Join(backendRoot, "..", filepath.FromSlash(p))); err == nil {
				continue
			}
			t.Errorf("%s:%d: the hint names %q, which exists neither under backend/ nor at the repository root: fix the path", beforeReadyFile, l.N, p)
		}
	}
}
