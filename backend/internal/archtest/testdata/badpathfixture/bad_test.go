// Package badpathfixture is parsed, never built, by TestBeforeReadyUnresolvedPathsFail: each
// test writes a path that goes up from a base the guard cannot resolve.
package badpathfixture

import (
	"os"
	"path/filepath"
	"testing"
)

func repoRoot(t *testing.T) string { return "" }

func TestUnknownBaseInAJoin(t *testing.T) {
	_, _ = os.ReadFile(filepath.Join(repoRoot(t), "../web/x.ts"))
}

func TestUnknownBaseInAConcatenation(t *testing.T) { _, _ = os.ReadFile(repoRoot(t) + "/../web/y.ts") }

func TestBaseIsAParameter(t *testing.T) { read("/data") }

func read(dir string) { _, _ = os.ReadFile(filepath.Join(dir, "../z.json")) }

func TestResolvable(t *testing.T) { _, _ = os.ReadFile("../ok.json") }
