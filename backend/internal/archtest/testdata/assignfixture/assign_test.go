// Package assignfixture is parsed, never built, by TestBeforeReadyUpwardPathsAssignedMoreThanOnce:
// a path written in an assignment is read even when the name it is assigned to is not assigned
// exactly once.
package assignfixture

import (
	"os"
	"path/filepath"
	"testing"
)

var root string

func init() { root = "../.." }

var dir = "a"

type params struct{ Root string }

func readDir() { _, _ = os.ReadDir(dir) }

// A: two subtests each define dir.
func TestTwoSubtestsDefineTheSameName(t *testing.T) {
	t.Run("a", func(t *testing.T) {
		dir := "../../proto"
		_, _ = os.ReadDir(dir)
	})
	t.Run("b", func(t *testing.T) {
		dir := "../../docs"
		_, _ = os.ReadDir(dir)
	})
}

// B: the left side is a field, or an element of a map.
func TestAssignedToAFieldOrAnElement(t *testing.T) {
	var p params
	p.Root = "../../web"
	m := map[string]string{}
	m["k"] = "../x"
	_, _ = os.ReadFile(filepath.Join(p.Root, m["k"]))
}

// C: a package variable is set in init.
func TestReadsWhatInitSet(t *testing.T) { _, _ = os.ReadFile(filepath.Join(root, "go.work")) }

// D: a package variable is reassigned in the test, and a helper reads it.
func TestReassignsAPackageVariable(t *testing.T) {
	dir = "../x"
	readDir()
}

func TestOnlyOwnDirectory(t *testing.T) { _ = filepath.Join("testdata", "a") }
