package upfixture

import (
	"os"
	"path/filepath"
	"testing"
)

const viaConst = "../../proto/x.json"

func readIt() []byte {
	b, _ := os.ReadFile("../../NOTICE")
	return b
}

func TestDirect(t *testing.T) { _, _ = os.ReadFile("../data/a.txt") }

func TestThroughAConst(t *testing.T) { _, _ = os.ReadFile(viaConst) }

func TestThroughAHelper(t *testing.T) { _ = readIt() }

func TestThroughAGlob(t *testing.T) { _, _ = filepath.Glob("../../migrations/*.sql") }

func TestThroughAJoin(t *testing.T) { _ = filepath.Join("..", "..", "docs", "x.md") }

func TestThroughANonTestFile(t *testing.T) { _ = fromNonTestFile() }

func TestOwnDirectory(t *testing.T) { _, _ = os.ReadFile("testdata/a.txt") }

type reader struct{}

func (reader) load() []byte {
	b, _ := os.ReadFile("../../web/m.json")
	return b
}

func TestThroughAMethod(t *testing.T) { _ = reader{}.load() }
