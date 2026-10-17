// Package goldentest compares what a test produced with a file under
// testdata and rewrites that file when the test binary runs with -update.
// Only tests import it. The same flag serves every package that uses it:
//
//	go test ./internal/progression -run History -update
//
// Review the diff of the rewritten files before committing them.
package goldentest

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite the golden files from the current output")

// Compare fails the test unless got is the content of the file at path.
// With -update it writes the file instead.
func Compare(t testing.TB, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the golden file (write it with -update): %v", err)
	}
	if !bytes.Equal(got, want) {
		line := firstDiff(got, want)
		t.Errorf("output differs from %s at line %d (run with -update and read the diff):\n got  %s\n want %s", path, line, lineOf(got, line), lineOf(want, line))
	}
}

// firstDiff is the 1-based line where a and b first differ.
func firstDiff(a, b []byte) int {
	x, y := bytes.Split(a, []byte("\n")), bytes.Split(b, []byte("\n"))
	for i := 0; i < len(x) && i < len(y); i++ {
		if !bytes.Equal(x[i], y[i]) {
			return i + 1
		}
	}
	return min(len(x), len(y)) + 1
}

// lineOf is the 1-based line n of b, or "" past the end.
func lineOf(b []byte, n int) string {
	lines := strings.Split(string(b), "\n")
	if n > len(lines) {
		return ""
	}
	return lines[n-1]
}

// JSON writes v as indented JSON, without escaping < and >, and compares it
// with the file at path.
func JSON(t testing.TB, path string, v any) {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	Compare(t, path, buf.Bytes())
}

var uuidRE = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// Names turns the random IDs of a test into stable names.
type Names struct {
	labels  map[string]string
	unnamed map[string]string
}

// NewNames is an empty set of names.
func NewNames() *Names {
	return &Names{labels: map[string]string{}, unnamed: map[string]string{}}
}

// Label names an ID: it reads <name> in what Redact returns.
func (n *Names) Label(id, name string) { n.labels[id] = "<" + name + ">" }

// Redact puts the labels in place of the UUIDs of s, and numbers the others
// (<uuid1>, <uuid2>) in the order it first meets them.
func (n *Names) Redact(s string) string {
	return uuidRE.ReplaceAllStringFunc(s, func(id string) string {
		if l, ok := n.labels[id]; ok {
			return l
		}
		if l, ok := n.unnamed[id]; ok {
			return l
		}
		l := fmt.Sprintf("<uuid%d>", len(n.unnamed)+1)
		n.unnamed[id] = l
		return l
	})
}

var quotedTime = regexp.MustCompile(`"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z"`)

// MaskTimes puts the rank of each quoted RFC 3339 time of s, oldest first, in
// its place: the clock is the test's, the order is the behavior.
func MaskTimes(t testing.TB, s string) string {
	t.Helper()
	seen := map[string]time.Time{}
	for _, raw := range quotedTime.FindAllString(s, -1) {
		at, err := time.Parse(time.RFC3339Nano, strings.Trim(raw, `"`))
		if err != nil {
			t.Fatalf("parse the time %s: %v", raw, err)
		}
		seen[raw] = at
	}
	ordered := make([]string, 0, len(seen))
	for raw := range seen {
		ordered = append(ordered, raw)
	}
	sort.Slice(ordered, func(i, j int) bool { return seen[ordered[i]].Before(seen[ordered[j]]) })
	rank := make(map[string]string, len(ordered))
	for i, raw := range ordered {
		rank[raw] = fmt.Sprintf(`"<t%02d>"`, i+1)
	}
	return quotedTime.ReplaceAllStringFunc(s, func(raw string) string { return rank[raw] })
}
