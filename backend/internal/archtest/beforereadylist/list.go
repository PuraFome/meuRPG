// Package beforereadylist reads the before-ready list (backend/internal/archtest/before_ready.txt,
// docs/testing.md#before-ready) and decides which paths a line's inputs cover. The tool that
// selects and runs the lines (cmd/beforeready) and the guard that keeps the list honest
// (internal/archtest) both use it, so the guard checks the format and the coverage the tool
// acts on. It lives under archtest because cmd/ is never imported by internal/.
package beforereadylist

import (
	"bufio"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
)

const (
	// Run marks a line of tests to run.
	Run = "run"
	// Helper marks a function whose callers compare with a file.
	Helper = "helper"
	// NoInputs marks a run line whose tests read nothing outside their package and its imports.
	NoInputs = "-"
)

// Line is one `run` or `helper` line of the list.
type Line struct {
	N      int            // line number in the file, for messages
	Raw    string         // the whole line, to tell a changed line from an unchanged one
	Kind   string         // Run or Helper
	Pkg    string         // directory under backend/
	Regex  string         // the -run regex (Run), or the function name (Helper)
	Re     *regexp.Regexp // Regex compiled
	Last   string         // the regenerate command (Run), or why the function pins (Helper)
	Inputs []string       // Run only: paths from the repository root read outside the package
}

// Format is the grammar of the list, for messages.
const Format = "`run<TAB>package<TAB>regex<TAB>regenerate command<TAB>inputs` or `helper<TAB>package<TAB>function<TAB>reason`"

// Parse reads the list: tab-separated lines, `#` comments and blank lines. Any other line that
// does not have the shape of Format is an error.
func Parse(r io.Reader) ([]Line, error) {
	var lines []Line
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for n := 1; sc.Scan(); n++ {
		text := sc.Text()
		if strings.TrimSpace(text) == "" || strings.HasPrefix(text, "#") {
			continue
		}
		l, err := parseLine(n, text)
		if err != nil {
			return nil, err
		}
		lines = append(lines, l)
	}
	return lines, sc.Err()
}

// parseLine reads one line that is neither blank nor a comment.
func parseLine(n int, text string) (Line, error) {
	cols := strings.Split(text, "\t")
	wantCols := 4
	if cols[0] == Run {
		wantCols = 5
	}
	if len(cols) != wantCols || (cols[0] != Run && cols[0] != Helper) || strings.TrimSpace(cols[3]) == "" {
		return Line{}, fmt.Errorf("line %d: want %s, got %q", n, Format, text)
	}
	re, err := regexp.Compile(cols[2])
	if err != nil {
		return Line{}, fmt.Errorf("line %d: regex %q: %w", n, cols[2], err)
	}
	l := Line{N: n, Raw: text, Kind: cols[0], Pkg: cols[1], Regex: cols[2], Re: re, Last: cols[3]}
	if cols[0] == Run && strings.TrimSpace(cols[4]) != NoInputs {
		for _, in := range strings.Split(cols[4], ",") {
			l.Inputs = append(l.Inputs, strings.TrimSpace(in))
		}
	}
	return l, nil
}

// RunLines keeps the lines of tests to run.
func RunLines(lines []Line) []Line {
	var out []Line
	for _, l := range lines {
		if l.Kind == Run {
			out = append(out, l)
		}
	}
	return out
}

// Covering returns the input that contains the path (the path itself, or a directory above it),
// or "". Inputs and path are slash paths from the repository root; "." is the root itself, so
// an input "." covers every path and the path "." is covered by that input alone.
func Covering(inputs []string, p string) string {
	if p == "" {
		return ""
	}
	p = path.Clean(p)
	for _, in := range inputs {
		if in == "" {
			continue
		}
		c := path.Clean(in)
		if c == "." || p == c || strings.HasPrefix(p, c+"/") {
			return in
		}
	}
	return ""
}

// Covered says whether the path is one of the inputs or inside one of them.
func Covered(path string, inputs []string) bool {
	return Covering(inputs, path) != ""
}
