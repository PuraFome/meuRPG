package archtest

import (
	"fmt"
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
