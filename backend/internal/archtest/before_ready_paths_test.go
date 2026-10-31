package archtest

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/archtest/beforereadylist"
)

// TestBeforeReadyUpwardPathForms keeps the eyes of TestBeforeReadyInputsAreComplete honest on a
// fixture package (testdata/pathfixture), one test for each form in which a path that leaves
// the package directory is written: a literal, a constant, a concatenation with a base, a Join
// with a base and a later `..`, a Join of several arguments, a Join whose tail is only known at
// run time, a path that resolves to the repository root, a package variable and a local one.
// Each is resolved from its base, as the file system would.
func TestBeforeReadyUpwardPathForms(t *testing.T) {
	t.Parallel()
	got := upwardReads(t, filepath.Join("testdata", "pathfixture"))
	want := map[string][]string{
		"TestLiteral":                      {up(2, "NOTICE")},
		"TestConstant":                     {up(3, "proto/x.json")},
		"TestConcatenationWithABase":       {up(3, "proto/y.json")},
		"TestJoinWithABaseAndALaterDotDot": {up(2, "docs/z.md")},
		"TestJoinOfSeveralArguments":       {up(2, "migrations/sql/a.sql")},
		"TestJoinWithADynamicTail":         {up(2, "docs")},
		"TestAPathResolvingToTheRoot":      {up(3, "")},
		"TestThroughAPackageVariable":      {up(2, "data")},
		"TestThroughALocalVariable":        {up(3, "go.work")},
		"TestAGlobInAConcatenation":        {up(2, "migrations")},
		"TestOwnDirectory":                 nil,
	}
	for name, paths := range want {
		if !slices.Equal(got[name].paths, paths) {
			t.Errorf("%s reaches %q, want %q", name, got[name].paths, paths)
		}
		if len(got[name].problems) != 0 {
			t.Errorf("%s: unexpected problems %q", name, got[name].problems)
		}
	}
	// From a package two levels under backend/, up(3, "") is the repository root.
	if rootPaths := got["TestAPathResolvingToTheRoot"].paths; len(rootPaths) != 1 {
		t.Errorf("TestAPathResolvingToTheRoot reaches %q, want one path", rootPaths)
	} else if root := resolveFromRepo("internal/x", rootPaths[0]); root != "." {
		t.Errorf("the root path resolves to %q, want %q", root, ".")
	}
}

// A path that goes up from a base the guard cannot resolve fails the guard, naming the file, the
// line and the form: it never passes in silence.
func TestBeforeReadyUnresolvedPathsFail(t *testing.T) {
	t.Parallel()
	got := upwardReads(t, filepath.Join("testdata", "badpathfixture"))
	for _, name := range []string{"TestUnknownBaseInAJoin", "TestUnknownBaseInAConcatenation", "TestBaseIsAParameter"} {
		probs := got[name].problems
		if len(probs) != 1 || !strings.Contains(probs[0], "bad_test.go:") || !strings.Contains(probs[0], "cannot be resolved") {
			t.Errorf("%s: problems = %q, want one naming bad_test.go:LINE and the form", name, probs)
		}
	}
	if r := got["TestResolvable"]; len(r.problems) != 0 || !slices.Equal(r.paths, []string{up(1, "ok.json")}) {
		t.Errorf("TestResolvable = %+v", r)
	}
}

// pathReads is what the tests of one package read outside it, per Test function.
type pathReads struct {
	paths    []string // paths that go up from the package directory, cleaned, as seen from it
	problems []string // paths the guard could not resolve: file:line and the form
}

// resolveFromRepo turns a path seen from the directory of package pkg (under backend/) into a
// slash path from the repository root ("." is the root itself).
func resolveFromRepo(pkg, p string) string { return path.Join("backend", pkg, p) }

// upSegment matches a path that goes up: "..", "../x", "a/../b", "/../x".
var upSegment = regexp.MustCompile(`(^|/)\.\.(/|$)`)

// pathVal is a path expression read as far as it is known: the string it starts with, and
// whether something only known at run time follows.
type pathVal struct {
	s    string
	dyn  bool       // a part that is only known at run time follows s
	cut  bool       // s is a piece of a string, not a directory: keep its directory
	rest []ast.Expr // the parts that could not be resolved
}

