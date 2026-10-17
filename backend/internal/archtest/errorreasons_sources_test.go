package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// srcFile is a non-test Go file under internal/, parsed.
type srcFile struct {
	rel  string // path under internal/, with slashes
	fset *token.FileSet
	file *ast.File
}

// parseSources parses every non-test Go file under internal/ once per call.
func parseSources(t *testing.T) []srcFile {
	t.Helper()
	root := filepath.Join(backendRoot, "internal")
	var out []srcFile
	err := fs.WalkDir(os.DirFS(root), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, filepath.Join(root, p), nil, parser.SkipObjectResolution)
		if perr != nil {
			return perr
		}
		out = append(out, srcFile{rel: p, fset: fset, file: file})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// unmigratedBodyReasons are the call sites that still spell a body reason as
// a string or a package constant instead of an rpcerr.Reason. Each names the
// package (a directory under internal/), the reason and who moves it. The
// value must still be in the list; only the spelling is excused. An entry no
// source uses any more fails the test, so the migration removes it.
var unmigratedBodyReasons = []struct{ pkg, reason string }{
	// TODO(maps): internal/maps/httperror.go constants and tiles.go, ratelimit.go literals.
	{"maps", "UNSUPPORTED_TYPE"}, {"maps", "TOO_LARGE"}, {"maps", "DIMENSIONS"}, {"maps", "CORRUPT"},
	{"maps", "MALFORMED_REQUEST"}, {"maps", "QUOTA"}, {"maps", "BUSY"}, {"maps", "RATE_LIMITED"},
	// TODO(play): internal/play/messages_voice.go literals.
	{"play", "OFF"}, {"play", "DAILY_LIMIT_REACHED"}, {"play", "UNAVAILABLE"}, {"play", "REFUSED"}, {"play", "RATE_LIMITED"},
}

// errorTypes are the struct types whose `reason` field reaches a client in the
// JSON body of an HTTP route's error.
var errorTypes = map[string]bool{"httpError": true, "voiceError": true}

// TestErrorReasonsOnlyListedBodyReasons fails when Go code builds the error
// body of an HTTP route with a reason the list does not have, or spells a
// listed reason without the rpcerr.Reason registry (outside the named
// allowlist), or writes a reason into raw JSON text.
func TestErrorReasonsOnlyListedBodyReasons(t *testing.T) {
	t.Parallel()
	listed := map[string]string{} // reason -> code
	for _, b := range loadReasons(t).Body {
		listed[b.Reason] = b.Code
	}
	registry := registryReasons(t)
	consts := packageConsts(parseSources(t))
	seen := map[string]bool{}
	for _, src := range parseSources(t) {
		pkg := path.Dir(src.rel)
		if pkg == "platform/rpcerr" {
			continue
		}
		for _, site := range bodyReasonSites(src, consts) {
			where := src.fset.Position(site.pos).String()
			switch {
			case site.registry != "":
				if _, ok := registry[site.registry]; !ok {
					t.Errorf("%s: rpcerr.%s is not a registered Reason", where, site.registry)
				}
			case site.raw:
				t.Errorf("%s: a reason is written into JSON text; use rpcerr.Reason", where)
			case site.value == "":
				// A reason passed through from a parameter or a field.
			default:
				if _, ok := listed[site.value]; !ok {
					t.Errorf("%s: reason %q is not in proto/error-reasons.json", where, site.value)
				}
				key := strings.SplitN(pkg, "/", 2)[0] + "/" + site.value
				if !slices.ContainsFunc(unmigratedBodyReasons, func(u struct{ pkg, reason string }) bool { return u.pkg+"/"+u.reason == key }) {
					t.Errorf("%s: spell reason %q as an rpcerr.Reason constant, not a literal", where, site.value)
				}
				seen[key] = true
			}
		}
	}
	for _, u := range unmigratedBodyReasons {
		if !seen[u.pkg+"/"+u.reason] {
			t.Errorf("unmigratedBodyReasons names %s %s, which no source spells any more: remove the entry", u.pkg, u.reason)
		}
	}
}

// bodySite is one place that sets a body reason: a registry constant, a
// literal or constant value, raw JSON text, or a pass-through (value "").
type bodySite struct {
	pos      token.Pos
	registry string
	value    string
	raw      bool
}

var rawReason = regexp.MustCompile(`"reason":\s*"[A-Z_]+"`)

// bodyReasonSites finds the body reasons a file sets: the `reason` field of
// the error types, a "reason" key of a map literal, an invalid(reason, ...)
// call of package maps and JSON text with a reason in it.
func bodyReasonSites(src srcFile, consts map[string]string) []bodySite {
	var out []bodySite
	pkg := path.Dir(src.rel)
	classify := func(e ast.Expr) bodySite {
		return classifyReason(e, pkg, src.file, consts)
	}
	ast.Inspect(src.file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CompositeLit:
			out = append(out, compositeReasons(x, classify)...)
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "invalid" && pkg == "maps" && len(x.Args) == 2 {
				out = append(out, classify(x.Args[0]))
			}
		case *ast.BasicLit:
			if x.Kind == token.STRING && rawReason.MatchString(x.Value) {
				out = append(out, bodySite{pos: x.Pos(), raw: true})
			}
		}
		return true
	})
	return out
}

