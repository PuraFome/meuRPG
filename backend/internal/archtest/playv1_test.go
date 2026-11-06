package archtest

import (
	_ "embed"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"testing"
)

// playv1Exceptions is the list of files outside the play module and the
// wiring that still import playv1; it can only shrink.
//
//go:embed playv1_exceptions.txt
var playv1Exceptions string

// playv1Path is the import path of the generated play API, under the module path.
const playv1Path = "gen/meurpg/play/v1"

// playv1Allowed are the places a non-test file may import playv1: the play
// module and the API wiring.
var playv1Allowed = []string{"internal/play/", "internal/platform/wiring/", "cmd/"}

// TestPlayV1StaysInPlayAndWiring fails for a non-test file outside the play
// module and the wiring (cmd/, internal/platform/wiring) that imports
// gen/meurpg/play/v1 and is not in playv1_exceptions.txt, and for a listed
// file that no longer does. Test files may import it.
func TestPlayV1StaysInPlayAndWiring(t *testing.T) {
	root := os.DirFS(backendRoot)
	target := modulePath(t, root) + "/" + playv1Path
	found := map[string]string{}
	files := 0
	for _, top := range []string{"internal", "cmd"} {
		err := walkGoFiles(root, top, func(path string) error {
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			files++
			src, err := fs.ReadFile(root, path)
			if err != nil {
				return err
			}
			bad, err := importsPlayV1Outside(path, src, target)
			if err != nil {
				return err
			}
			if bad {
				found[path] = fmt.Sprintf("%s imports playv1 (%s) outside the play module and the API wiring: pass the plain types "+
					"through the module's link package (internal/play/link) and let the API layer map to playv1", path, playv1Path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("reading %s: %v", top, err)
		}
	}
	if files == 0 {
		t.Fatal("found no Go file: is backendRoot right?")
	}
	failProblems(t, compareExceptions("playv1_exceptions.txt", parseExceptions(playv1Exceptions), found))
	t.Logf("%d non-test Go files checked for playv1 imports; %d known exceptions", files, len(found))
}

// importsPlayV1Outside reports whether a file that is not in an allowed place imports target.
func importsPlayV1Outside(path string, src []byte, target string) (bool, error) {
	for _, ok := range playv1Allowed {
		if strings.HasPrefix(path, ok) {
			return false, nil
		}
	}
	f, err := parser.ParseFile(token.NewFileSet(), path, src, parser.ImportsOnly)
	if err != nil {
		return false, err
	}
	for _, imp := range f.Imports {
		if p, err := strconv.Unquote(imp.Path.Value); err == nil && p == target {
			return true, nil
		}
	}
	return false, nil
}

func TestImportsPlayV1Outside(t *testing.T) {
	const target = "example.test/backend/" + playv1Path
	imp := func(alias string) string {
		return "package x\nimport " + alias + ` "` + target + "\"\n"
	}
	cases := []struct {
		name, path, src string
		want            bool
	}{
		{"characters", "internal/characters/a.go", imp("playv1"), true},
		{"renamed", "internal/maps/a.go", imp("pb"), true},
		{"play module", "internal/play/a.go", imp("playv1"), false},
		{"cmd", "cmd/api/main.go", imp("playv1"), false},
		{"wiring", "internal/platform/wiring/w.go", imp("playv1"), false},
		{"other import", "internal/maps/a.go", "package x\nimport _ \"example.test/backend/gen/meurpg/maps/v1\"\n", false},
		{"similar path", "internal/maps/a.go", "package x\nimport _ \"" + target + "connect\"\n", false},
	}
	for _, c := range cases {
		got, err := importsPlayV1Outside(c.path, []byte(c.src), target)
		if err != nil || got != c.want {
			t.Errorf("%s: got %v (err %v), want %v", c.name, got, err, c.want)
		}
	}
}