// pathEval resolves path expressions: string literals, constants and variables that are
// assigned once (in the function, or in the package), concatenations with +, and the
// arguments of filepath.Join and path.Join, all of them.
type pathEval struct {
	pkg, local map[string]ast.Expr // name -> its value; nil when it has several
	depth      int
}

const maxEvalDepth = 12

// single returns the value a name was defined with exactly once, in the function or in the
// package, or nil.
func (e *pathEval) single(name string) ast.Expr {
	if v, ok := e.local[name]; ok {
		return v
	}
	return e.pkg[name]
}

// isLocal says whether the function defines the name.
func (e *pathEval) isLocal(name string) bool {
	_, ok := e.local[name]
	return ok
}

func (e *pathEval) eval(x ast.Expr) (pathVal, bool) {
	if e.depth++; e.depth > maxEvalDepth {
		e.depth--
		return pathVal{}, false
	}
	defer func() { e.depth-- }()
	switch v := x.(type) {
	case *ast.ParenExpr:
		return e.eval(v.X)
	case *ast.BasicLit:
		if s, err := strconv.Unquote(v.Value); v.Kind == token.STRING && err == nil {
			return pathVal{s: s}, true
		}
	case *ast.Ident:
		init, known := e.local[v.Name]
		if !known {
			init, known = e.pkg[v.Name]
		}
		if known && init != nil {
			// What the name holds is read where it is assigned, not here.
			if v, ok := e.eval(init); ok {
				return v, true
			}
		}
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			return e.concat(v)
		}
	case *ast.CallExpr:
		return e.call(v)
	}
	return pathVal{rest: []ast.Expr{x}}, false
}

func (e *pathEval) concat(b *ast.BinaryExpr) (pathVal, bool) {
	l, ok := e.eval(b.X)
	if !ok {
		return pathVal{rest: []ast.Expr{b}}, false
	}
	if l.dyn {
		l.rest = append(slices.Clone(l.rest), b.Y)
		return l, true
	}
	r, ok := e.eval(b.Y)
	if !ok {
		return pathVal{s: l.s, dyn: true, cut: true, rest: r.rest}, l.s != ""
	}
	return pathVal{s: l.s + r.s, dyn: r.dyn, cut: r.dyn, rest: r.rest}, true
}

// call resolves filepath.Join and path.Join (every argument), and filepath.FromSlash, ToSlash
// and Clean (their argument).
func (e *pathEval) call(c *ast.CallExpr) (pathVal, bool) {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	pkg, isIdent := ast.Expr(nil), false
	if ok {
		pkg = sel.X
		id, isID := pkg.(*ast.Ident)
		isIdent = isID && (id.Name == "filepath" || id.Name == "path")
	}
	if !isIdent {
		return pathVal{rest: []ast.Expr{c}}, false
	}
	switch sel.Sel.Name {
	case "FromSlash", "ToSlash", "Clean":
		if len(c.Args) == 1 {
			return e.eval(c.Args[0])
		}
	case "Join":
		s := ""
		for i, a := range c.Args {
			v, ok := e.eval(a)
			if !ok {
				return pathVal{s: s, dyn: true, rest: c.Args[i:]}, s != ""
			}
			piece := v.s
			if v.dyn && v.cut {
				piece = path.Dir(piece + "x")
			}
			s = joinPath(s, piece)
			if v.dyn {
				return pathVal{s: s, dyn: true, rest: v.rest}, s != ""
			}
		}
		return pathVal{s: s}, true
	}
	return pathVal{rest: []ast.Expr{c}}, false
}

func joinPath(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "/" + b
}

// goesUp says whether a cleaned path leaves its directory.
func goesUp(p string) bool { return p == ".." || strings.HasPrefix(p, "../") }

// hasUpLiteral says whether a node holds a string literal with a `..` segment.
func hasUpLiteral(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		if lit, ok := x.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if s, err := strconv.Unquote(lit.Value); err == nil && upSegment.MatchString(s) {
				found = true
			}
		}
		return !found
	})
	return found
}

// cutAtWildcard keeps the directories of a pattern before the first wildcard.
func cutAtWildcard(p string) string {
	i := strings.IndexAny(p, "*?[")
	if i < 0 {
		return p
	}
	return path.Dir(p[:i] + "x")
}

