// Package pinfixture is parsed, never built, by TestBeforeReadyDetector.
package pinfixture

import (
	"flag"
	"os"
	"testing"
)

const goldenPath = "testdata/x.golden"

var update = flag.Bool("update", false, "rewrite")

var frozenDigests = map[string]string{"a": "b"}

var counter = new(int)

func readIt(t *testing.T) { _, _ = os.ReadFile(goldenPath) }

func TestThroughAConst(t *testing.T) { _, _ = os.ReadFile(goldenPath) }

func TestThroughAFlag(t *testing.T) { _ = *update }

func TestThroughTheDigests(t *testing.T) { _ = frozenDigests["a"] }

func TestThroughAHelper(t *testing.T) { readIt(t) }

func TestPlain(t *testing.T) {}

func TestNotAFlag(t *testing.T) { *counter = 1 }

type loader struct{}

// load is a method: a helper line cannot name it, because the guard follows plain calls only.
func (loader) load() { _, _ = os.ReadFile(goldenPath) }