// compositeReasons is the reason a struct literal of an error type, or a map
// literal with a "reason" key, sets.
func compositeReasons(lit *ast.CompositeLit, classify func(ast.Expr) bodySite) []bodySite {
	var out []bodySite
	// A map[string]string with a "reason" key is a JSON error body; a
	// map[string]any is an event's payload, which is not an error reason.
	isMap := false
	if mt, ok := lit.Type.(*ast.MapType); ok {
		if v, ok := mt.Value.(*ast.Ident); ok && v.Name == "string" {
			isMap = true
		}
	}
	typeName := ""
	if id, ok := lit.Type.(*ast.Ident); ok {
		typeName = id.Name
	}
	if !isMap && !errorTypes[typeName] {
		return nil
	}
	for _, el := range lit.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		switch key := kv.Key.(type) {
		case *ast.Ident:
			if key.Name == "reason" && !isMap {
				out = append(out, classify(kv.Value))
			}
		case *ast.BasicLit:
			if unquoted, _ := strconv.Unquote(key.Value); unquoted == "reason" && isMap {
				out = append(out, classify(kv.Value))
			}
		}
	}
	return out
}

// classifyReason says what an expression that sets a reason is.
func classifyReason(e ast.Expr, pkg string, file *ast.File, consts map[string]string) bodySite {
	site := bodySite{pos: e.Pos()}
	switch x := e.(type) {
	case *ast.BasicLit:
		site.value, _ = strconv.Unquote(x.Value)
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok && id.Name == "rpcerr" {
			site.registry = x.Sel.Name
		}
	case *ast.Ident:
		site.value = consts[pkg+"."+x.Name]
	case *ast.CallExpr:
		// rpcerr.Reason("X") or string(rpcerr.ReasonX): look inside.
		if len(x.Args) == 1 {
			return classifyReason(x.Args[0], pkg, file, consts)
		}
	}
	return site
}

// packageConsts is every package-level string constant under internal/, by
// "<directory>.<name>".
func packageConsts(files []srcFile) map[string]string {
	out := map[string]string{}
	for _, f := range files {
		for name, value := range stringConsts(f.file) {
			out[path.Dir(f.rel)+"."+name] = value
		}
	}
	return out
}

// detailTypes are the packages' import aliases for the generated messages.
var genImport = regexp.MustCompile(`/gen/meurpg/([a-z]+)/v1$`)

// TestErrorReasonsOnlyListedDetails fails when Go code attaches an error
// detail the list does not have. The detail's type must be readable from the
// code: a composite literal, or a variable or parameter declared with one.
func TestErrorReasonsOnlyListedDetails(t *testing.T) {
	t.Parallel()
	listed := map[string]bool{}
	for _, d := range loadReasons(t).Details {
		listed[d.Message] = true
	}
	calls := 0
	for _, src := range parseSources(t) {
		aliases := genAliases(src.file)
		for _, decl := range src.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || !isNewErrorDetail(call) || len(call.Args) != 1 {
					return true
				}
				calls++
				name, ok := messageOf(call.Args[0], fn, aliases)
				where := src.fset.Position(call.Pos()).String()
				switch {
				case !ok:
					t.Errorf("%s: cannot tell the type of the detail; build it from a literal of the generated message", where)
				case !listed[name]:
					t.Errorf("%s: detail %s is not in proto/error-reasons.json", where, name)
				}
				return true
			})
		}
	}
	if calls < 20 {
		t.Errorf("found only %d NewErrorDetail calls: the scan is not looking in the right place", calls)
	}
}

