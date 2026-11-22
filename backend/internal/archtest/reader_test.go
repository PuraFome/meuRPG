package archtest

import (
	_ "embed"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

// readerExceptions lists the places of the play module that ask about the caller's role without the reader.
// An entry is "file function kind count": the count is part of the entry, so one more place is a new entry.
// The queue (an entry with no reason) can only shrink; "permanent:" entries are the reader's own functions.
//
//go:embed reader_exceptions.txt
var readerExceptions string

// readerDirs lists the package roots whose questions about the caller's role go through the reader.
// The roots are read with their subdirectories, but not playdb (generated) or testdata.
// The maps and characters modules are not scanned.
var readerDirs = []string{"internal/play"}

// readerSkipDirs are the directories under a root that the scan leaves out.
var readerSkipDirs = []string{"playdb"}

var generatedHeaderRe = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

// TestRoleQuestionsGoThroughTheReader keeps the questions about the caller's role inside the reader of the play module.
// It reads the non-test, non-generated Go files of internal/play and its subdirectories (not playdb).
// It counts, per function, four kinds of place:
// role (a comparison with authz.RoleMaster or authz.RolePlayer, or a case clause naming one),
// flag (a read of a field named master),
// mint (a combatViewer or an authz.Membership with a Role built by hand, or an assignment to a Role field) and
// scene (a call of ListScenePlayers or PlayerReadsScene).
// A role passed as an argument or a value is not a question, and a function literal counts on the function that holds it.
// Each place is listed in reader_exceptions.txt with its count; a new place, or a changed count, fails until the list is edited.
// The limits: it reads names, not types, so another selector named master or Role counts, and a reader question
// written with a differently named flag is not seen.
func TestRoleQuestionsGoThroughTheReader(t *testing.T) {
	root := os.DirFS(backendRoot)
	found := map[string]string{}
	files := 0
	for _, dir := range readerDirs {
		err := walkReaderFiles(root, dir, func(p string) error {
			files++
			src, err := fs.ReadFile(root, p)
			if err != nil {
				return err
			}
			places, err := readerPlaces(p, src)
			if err != nil {
				return err
			}
			for k := range readerEntries(places) {
				found[k] = readerMessage(k)
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
	failProblems(t, compareExceptions("reader_exceptions.txt", parseExceptions(readerExceptions), found))
	t.Logf("%d non-test, non-generated Go files read", files)
}

// walkReaderFiles calls fn for every non-test .go file under dir, skipping the directories the go tool ignores and
// the generated ones (readerSkipDirs).
func walkReaderFiles(root fs.FS, dir string, fn func(path string) error) error {
	return fs.WalkDir(root, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirName(d.Name()) || slices.Contains(readerSkipDirs, strings.TrimPrefix(p, dir+"/")) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || isTestFile(p) {
			return nil
		}
		return fn(p)
	})
}

// readerEntries turns the places of one file ("file function kind" -> count) into entry lines:
// "file function kind count", each with its count.
func readerEntries(places map[string]int) map[string]int {
	out := make(map[string]int, len(places))
	for k, n := range places {
		out[k+" "+strconv.Itoa(n)] = n
	}
	return out
}

// readerMessage is what a failure says about an entry line.
func readerMessage(entry string) string {
	f := strings.Fields(entry)
	kind := f[len(f)-2]
	switch kind {
	case "mint":
		return entry + ": a reader built by hand: build the reader with viewerOf, viewerFor or combatTx.viewer; " +
			"or add this line to reader_exceptions.txt: " + entry
	case "scene":
		return entry + ": a read of the scene's players of its own: read the scene's players with sceneViewersOf or readsScene; " +
			"or add this line to reader_exceptions.txt: " + entry
	}
	return entry + ": a new question about the caller's role: ask the reader: a combatViewer question (sees, owns, readsHitPointsOf), " +
		"or a new one written beside them; or add this line to reader_exceptions.txt: " + entry
}

// readerPlaces counts the role, flag, mint and scene places of one file, per function: "path function kind" -> count.
// A function literal counts on the function that holds it, a package-level declaration under "var".
// A generated file has none.
func readerPlaces(path string, src []byte) (map[string]int, error) {
	f, err := parser.ParseFile(token.NewFileSet(), path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	if end := int(f.Package) - 1; end > 0 && end <= len(src) && generatedHeaderRe.Match(src[:end]) {
		return nil, nil
	}
	places := map[string]int{}
	for _, decl := range f.Decls {
		var fn string
		switch d := decl.(type) {
		case *ast.FuncDecl:
			fn = funcName(d)
		case *ast.GenDecl:
			if d.Tok != token.VAR && d.Tok != token.CONST {
				continue
			}
			fn = "var"
		default:
			continue
		}
		countReaderPlaces(decl, func(kind string) { places[path+" "+fn+" "+kind]++ })
	}
	return places, nil
}

// countReaderPlaces calls add once for each place in the node.
func countReaderPlaces(n ast.Node, add func(kind string)) {
	notRead := map[*ast.SelectorExpr]bool{}
	ast.Inspect(n, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BinaryExpr:
			if (x.Op == token.EQL || x.Op == token.NEQ) && (isRoleSel(x.X) || isRoleSel(x.Y)) {
				add("role")
			}
		case *ast.CaseClause:
			if slices.ContainsFunc(x.List, isRoleSel) {
				add("role")
			}
		case *ast.SelectorExpr:
			if x.Sel.Name == "master" && !notRead[x] {
				add("flag")
			}
		case *ast.AssignStmt:
			countAssign(x, notRead, add)
		case *ast.CompositeLit:
			if isMint(x) {
				add("mint")
			}
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok && (sel.Sel.Name == "ListScenePlayers" || sel.Sel.Name == "PlayerReadsScene") {
				add("scene")
			}
		}
		return true
	})
}

// countAssign handles an assignment: a plain write of a master field is not a read, a write of a Role field is a mint.
func countAssign(a *ast.AssignStmt, notRead map[*ast.SelectorExpr]bool, add func(kind string)) {
	for _, l := range a.Lhs {
		sel, ok := l.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		if sel.Sel.Name == "master" && (a.Tok == token.ASSIGN || a.Tok == token.DEFINE) {
			notRead[sel] = true
		}
		if sel.Sel.Name == "Role" {
			add("mint")
		}
	}
}

// isRoleSel reports whether the expression is authz.RoleMaster or authz.RolePlayer.
func isRoleSel(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "authz" && (sel.Sel.Name == "RoleMaster" || sel.Sel.Name == "RolePlayer")
}

// isMint reports whether a composite literal builds a reader by hand: a combatViewer, or an authz.Membership with a Role.
func isMint(c *ast.CompositeLit) bool {
	switch t := c.Type.(type) {
	case *ast.Ident:
		return t.Name == "combatViewer"
	case *ast.SelectorExpr:
		if pkg, ok := t.X.(*ast.Ident); !ok || pkg.Name != "authz" || t.Sel.Name != "Membership" {
			return false
		}
		for _, el := range c.Elts {
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Role" {
					return true
				}
			}
		}
	}
	return false
}

func TestReaderPlaces(t *testing.T) {
	const head = "package play\nimport \"github.com/PuraFome/meuRPG/backend/internal/authz\"\n"
	const p = "internal/play/a.go"
	k := func(fn, kind string) string { return p + " " + fn + " " + kind }
	cases := []struct {
		name, src string
		want      map[string]int
	}{
		{"comparisons", "func F(m authz.Membership) bool { return m.Role == authz.RoleMaster || authz.RolePlayer != m.Role }", map[string]int{k("F", "role"): 2}},
		{"switch cases", "func F(r authz.Role) { switch r { case authz.RoleMaster: case authz.RolePlayer: } }", map[string]int{k("F", "role"): 2}},
		{"local role", "func F(role authz.Role) bool { return role == authz.RolePlayer }", map[string]int{k("F", "role"): 1}},
		{"role as argument", "func F(ctx int) { authz.RequireCampaignRole(ctx, x, authz.RoleMaster); f(authz.RolePlayer) }", nil},
		{"flag reads", "func F(v V, x bool) bool { if v.master {}; _ = v.master || x; f(v.master); return c.master }", map[string]int{k("F", "flag"): 4}},
		{"flag writes", "func F(c C) { c.master = true; _ = T{master: true} }", nil},
		{"flag read on the right", "func F(c C, d D) { c.master = d.master }", map[string]int{k("F", "flag"): 1}},
		{"op-assign reads", "func F(c C, y bool) { c.master |= y }", map[string]int{k("F", "flag"): 1}},
		{"local named master", "func F(master bool) bool { var m = master; return master && m }", nil},
		{"other names", "func F(ev E, byMaster bool) bool { return ev.ByMaster || byMaster }", nil},
		{"viewer mint", "func F(u string) { _ = combatViewer{master: true}; _ = combatViewer{}; _ = &combatViewer{userID: u} }", map[string]int{k("F", "mint"): 3}},
		{"membership with Role", "func F(u string, m authz.Membership) { _ = authz.Membership{UserID: u, Role: authz.RolePlayer}; m.Role = authz.RoleMaster }", map[string]int{k("F", "mint"): 2}},
		{"membership without Role", "func F(u string) { _ = authz.Membership{UserID: u} }", nil},
		{"scene reads", "func F(q Q) { q.ListScenePlayers(ctx, id); q.PlayerReadsScene(ctx, a) }", map[string]int{k("F", "scene"): 2}},
		{"scene helpers", "func F(q Q) { sceneViewersOf(ctx, q, s); readsScene(ctx, q, s) }", nil},
		{"function literal", "func F(v V) func() bool { return func() bool { return v.master } }", map[string]int{k("F", "flag"): 1}},
		{"package level and init", "var x = func() bool { return v.master }\nfunc init() { _ = c.master }", map[string]int{k("var", "flag"): 1, k("init", "flag"): 1}},
		{"two inits add", "func init() { _ = c.master }\nfunc init() { _ = c.master }", map[string]int{k("init", "flag"): 2}},
		{"methods apart", "func (a A) M() bool { return a.master }\nfunc (b *B) M() bool { return b.master }", map[string]int{k("(A).M", "flag"): 1, k("(*B).M", "flag"): 1}},
		{"tuple assignment writes", "func F(a, b C, x, y bool) { a.master, b.master = x, y }", nil},
		{"tuple assignment reads", "func F(c, d C) { c.master, d = d.master, c.master }", map[string]int{k("F", "flag"): 2}},
		{"case with both roles", "func F(r authz.Role) { switch r { case authz.RoleMaster, authz.RolePlayer: } }", map[string]int{k("F", "role"): 1}},
		{"generated line after package", "func F(v V) bool { return v.master }\n// Code generated by x. DO NOT EDIT.\n", map[string]int{k("F", "flag"): 1}},
		{"two kinds", "func F(v V, r authz.Role) bool { return v.master && r == authz.RoleMaster }", map[string]int{k("F", "flag"): 1, k("F", "role"): 1}},
	}
	for _, c := range cases {
		got, err := readerPlaces(p, []byte(head+c.src))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !maps.Equal(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	gen := "// Code generated by sqlc. DO NOT EDIT.\n\n" + head + "func F(v V) bool { return v.master }"
	if got, err := readerPlaces(p, []byte(gen)); err != nil || len(got) != 0 {
		t.Errorf("generated file: got %v, %v, want nothing", got, err)
	}
}

func TestReaderListComparison(t *testing.T) {
	const p = "internal/play/a.go"
	src := "package play\nfunc F(v V) bool { return v.master && w.master }"
	places, err := readerPlaces(p, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for k := range readerEntries(places) {
		found[k] = readerMessage(k)
	}
	cases := []struct {
		name   string
		listed []string
		want   int
	}{
		{"at its count", []string{p + " F flag 2"}, 0},
		{"over its count", []string{p + " F flag 1"}, 2},
		{"unlisted", nil, 1},
	}
	for _, c := range cases {
		if got := compareExceptions("reader_exceptions.txt", c.listed, found); len(got) != c.want {
			t.Errorf("%s: got %q, want %d problems", c.name, got, c.want)
		}
	}
	const add = "; or add this line to reader_exceptions.txt: "
	messages := map[string]string{
		"role":  "a new question about the caller's role: ask the reader: a combatViewer question (sees, owns, readsHitPointsOf), or a new one written beside them",
		"flag":  "a new question about the caller's role: ask the reader: a combatViewer question (sees, owns, readsHitPointsOf), or a new one written beside them",
		"mint":  "a reader built by hand: build the reader with viewerOf, viewerFor or combatTx.viewer",
		"scene": "a read of the scene's players of its own: read the scene's players with sceneViewersOf or readsScene",
	}
	for kind, msg := range messages {
		e := p + " F " + kind + " 1"
		if got, want := readerMessage(e), e+": "+msg+add+e; got != want {
			t.Errorf("%s: got %q, want %q", kind, got, want)
		}
	}
}

func TestWalkReaderFiles(t *testing.T) {
	root := fstest.MapFS{
		"internal/play/a.go":                  {},
		"internal/play/a_test.go":             {},
		"internal/play/sub/b.go":              {},
		"internal/play/sub/b_test.go":         {},
		"internal/play/sub/playdb/x.go":       {},
		"internal/play/playdb/queries.sql.go": {},
		"internal/play/testdata/c.go":         {},
		"internal/play/readme.md":             {},
	}
	var got []string
	err := walkReaderFiles(root, "internal/play", func(p string) error {
		got = append(got, p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"internal/play/a.go", "internal/play/sub/b.go", "internal/play/sub/playdb/x.go"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
