// Command beforeready chooses which lines of the before-ready list a change can affect
// (docs/testing.md#before-ready). It compares the working tree with the merge base on
// origin/main, finds the packages each line's tests import, and writes the list of the
// lines to run. When it cannot tell, it selects every line.
//
//	beforeready -list FILE -out FILE [-all]
//
// stdout says what runs and why, and what is skipped. -out is the list file to run: the
// selected `run` lines (and the `helper` lines), in the format of the input.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	listFlag := flag.String("list", "", "the before-ready list")
	out := flag.String("out", "", "where to write the list of the lines to run")
	all := flag.Bool("all", false, "run every line")
	flag.Parse()
	if *listFlag == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "beforeready: -list and -out are required")
		os.Exit(2)
	}
	raw, err := os.ReadFile(*listFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "beforeready:", err)
		os.Exit(2)
	}
	lines, err := ParseList(bytes.NewReader(raw))
	if err != nil {
		fmt.Fprintln(os.Stderr, "beforeready:", err)
		os.Exit(2)
	}

	facts, ferr := computeFacts(*listFlag)
	run, skipped, note := plan(lines, *all, facts, ferr)
	if note != "" {
		fmt.Println(note)
	}
	if err := os.WriteFile(*out, []byte(filtered(string(raw), run)), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "beforeready:", err)
		os.Exit(2)
	}
	report(run, skipped)
}

// plan chooses the lines to run. It runs every line when all is set and when the facts could
// not be computed (ferr): never nothing for lack of knowledge. note says so.
func plan(lines []Line, all bool, f Facts, ferr error) (run []Decision, skipped []Line, note string) {
	switch {
	case all:
		return All(lines, "BEFORE_READY_ALL is set"), nil, ""
	case ferr != nil:
		note = fmt.Sprintf("before-ready: cannot tell which lines the change affects: %v\nbefore-ready: running every line", ferr)
		return All(lines, "the change could not be compared with the merge base"), nil, note
	}
	run, skipped = Select(lines, f)
	return run, skipped, ""
}

// filtered keeps the comments, the helper lines and the selected run lines of the list.
func filtered(raw string, run []Decision) string {
	keep := map[string]bool{}
	for _, d := range run {
		keep[d.Line.Raw] = true
	}
	var b strings.Builder
	for _, text := range strings.Split(raw, "\n") {
		if strings.HasPrefix(text, "run\t") && !keep[text] {
			continue
		}
		b.WriteString(text + "\n")
	}
	return b.String()
}

func report(run []Decision, skipped []Line) {
	if len(run) == 0 {
		fmt.Println("before-ready: the change touches no Go package, proto file, migration or input of the list: nothing to run")
	}
	byPkg := map[string][]string{}
	for _, d := range run {
		byPkg[d.Line.Pkg] = append(byPkg[d.Line.Pkg], d.Reason)
	}
	for _, pkg := range Packages(run) {
		reasons := dedupe(byPkg[pkg])
		fmt.Printf("  run   %-28s %d line(s): %s\n", pkg, len(byPkg[pkg]), strings.Join(firstN(reasons, 2), "; "))
	}
	skippedPkgs := map[string]bool{}
	ran := map[string]bool{}
	for _, d := range run {
		ran[d.Line.Pkg] = true
	}
	for _, l := range skipped {
		if !ran[l.Pkg] {
			skippedPkgs[l.Pkg] = true
		}
	}
	var names []string
	for p := range skippedPkgs {
		names = append(names, p)
	}
	sort.Strings(names)
	if len(names) > 0 {
		fmt.Printf("  skip  %s: the change reaches none of their lines (BEFORE_READY_ALL=1 runs them)\n", strings.Join(names, ", "))
	}
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func firstN(in []string, n int) []string {
	if len(in) <= n {
		return in
	}
	return append(append([]string{}, in[:n]...), fmt.Sprintf("and %d more", len(in)-n))
}

// computeFacts reads the change from git and the imports from go list.
func computeFacts(listFile string) (Facts, error) {
	var f Facts
	root, err := sh("", "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return f, fmt.Errorf("not a git work tree: %w", err)
	}
	root = strings.TrimSpace(root)
	base, err := sh(root, "git", "merge-base", "HEAD", "origin/main")
	if err != nil {
		return f, fmt.Errorf("no merge base with origin/main: %w", err)
	}
	base = strings.TrimSpace(base)
	diff, err := sh(root, "git", "diff", "--name-only", "--no-renames", base)
	if err != nil {
		return f, err
	}
	untracked, err := sh(root, "git", "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return f, err
	}
	f.Changed = append(fields(diff), fields(untracked)...)
	f.NewLines = newLines(root, base, listFile)
	f.PkgDirs, f.Deps, err = packageGraph(filepath.Join(root, backendDir))
	return f, err
}

// newLines returns the run lines of the list that the list of the merge base lacks, or nil.
func newLines(root, base, listFile string) map[string]bool {
	old, err := sh(root, "git", "show", base+":"+listPath)
	if err != nil {
		return nil
	}
	have := map[string]bool{}
	for _, text := range strings.Split(old, "\n") {
		have[text] = true
	}
	cur, err := os.ReadFile(listFile)
	if err != nil {
		return nil
	}
	out := map[string]bool{}
	for _, text := range strings.Split(string(cur), "\n") {
		if strings.HasPrefix(text, "run\t") && !have[text] {
			out[text] = true
		}
	}
	return out
}

// packageGraph lists the module's packages and, for each, the module packages its tests need.
func packageGraph(backend string) (dirs []string, deps map[string][]string, err error) {
	mod, err := sh(backend, "go", "list", "-m")
	if err != nil {
		return nil, nil, fmt.Errorf("go list -m: %w", err)
	}
	mod = strings.TrimSpace(mod)
	text, err := sh(backend, "go", "list", "-test", "-f", `{{.ImportPath}}{{"\t"}}{{join .Deps " "}}`, "./...")
	if err != nil {
		return nil, nil, fmt.Errorf("go list -test: %w", err)
	}
	return parseGraph(mod, text)
}

func fields(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// sh runs a command in dir and returns its stdout.
func sh(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}
