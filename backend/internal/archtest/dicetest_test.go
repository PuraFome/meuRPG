package archtest

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
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
// the test dice and is not part of that package. Test files may import it.
func TestNoProductionCodeImportsTheTestDice(t *testing.T) {
	root := os.DirFS(backendRoot)
	module := modulePath(t, root)
	got, err := importersOf(root, module+"/"+dicetestPackage)
	if err != nil {
		t.Fatalf("reading the backend's source: %v", err)
	}
	for _, f := range got {
		t.Errorf("%s imports %s: the test dice are for _test.go files only; production code rolls with dice.Crypto", f, dicetestPackage)
	}
}

// TestTheTestDiceRuleSeesAnImportInProductionCode proves the rule can fail: a
// production file importing the package is found, a test file and the package
// itself are not.
func TestTheTestDiceRuleSeesAnImportInProductionCode(t *testing.T) {
	const target = "example.test/backend/" + dicetestPackage
	src := func(imp string) *fstest.MapFile {
		return &fstest.MapFile{Data: []byte("package p\n\nimport _ \"" + imp + "\"\n")}
	}
	fsys := fstest.MapFS{
		"cmd/api/main.go":                   src(target),
		"internal/play/play.go":             src(target),
		"internal/play/play_test.go":        src(target),
		"internal/leaktest/harness_test.go": src(target),
		"internal/play/other.go":            src("example.test/backend/internal/platform/dice"),
		dicetestPackage + "/dicetest.go":    src(target),
		"internal/play/testdata/x.go":       src(target),
	}
	got, err := importersOf(fsys, target)
	if err != nil {
		t.Fatalf("importersOf() error = %v", err)
	}
	if want := "cmd/api/main.go,internal/play/play.go"; strings.Join(got, ",") != want {
		t.Errorf("importersOf() = %v, want %s", got, want)
	}
}

// importersOf returns the non-test Go files under root, outside the package
// itself, that import target. A path is relative to root.
func importersOf(root fs.FS, target string) ([]string, error) {
	fset := token.NewFileSet()
	var found []string
	err := fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != "." && (name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.HasPrefix(path, dicetestPackage+"/") {
			return nil
		}
		src, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		f, err := parser.ParseFile(fset, path, src, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			if p == target {
				found = append(found, path)
			}
		}
		return nil
	})
	return found, err
}