// pathScan collects, for one node of the package (a function, a method, a package-level name),
// the upward paths it holds, the problems, and the nodes it mentions.
type pathScan struct {
	ev       *pathEval
	fset     *token.FileSet
	route    func(name string) *pathScan // the node of a name, to hand it what is assigned to it
	paths    []string
	problems []string
	uses     []string
}

// pathExpr handles a path expression and says whether it did: the caller does not look inside.
// An expression it cannot resolve that writes a `..` is a problem, never a pass.
func (sc *pathScan) pathExpr(x ast.Expr) bool {
	v, ok := sc.ev.eval(x)
	if !ok || v.dyn {
		for _, r := range v.rest {
			if hasUpLiteral(r) {
				pos := sc.fset.Position(x.Pos())
				sc.problems = append(sc.problems, fmt.Sprintf("%s:%d: the path %s cannot be resolved: it goes up from a base the guard cannot read; write it from a literal, a constant or a variable assigned once", filepath.Base(pos.Filename), pos.Line, types.ExprString(x)))
				return true
			}
		}
	}
	if !ok {
		return false
	}
	s := v.s
	if v.dyn && v.cut {
		s = path.Dir(s + "x")
	}
	if clean := path.Clean(cutAtWildcard(s)); s != "" && !path.IsAbs(clean) && goesUp(clean) {
		sc.paths = append(sc.paths, clean)
	}
	for _, r := range v.rest {
		sc.walk(r)
	}
	return true
}

// walk visits a node for its paths and the names it mentions.
func (sc *pathScan) walk(root ast.Node) {
	ast.Inspect(root, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			sc.assign(v.Lhs, v.Rhs, v.Tok == token.DEFINE)
			return false
		case *ast.ValueSpec:
			lhs := make([]ast.Expr, len(v.Names))
			for i, id := range v.Names {
				lhs[i] = id
			}
			sc.assign(lhs, v.Values, true)
			return false
		case *ast.SelectorExpr:
			sc.uses = append(sc.uses, v.Sel.Name)
			sc.walk(v.X)
			return false
		case *ast.CallExpr:
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Join" {
				return !sc.pathExpr(v)
			}
		case *ast.BinaryExpr:
			if v.Op == token.ADD {
				return !sc.pathExpr(v)
			}
		case *ast.BasicLit:
			if v.Kind == token.STRING {
				sc.pathExpr(v)
			}
		case *ast.Ident:
			if !sc.pathExpr(v) && !sc.ev.isLocal(v.Name) {
				sc.uses = append(sc.uses, v.Name)
			}
		}
		return true
	})
}

// assign visits the values of an assignment or declaration (decl: it defines its names). A name
// defined once, whose value resolves, is not a path by itself (it may be only the base of one):
// it is read where it is used. Every other value is read where it is written, so that a path
// assigned to a name that is defined twice, to a field or an element, or to a package variable
// from another function, is never dropped. What is assigned to a package variable (or a
// parameter) from a function is handed to the node of that name, which every user of the name
// reaches.
func (sc *pathScan) assign(lhs, rhs []ast.Expr, decl bool) {
	for _, l := range lhs {
		if _, ok := l.(*ast.Ident); !ok {
			sc.walk(l)
		}
	}
	for i, r := range rhs {
		var id *ast.Ident
		if len(lhs) == len(rhs) {
			id, _ = lhs[i].(*ast.Ident)
		}
		if id == nil || id.Name == "_" {
			sc.walk(r)
			continue
		}
		switch v, ok := sc.ev.eval(r); {
		case decl && ok && !v.dyn && sc.ev.single(id.Name) == r:
			// read where the name is used
		case !decl && !sc.ev.isLocal(id.Name):
			sub := &pathScan{ev: sc.ev, fset: sc.fset, route: sc.route}
			sub.walk(r)
			target := sc.route(id.Name)
			target.paths = append(target.paths, sub.paths...)
			target.problems = append(target.problems, sub.problems...)
			target.uses = append(target.uses, sub.uses...)
		default:
			sc.walk(r)
		}
	}
}

