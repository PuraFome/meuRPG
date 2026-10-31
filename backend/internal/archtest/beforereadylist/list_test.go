package beforereadylist

import (
	"os"
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
		{"proto", ""},
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
	if Covered("x", nil) {
		t.Error("no inputs cover nothing")
	}
}
