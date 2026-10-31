package archtest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

// dicetestPackage is the deterministic dice for tests, relative to backend/.
// Production wires dice.Crypto; nothing a production build compiles may reach
// this package, so no flag, no environment variable and no config can switch
// the server to it.
const dicetestPackage = "internal/platform/dice/dicetest"

// TestNoProductionCodeImportsTheTestDice reads every non-test Go file of the
// backend module (internal/, cmd/ and the rest) and fails for one that imports
// the test dice or a package under it and is not a file of that package's own directory.
// A helper package outside it that is not a _test.go file fails too.
// An aliased, dot or blank import and a file under a build tag count as imports.
// Test files may import it.
func TestNoProductionCodeImportsTheTestDice(t *testing.T) {
	root := os.DirFS(backendRoot)
	module := modulePath(t, root)
	got, err := importersOf(root, module+"/"+dicetestPackage)
	if err != nil {
		t.Fatalf("reading the backend's source: %v", err)
	}
	for _, f := range got {
		t.Errorf("%s imports %s: only _test.go files may import the test dice, and a non-test helper package may not either; production code rolls with dice.Crypto", f, dicetestPackage)
	}
}

// TestTheTestDiceRuleSeesAnImportInProductionCode proves the rule can fail.
// Non-test files that import the package or one under it are found, whatever the import form.
// A test file, the package's own files and a file under testdata are not.
func TestTheTestDiceRuleSeesAnImportInProductionCode(t *testing.T) {
	const target = "example.test/backend/" + dicetestPackage
	file := func(head, imp string) *fstest.MapFile {
		return &fstest.MapFile{Data: []byte(head + "package p\n\nimport " + imp + "\n")}
	}
	plain := func(imp string) *fstest.MapFile { return file("", strconv.Quote(imp)) }
	fsys := fstest.MapFS{
		"cmd/api/main.go":                     plain(target),
		"internal/play/aliased.go":            file("", "dt "+strconv.Quote(target)),
		"internal/play/dot.go":                file("", ". "+strconv.Quote(target)),
		"internal/play/blank.go":              file("", "_ "+strconv.Quote(target)),
		"internal/play/tagged.go":             file("//go:build integration\n\n", strconv.Quote(target)),
		"internal/play/sub.go":                plain(target + "/sub"),
		"internal/play/contenttest/helper.go": plain(target),
		dicetestPackage + "/sub/own.go":       plain(target),
		"internal/play/play_test.go":          plain(target),
		"internal/leaktest/harness_test.go":   plain(target),
		"internal/play/other.go":              plain("example.test/backend/internal/platform/dice"),
		"internal/play/lookalike.go":          plain(target + "x"),
		dicetestPackage + "/dicetest.go":      plain(target),
		"internal/play/testdata/x.go":         plain(target),
	}
	got, err := importersOf(fsys, target)
	if err != nil {
		t.Fatalf("importersOf() error = %v", err)
	}
	want := []string{
		"cmd/api/main.go",
		"internal/play/aliased.go",
		"internal/play/blank.go",
		"internal/play/contenttest/helper.go",
		"internal/play/dot.go",
		"internal/play/sub.go",
		"internal/play/tagged.go",
		dicetestPackage + "/sub/own.go",
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("importersOf() = %v, want %v", got, want)
	}
}

// importersOf returns the non-test Go files under root that import target or a package under it.
// The files of target's own directory are left out.
// A path is relative to root.
func importersOf(root fs.FS, target string) ([]string, error) {
	fset := token.NewFileSet()
	var found []string
	err := fs.WalkDir(root, ".", func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); file != "." && (name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") || path.Dir(file) == dicetestPackage {
			return nil
		}
		src, err := fs.ReadFile(root, file)
		if err != nil {
			return err
		}
		f, err := parser.ParseFile(fset, file, src, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return fmt.Errorf("%s: %w", file, err)
			}
			if p == target || strings.HasPrefix(p, target+"/") {
				found = append(found, file)
				break
			}
		}
		return nil
	})
	return found, err
}

// rollerServices are the packages whose Config has a Roller field that, when nil, falls back to dice.Crypto, by directory.
var rollerServices = []string{"internal/play", "internal/characters"}

// TestEveryTestConfigOfAServiceSetsItsRoller reads every _test.go file and fails for a play.Config or characters.Config literal without a Roller.
// A harness that forgets it silently rolls random dice, and the tie it meets is a flaky test.
func TestEveryTestConfigOfAServiceSetsItsRoller(t *testing.T) {
	root := os.DirFS(backendRoot)
	module := modulePath(t, root)
	got, err := configsWithoutRoller(root, module)
	if err != nil {
		t.Fatalf("reading the backend's source: %v", err)
	}
	for _, f := range got {
		t.Errorf("%s: set Roller (dicetest.New(), or the test's own scripted roller) so the service never rolls with dice.Crypto", f)
	}
}