// singleValues maps the names a node assigns exactly once to their value, and the others to nil.
func singleValues(into map[string]ast.Expr, name string, value ast.Expr) {
	if _, seen := into[name]; seen {
		into[name] = nil
		return
	}
	into[name] = value
}

// localValues are the names a function body assigns, with their value when there is one.
func localValues(body ast.Node) map[string]ast.Expr {
	out := map[string]ast.Expr{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			for i, l := range v.Lhs {
				id, ok := l.(*ast.Ident)
				if !ok || id.Name == "_" {
					continue
				}
				switch {
				case v.Tok != token.DEFINE:
					// An assignment to a name the function defined makes it several; to one
					// it did not define (a package variable, a parameter) it defines nothing.
					if _, defined := out[id.Name]; defined {
						out[id.Name] = nil
					}
				case len(v.Lhs) == len(v.Rhs):
					singleValues(out, id.Name, v.Rhs[i])
				default:
					out[id.Name] = nil
				}
			}
		case *ast.RangeStmt:
			for _, x := range []ast.Expr{v.Key, v.Value} {
				if id, ok := x.(*ast.Ident); ok {
					out[id.Name] = nil
				}
			}
		case *ast.ValueSpec:
			for i, id := range v.Names {
				if len(v.Names) == len(v.Values) {
					singleValues(out, id.Name, v.Values[i])
				} else {
					out[id.Name] = nil
				}
			}
		}
		return true
	})
	return out
}

// markAssignedPackageVars sets to "several" (nil) the package variables a function body assigns
// without having defined them itself.
func markAssignedPackageVars(body ast.Node, pkgValues map[string]ast.Expr) {
	locals := localValues(body)
	ast.Inspect(body, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok && as.Tok != token.DEFINE {
			for _, l := range as.Lhs {
				id, isID := l.(*ast.Ident)
				if !isID {
					continue
				}
				if _, local := locals[id.Name]; !local {
					if _, pkg := pkgValues[id.Name]; pkg {
						pkgValues[id.Name] = nil
					}
				}
			}
		}
		return true
	})
}

// upwardReads returns, for each Test function of the package in dir, the paths that go up from
// the package directory and that it reaches, and the paths it could not resolve. A path is
// resolved from its base, as the file system would: the string literals of its body, the
// constants and the variables assigned once (in the function or in the package), the
// concatenations with +, and every argument of filepath.Join and path.Join; a Join whose tail
// is only known at run time reaches its directory. It follows the functions and methods (by
// name) the test calls, in test and non-test files, and the package-level names it uses. Its
// limits: a path built by a function call, read in another package or through an embed, or in a
// file a build tag leaves out, is not seen, unless it writes a `..` the guard cannot place: then
// it is a problem.
func upwardReads(t *testing.T, dir string) map[string]pathReads {
	t.Helper()
	pd := parseDir(t, dir)
	pkgValues := map[string]ast.Expr{}
	for _, file := range pd.files {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || (gd.Tok != token.CONST && gd.Tok != token.VAR) {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, id := range vs.Names {
					if len(vs.Names) == len(vs.Values) {
						singleValues(pkgValues, id.Name, vs.Values[i])
					} else {
						pkgValues[id.Name] = nil
					}
				}
			}
		}
	}
	// A package variable that a function assigns is several values, not the one it was declared with.
	for _, file := range pd.files {
		for _, decl := range file.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok && fd.Body != nil {
				markAssignedPackageVars(fd.Body, pkgValues)
			}
		}
	}
	nodes := map[string]*pathScan{} // node name -> what it holds
	var scanOf func(name string) *pathScan
	scanOf = func(name string) *pathScan {
		if nodes[name] == nil {
			nodes[name] = &pathScan{fset: pd.fset, route: scanOf}
		}
		return nodes[name]
	}
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
				sc := scanOf(d.Name.Name)
				sc.ev = &pathEval{pkg: pkgValues, local: localValues(d.Body)}
				sc.walk(d.Body)
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for j, id := range vs.Names {
						sc := scanOf(id.Name)
						sc.ev = &pathEval{pkg: pkgValues}
						if len(vs.Names) == len(vs.Values) {
							sc.assign([]ast.Expr{id}, []ast.Expr{vs.Values[j]}, true)
						} else {
							sc.assign(nil, vs.Values, true)
						}
					}
				}
			}
		}
	}
	out := map[string]pathReads{}
	for _, test := range tests {
		seen := map[string]bool{}
		var r pathReads
		for stack := []string{test}; len(stack) > 0; {
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if seen[n] || nodes[n] == nil {
				continue
			}
			seen[n] = true
			r.paths = append(r.paths, nodes[n].paths...)
			r.problems = append(r.problems, nodes[n].problems...)
			stack = append(stack, nodes[n].uses...)
		}
		sort.Strings(r.paths)
		sort.Strings(r.problems)
		r.paths, r.problems = slices.Compact(r.paths), slices.Compact(r.problems)
		out[test] = r
	}
	return out
}

