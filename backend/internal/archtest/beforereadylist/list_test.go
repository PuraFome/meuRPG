package beforereadylist

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestParseRoundTrip(t *testing.T) {
	t.Parallel()
	const text = "# comment\n\nrun\tinternal/x\t^(TestA|TestB)$\tcd backend && go test ./internal/x -update\tNOTICE, backend/migrations\nhelper\tinternal/x\trunGolden\twhy it pins\nrun\tcmd/y\t^TestY$\tno generator\t-\n"
	lines, err := Parse(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("parsed %d lines, want 3", len(lines))
	}
	l := lines[0]
	if l.N != 3 || l.Kind != Run || l.Pkg != "internal/x" || l.Regex != "^(TestA|TestB)$" || l.Last != "cd backend && go test ./internal/x -update" ||
		!slices.Equal(l.Inputs, []string{"NOTICE", "backend/migrations"}) || !l.Re.MatchString("TestB") || l.Re.MatchString("TestC") {
		t.Errorf("run line = %+v", l)
	}
	if want := strings.Split(text, "\n")[2]; l.Raw != want {
		t.Errorf("Raw = %q, want %q", l.Raw, want)
	}
	if h := lines[1]; h.Kind != Helper || h.Regex != "runGolden" || h.Last != "why it pins" || h.Inputs != nil {
		t.Errorf("helper line = %+v", h)
	}
	if y := lines[2]; y.Inputs != nil || y.N != 5 {
		t.Errorf("a line with `-` inputs = %+v", y)
	}
	if got := RunLines(lines); len(got) != 2 || got[1].Pkg != "cmd/y" {
		t.Errorf("RunLines = %+v", got)
	}
}

func TestParseRefuses(t *testing.T) {
	t.Parallel()
	for name, text := range map[string]string{
		"a run line without inputs":   "run\tinternal/x\t^T$\tregen\n",
		"a helper line with inputs":   "helper\tinternal/x\tf\twhy\t-\n",
		"an unknown kind":             "skip\tinternal/x\t^T$\tregen\t-\n",
		"spaces for tabs":             "run internal/x ^T$ regen -\n",
		"an empty regenerate command": "run\tinternal/x\t^T$\t \t-\n",
		"a regex that does not parse": "run\tinternal/x\t^(T$\tregen\t-\n",
	} {
		if _, err := Parse(strings.NewReader(text)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// The real list parses with the same function the tool and the guard use.
func TestParseTheRealList(t *testing.T) {
	t.Parallel()
	f, err := os.Open("../before_ready.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	lines, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(RunLines(lines)) == 0 {
		t.Error("the list has no run line")
	}
}

func TestCovering(t *testing.T) {
	t.Parallel()
	inputs := []string{"NOTICE", "backend/migrations", "proto/"}
	cases := []struct {
		path string
		want string
	}{
		{"NOTICE", "NOTICE"},
		{"NOTICE.md", ""},
		{"backend/migrations", "backend/migrations"},
		{"backend/migrations/", "backend/migrations"},
		{"backend/migrations/0001.sql", "backend/migrations"},
		{"backend/migrations2/x", ""},
		{"backend", ""},
		{"proto/play/v1/play.proto", "proto/"},
		{"proto", "proto/"},
		{"web/a.ts", ""},
	}
	for _, c := range cases {
		if got := Covering(inputs, c.path); got != c.want {
			t.Errorf("Covering(%q) = %q, want %q", c.path, got, c.want)
		}
		if got := Covered(c.path, inputs); got != (c.want != "") {
			t.Errorf("Covered(%q) = %v", c.path, got)
		}
	}
	// The repository root: an input "." covers every path, and the root itself is covered by
	// "." alone.
	root := []string{"."}
	for _, p := range []string{".", "NOTICE", "backend/migrations/0001.sql"} {
		if got := Covering(root, p); got != "." {
			t.Errorf("Covering(%q, %q) = %q, want the root input", root, p, got)
		}
	}
	for _, in := range []string{"backend", "NOTICE", "proto/"} {
		if Covered(".", []string{in}) {
			t.Errorf("the repository root is covered by %q", in)
		}
	}
	if Covered("NOTICE", []string{""}) || Covered("", []string{"."}) {
		t.Error("an empty input or an empty path covers nothing")
	}
	if Covered("x", nil) {
		t.Error("no inputs cover nothing")
	}
}

// repoRoot is the repository root seen from this package's directory.
const repoRoot = "../../../.."

// parseRealList parses backend/internal/archtest/before_ready.txt.
func parseRealList(t *testing.T) []Line {
	t.Helper()
	f, err := os.Open("../before_ready.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	lines, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	return lines
}

// Every input is a path from the repository root that exists. These tests read the whole
// repository, so they live here, in a package no line of the list runs, not in the archtest
// package whose line would then have to name the root.
func TestRealListInputsExist(t *testing.T) {
	t.Parallel()
	for _, l := range RunLines(parseRealList(t)) {
		for _, in := range l.Inputs {
			if in == "" || strings.HasPrefix(in, "/") || strings.Contains(in, "..") {
				t.Errorf("before_ready.txt:%d: input %q must be a path from the repository root", l.N, in)
				continue
			}
			if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(in))); err != nil {
				t.Errorf("before_ready.txt:%d: input %q does not exist: %v", l.N, in, err)
			}
		}
	}
}

// hintPath matches a repository path written in a regenerate command or a reason: under a
// top-level directory of the backend module or of the repository.
var hintPath = regexp.MustCompile(`(?:\./)?(?:internal|cmd|backend|proto|docs|migrations)/[A-Za-z0-9_./-]+`)

// A failure prints the fourth column next to the failed test, and a path in it that moved (a
// file named under a directory it is not in) sends the reader to nothing. A path is read from
// backend/ or from the repository root.
func TestRealListHintsNamePathsThatExist(t *testing.T) {
	t.Parallel()
	for _, l := range parseRealList(t) {
		for _, p := range hintPath.FindAllString(l.Last, -1) {
			p = strings.TrimPrefix(strings.TrimRight(p, ".,)"), "./")
			if _, err := os.Stat(filepath.Join(repoRoot, "backend", filepath.FromSlash(p))); err == nil {
				continue
			}
			if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(p))); err == nil {
				continue
			}
			t.Errorf("before_ready.txt:%d: the hint names %q, which exists neither under backend/ nor at the repository root: fix the path", l.N, p)
		}
	}
}