// TestTheRollerRuleSeesAConfigWithoutOne proves the rule can fail.
func TestTheRollerRuleSeesAConfigWithoutOne(t *testing.T) {
	const module = "example.test/backend"
	file := func(pkg, imports, body string) *fstest.MapFile {
		return &fstest.MapFile{Data: []byte("package " + pkg + "\n\n" + imports + "\n\nvar _ = " + body + "\n")}
	}
	playImport := `import "example.test/backend/internal/play"`
	aliased := `import pl "example.test/backend/internal/play"`
	charsImport := `import "example.test/backend/internal/characters"`
	fsys := fstest.MapFS{
		"internal/leaktest/a_test.go":          file("leaktest", playImport, "play.Config{Pool: 1}"),
		"internal/leaktest/b_test.go":          file("leaktest", aliased, "pl.Config{Pool: 1}"),
		"internal/maps/c_test.go":              file("maps", charsImport, "characters.Config{Pool: 1}"),
		"internal/play/d_test.go":              file("play", "", "Config{Pool: 1}"),
		"internal/characters/e_test.go":        file("characters", "", "&Config{}"),
		"internal/leaktest/nil_test.go":        file("leaktest", playImport, "play.Config{Pool: 1, Roller: nil}"),
		"internal/leaktest/dot_test.go":        file("leaktest", `import . "example.test/backend/internal/play"`, "Config{Pool: 1}"),
		"internal/leaktest/ok_test.go":         file("leaktest", playImport, "play.Config{Pool: 1, Roller: crypto{}}"),
		"internal/play/ok_test.go":             file("play", "", "Config{Roller: crypto{}}"),
		"internal/play/other_test.go":          file("play", "", "LiveConfig{Heartbeat: 1}"),
		"internal/maps/other_test.go":          file("maps", "", "Config{Pool: 1}"),
		"internal/leaktest/production.go":      file("leaktest", playImport, "play.Config{Pool: 1}"),
		"internal/leaktest/testdata/x_test.go": file("leaktest", playImport, "play.Config{Pool: 1}"),
	}
	got, err := configsWithoutRoller(fsys, module)
	if err != nil {
		t.Fatalf("configsWithoutRoller() error = %v", err)
	}
	want := []string{
		"internal/characters/e_test.go:5",
		"internal/leaktest/a_test.go:5",
		"internal/leaktest/dot_test.go:5",
		"internal/leaktest/nil_test.go:5",
		"internal/leaktest/b_test.go:5",
		"internal/maps/c_test.go:5",
		"internal/play/d_test.go:5",
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("configsWithoutRoller() = %v, want %v", got, want)
	}
}

// configsWithoutRoller returns "file:line" for each Config literal of a service in rollerServices, in a _test.go file, that has no Roller key.
// A literal is the package's own Config inside the package's directory, or Config of the package imported by its path elsewhere.
func configsWithoutRoller(root fs.FS, module string) ([]string, error) {
	fset := token.NewFileSet()
	var found []string
	err := fs.WalkDir(root, ".", func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); file != "." && (name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(file, "_test.go") {
			return nil
		}
		src, err := fs.ReadFile(root, file)
		if err != nil {
			return err
		}
		f, err := parser.ParseFile(fset, file, src, 0)
		if err != nil {
			return err
		}
		own := slices.Contains(rollerServices, path.Dir(file))
		names := map[string]bool{} // the names this file gives the services' packages
		dotted := false
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			for _, dir := range rollerServices {
				if p != module+"/"+dir {
					continue
				}
				name := path.Base(dir)
				if imp.Name != nil {
					name = imp.Name.Name
				}
				if name == "." {
					dotted = true // Config{} is the service's, as inside its package
					continue
				}
				names[name] = true
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok || !isServiceConfig(lit.Type, own || dotted, names) || setsRoller(lit) {
				return true
			}
			found = append(found, fmt.Sprintf("%s:%d", file, fset.Position(lit.Pos()).Line))
			return true
		})
		return nil
	})
	return found, err
}

// isServiceConfig says whether a literal's type is a service's Config: Config inside its package, or pkg.Config of an imported one.
func isServiceConfig(typ ast.Expr, own bool, names map[string]bool) bool {
	switch x := typ.(type) {
	case *ast.Ident:
		return own && x.Name == "Config"
	case *ast.SelectorExpr:
		pkg, ok := x.X.(*ast.Ident)
		return ok && x.Sel.Name == "Config" && names[pkg.Name]
	}
	return false
}

// setsRoller says whether a literal has a Roller key.
func setsRoller(lit *ast.CompositeLit) bool {
	return slices.ContainsFunc(lit.Elts, func(e ast.Expr) bool {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			return false
		}
		key, ok := kv.Key.(*ast.Ident)
		if val, isIdent := kv.Value.(*ast.Ident); isIdent && val.Name == "nil" {
			return false // Roller: nil is the fallback to dice.Crypto spelled out
		}
		return ok && key.Name == "Roller"
	})
}