// TestBeforeReadyUpwardPathsDetector keeps the older cases honest on testdata/upfixture: a path
// that goes up is found written in the test, through a constant, through a helper, cut before a
// wildcard and as the arguments of Join, in a method and in a non-test file, and a path inside
// the package's own directory is not one.
func TestBeforeReadyUpwardPathsDetector(t *testing.T) {
	t.Parallel()
	got := upwardReads(t, filepath.Join("testdata", "upfixture"))
	want := map[string][]string{
		"TestDirect":              {up(1, "data/a.txt")},
		"TestThroughAConst":       {up(2, "proto/x.json")},
		"TestThroughAHelper":      {up(2, "NOTICE")},
		"TestThroughAGlob":        {up(2, "migrations")},
		"TestThroughAJoin":        {up(2, "docs/x.md")},
		"TestThroughAMethod":      {up(2, "web/m.json")},
		"TestThroughANonTestFile": {up(2, "LICENSE")},
		"TestOwnDirectory":        nil,
	}
	for name, paths := range want {
		if !slices.Equal(got[name].paths, paths) {
			t.Errorf("%s reaches %q, want %q", name, got[name].paths, paths)
		}
	}
	if !beforereadylist.Covered("backend/migrations/0001.sql", []string{"backend/migrations"}) || beforereadylist.Covered("backend/migrations2/x", []string{"backend/migrations"}) {
		t.Error("Covered must take a directory and what is under it, not a name that starts the same")
	}
}

// up writes a path that goes up n directories and then down rest, as the cleaned form the
// detector reports. The expected values of the detector tests are written with it so that they
// hold no ".." literal that reaches outside backend/ (the guard reads this file too); the few
// literals that remain (goesUp and its table, upSegment) stay inside backend/.
func up(n int, rest string) string {
	parts := slices.Repeat([]string{".."}, n)
	return path.Join(append(parts, rest)...)
}

// A path written in an assignment is read even when the name it is assigned to is assigned more
// than once, defined in two subtests, set through a field or an element, set in init or
// reassigned in a test (testdata/assignfixture): the value is a path where it is written, not
// only where one name stands for it.
func TestBeforeReadyUpwardPathsAssignedMoreThanOnce(t *testing.T) {
	t.Parallel()
	got := upwardReads(t, filepath.Join("testdata", "assignfixture"))
	want := map[string][]string{
		"TestTwoSubtestsDefineTheSameName": {up(2, "docs"), up(2, "proto")},
		"TestAssignedToAFieldOrAnElement":  {up(2, "web"), up(1, "x")},
		"TestReadsWhatInitSet":             {up(2, "")},
		"TestReassignsAPackageVariable":    {up(1, "x")},
		"TestOnlyOwnDirectory":             nil,
	}
	for name, paths := range want {
		r, found := got[name]
		if !found {
			t.Errorf("the fixture has no %s: the test was renamed or removed", name)
			continue
		}
		if !slices.Equal(r.paths, paths) {
			t.Errorf("%s reaches %q, want %q", name, r.paths, paths)
		}
		if len(r.problems) != 0 {
			t.Errorf("%s: unexpected problems %q", name, r.problems)
		}
	}
}

func TestGoesUpOnlyForWholeSegments(t *testing.T) {
	t.Parallel()
	for p, want := range map[string]bool{"..": true, "../x": true, "../../x": true, "..x": false, "..a/b": false, "a/b": false, ".": false, "": false} {
		if got := goesUp(p); got != want {
			t.Errorf("goesUp(%q) = %v, want %v", p, got, want)
		}
	}
}