func isNewErrorDetail(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "NewErrorDetail"
}

// genAliases maps the import names of the generated packages in a file to the
// module they name ("charactersv1" -> "characters").
func genAliases(file *ast.File) map[string]string {
	out := map[string]string{}
	for _, imp := range file.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		m := genImport.FindStringSubmatch(p)
		if m == nil {
			continue
		}
		name := m[1] + "v1"
		if imp.Name != nil {
			name = imp.Name.Name
		}
		out[name] = m[1]
	}
	return out
}

// messageOf is the full proto name of the message an expression builds or a
// variable holds.
func messageOf(e ast.Expr, fn *ast.FuncDecl, aliases map[string]string) (string, bool) {
	switch x := e.(type) {
	case *ast.UnaryExpr:
		return messageOf(x.X, fn, aliases)
	case *ast.StarExpr:
		return messageOf(x.X, fn, aliases)
	case *ast.CompositeLit:
		return messageOf(x.Type, fn, aliases)
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok && aliases[id.Name] != "" {
			return "meurpg." + aliases[id.Name] + ".v1." + x.Sel.Name, true
		}
	case *ast.Ident:
		if typ := declaredType(x.Name, fn); typ != nil {
			return messageOf(typ, fn, aliases)
		}
	}
	return "", false
}

// declaredType is the type expression (or literal) a name in a function is
// declared with: a parameter, `x := &pkg.T{}` or `var x pkg.T`.
func declaredType(name string, fn *ast.FuncDecl) ast.Expr {
	for _, field := range fn.Type.Params.List {
		for _, n := range field.Names {
			if n.Name == name {
				return field.Type
			}
		}
	}
	var found ast.Expr
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for i, l := range x.Lhs {
				if id, ok := l.(*ast.Ident); ok && id.Name == name && i < len(x.Rhs) && found == nil {
					found = x.Rhs[i]
				}
			}
		case *ast.ValueSpec:
			for _, id := range x.Names {
				if id.Name == name && x.Type != nil && found == nil {
					found = x.Type
				}
			}
		}
		return true
	})
	return found
}

// TestErrorReasonsNoLiteralViolationReasons fails when a table-content
// violation is built with a string literal for its reason: use a constant of
// the list (rules.Reason* or the characters package's reason*).
func TestErrorReasonsNoLiteralViolationReasons(t *testing.T) {
	t.Parallel()
	listed := map[string]bool{}
	for _, v := range loadReasons(t).Violations {
		listed[v.Reason] = true
	}
	consts := packageConsts(parseSources(t))
	for _, src := range parseSources(t) {
		ast.Inspect(src.file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok || !isViolation(lit) {
				return true
			}
			for _, el := range lit.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if key, isIdent := kv.Key.(*ast.Ident); !ok || !isIdent || key.Name != "Reason" {
					continue
				}
				where := src.fset.Position(kv.Pos()).String()
				site := classifyReason(kv.Value, path.Dir(src.rel), src.file, consts)
				if _, isLit := kv.Value.(*ast.BasicLit); isLit {
					t.Errorf("%s: a violation reason is a literal; use a constant", where)
				} else if site.value != "" && !listed[site.value] {
					t.Errorf("%s: violation reason %q is not in proto/error-reasons.json", where, site.value)
				}
			}
			return true
		})
	}
}

func isViolation(lit *ast.CompositeLit) bool {
	switch x := lit.Type.(type) {
	case *ast.SelectorExpr:
		return x.Sel.Name == "TableContentViolation"
	case *ast.StarExpr:
		sel, ok := x.X.(*ast.SelectorExpr)
		return ok && sel.Sel.Name == "TableContentViolation"
	}
	return false
}
