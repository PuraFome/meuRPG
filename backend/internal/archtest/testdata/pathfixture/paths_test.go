// Package pathfixture is parsed, never built, by TestBeforeReadyUpwardPathForms: one test for
// each form in which a test can write a path that leaves its package directory.
package pathfixture

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	base    = "../.."
	reasons = "../../../proto/x.json"
)

var dataDir = filepath.Join(base, "data")

func TestLiteral(t *testing.T) { _, _ = os.ReadFile("../../NOTICE") }

func TestConstant(t *testing.T) { _, _ = os.ReadFile(reasons) }

func TestConcatenationWithABase(t *testing.T) { _, _ = os.ReadFile(base + "/../proto/y.json") }

func TestJoinWithABaseAndALaterDotDot(t *testing.T) {
	_, _ = os.ReadFile(filepath.Join(base, "web", "../docs/z.md"))
}

func TestJoinOfSeveralArguments(t *testing.T) {
	_, _ = os.ReadFile(filepath.Join("..", "..", "migrations", "sql", "a.sql"))
}

func TestJoinWithADynamicTail(t *testing.T) {
	for _, doc := range []string{"a.md"} {
		_, _ = os.Stat(filepath.Join("..", "..", "docs", filepath.FromSlash(doc)))
	}
}

func TestAPathResolvingToTheRoot(t *testing.T) { _, _ = os.ReadDir(filepath.Join(base, "..")) }

func TestThroughAPackageVariable(t *testing.T) { _, _ = os.ReadDir(dataDir) }

func TestThroughALocalVariable(t *testing.T) {
	root := filepath.Join(base, "..")
	_, _ = os.ReadFile(filepath.Join(root, "go.work"))
}

func TestAGlobInAConcatenation(t *testing.T) { _, _ = filepath.Glob(base + "/migrations/*.sql") }

func TestOwnDirectory(t *testing.T) { _, _ = os.ReadFile(filepath.Join("testdata", "a.txt")) }
