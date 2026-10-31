// Command beforeready chooses which lines of the before-ready list a change can affect
// (docs/testing.md#before-ready) and answers what .github/scripts/before-ready.sh needs to run
// them, so the list is parsed in Go alone (internal/archtest/beforereadylist). It compares the
// working tree with the merge base on origin/main, finds the packages each line's tests
// import, and writes the list of the lines to run. When it cannot tell, it selects every line.
//
//	beforeready -list FILE -out FILE [-all] [-custom] [-only REGEX] [-base REV]
//	beforeready -list FILE -packages
//	beforeready -list FILE -regex-of PACKAGE
//	beforeready -list FILE -regen-of PACKAGE -test NAME
//
// The first form prints on stdout what runs and why, and what is skipped. -out is the list file
// to run: the selected `run` lines (and the `helper` lines), in the format of the input; -only
// is an RE2 regex on the package directory that narrows them (an invalid one exits 2); -custom
// says the list is not the project's own, so every line of it runs. The other forms read that
// -out file: the packages to test, the -run regex of one package, and the command that
// regenerates the file of a failed test.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/archtest/beforereadylist"
)

// exitUsage is the exit code of a refusal: bad flags, a list or pattern that cannot be read.
const exitUsage = 2

func main() {
	os.Exit(realMain(os.Args[1:], os.Stdout, os.Stderr))
}

// realMain is main with its arguments and streams, and returns the exit code.
func realMain(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("beforeready", flag.ContinueOnError)
	fs.SetOutput(stderr)
	listFlag := fs.String("list", "", "the before-ready list")
	out := fs.String("out", "", "where to write the list of the lines to run")
	all := fs.Bool("all", false, "run every line")
	custom := fs.Bool("custom", false, "the list is not the project's own (BEFORE_READY_LIST): run every line of it")
	base := fs.String("base", "", "compare with this revision instead of the merge base with origin/main")
	only := fs.String("only", "", "an RE2 regex on the package directory: run only the packages it matches")
	packagesQ := fs.Bool("packages", false, "print the packages of the run lines of -list, one per line")
	regexQ := fs.String("regex-of", "", "print the -run regex of this package: the regexes of its lines joined by `|`")
	regenQ := fs.String("regen-of", "", "print the command that regenerates the file of the failed -test of this package")
	test := fs.String("test", "", "with -regen-of: the name of the failed test")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	fail := func(err error) int {
		fmt.Fprintln(stderr, "beforeready:", err)
		return exitUsage
	}
	if *listFlag == "" {
		return fail(errors.New("-list is required"))
	}
	raw, err := os.ReadFile(*listFlag)
	if err != nil {
		return fail(err)
	}
	parsed, err := beforereadylist.Parse(bytes.NewReader(raw))
	if err != nil {
		return fail(err)
	}
	lines := beforereadylist.RunLines(parsed)

	switch {
	case *packagesQ:
		for _, p := range Packages(All(lines, "")) {
			fmt.Fprintln(stdout, p)
		}
		return 0
	case *regexQ != "":
		re := regexOf(lines, *regexQ)
		if re == "" {
			return fail(fmt.Errorf("no line for the package %q", *regexQ))
		}
		fmt.Fprintln(stdout, re)
		return 0
	case *regenQ != "":
		fmt.Fprintln(stdout, regenFor(lines, *regenQ, *test))
		return 0
	case *out == "":
		return fail(errors.New("-out is required"))
	}
	o := options{list: *listFlag, out: *out, only: *only, base: *base, all: *all, custom: *custom}
	if err := selectLines(o, lines, raw, stdout); err != nil {
		return fail(err)
	}
	return 0
}

// options are the flags of the selection.
type options struct {
	list, out, only, base string
	all, custom           bool
}

// selectLines chooses the lines to run, writes them to o.out and says what runs and why.
func selectLines(o options, lines []Line, raw []byte, stdout io.Writer) error {
	onlyRe, err := compileOnly(o.only, lines)
	if err != nil {
		return err
	}
	customList := ""
	if o.custom {
		customList = o.list
	}
	var facts Facts
	var ferr error
	if !o.all && !o.custom {
		facts, ferr = computeFacts(o.list, o.base)
	}
	run, skipped, note := plan(lines, o.all, customList, facts, ferr)
	if note != "" {
		fmt.Fprintln(stdout, note)
	}
	selected := len(run)
	narrowed := outside(run, onlyRe)
	run, skipped = within(run, onlyRe), withinLines(skipped, onlyRe)
	if err := os.WriteFile(o.out, []byte(filtered(string(raw), run)), 0o600); err != nil {
		return err
	}
	report(stdout, run, skipped, narrowed, selected, o.only)
	return nil
}

// compileOnly compiles the BEFORE_READY_ONLY regex (RE2, like every regex of the Go tool: a
// pattern it cannot read is an error, never a filter dropped in silence) and checks that it
// matches a package of the list. An empty pattern is no filter: nil.
func compileOnly(only string, lines []Line) (*regexp.Regexp, error) {
	if only == "" {
		return nil, nil
	}
	re, err := regexp.Compile(only)
	if err != nil {
		return nil, fmt.Errorf("BEFORE_READY_ONLY %q is not a valid regex (RE2 syntax): %w", only, err)
	}
	if len(within(All(lines, ""), re)) == 0 {
		return nil, fmt.Errorf("BEFORE_READY_ONLY %q matches no package of the list", only)
	}
	return re, nil
}

// regexOf returns the -run regex of one package: the regexes of its lines, each in a group,
// joined by `|`. "" when the package has no line.
func regexOf(lines []Line, pkg string) string {
	var parts []string
	for _, l := range lines {
		if l.Pkg == pkg {
			parts = append(parts, "("+l.Regex+")")
		}
	}
	return strings.Join(parts, "|")
}

// regenFor returns the command that rewrites the file of one failed test: the one of the first
// line of its package whose regex matches the name, with {test} replaced by the name. "" when
// no line matches.
func regenFor(lines []Line, pkg, test string) string {
	for _, l := range lines {
		if l.Pkg == pkg && l.Re.MatchString(test) {
			return strings.ReplaceAll(l.Last, "{test}", test)
		}
	}
	return ""
}

// plan chooses the lines to run. It runs every line when all is set, when the list is a custom
// one (custom is its file: the selection by change reads the project's own list, so a custom
// list runs whole) and when the facts could not be computed (ferr): never nothing for lack of
// knowledge. note says why when it is not the plain BEFORE_READY_ALL.
func plan(lines []Line, all bool, custom string, f Facts, ferr error) (run []Decision, skipped []Line, note string) {
	switch {
	case all:
		return All(lines, "BEFORE_READY_ALL is set"), nil, ""
	case custom != "":
		note = fmt.Sprintf("before-ready: BEFORE_READY_LIST is set: running every line of %s (the selection by change reads only the project's own list)", custom)
		return All(lines, "BEFORE_READY_LIST is a custom list"), nil, note
	case ferr != nil:
		note = fmt.Sprintf("before-ready: cannot tell which lines the change affects: %v\nbefore-ready: running every line", ferr)
		return All(lines, "the selection could not be computed"), nil, note
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
	for _, text := range strings.Split(strings.TrimSuffix(raw, "\n"), "\n") {
		if strings.HasPrefix(text, "run\t") && !keep[text] {
			continue
		}
		b.WriteString(text + "\n")
	}
	return b.String()
}

// report says what runs and why, and what is skipped. selected is how many lines the selection
// chose before -only narrowed them, and narrowed the lines -only left out (only is that pattern).
func report(w io.Writer, run []Decision, skipped []Line, narrowed []Line, selected int, only string) {
	switch {
	case selected == 0:
		fmt.Fprintln(w, "before-ready: the change touches no Go package, proto file, migration or input of the list: nothing to run")
	case len(run) == 0:
		fmt.Fprintf(w, "before-ready: nothing ran: the change selects none of the lines that BEFORE_READY_ONLY '%s' keeps (BEFORE_READY_ALL=1 runs them)\n", only)
	}
	byPkg := map[string][]string{}
	for _, d := range run {
		byPkg[d.Line.Pkg] = append(byPkg[d.Line.Pkg], d.Reason)
	}
	for _, pkg := range Packages(run) {
		reasons := dedupe(byPkg[pkg])
		fmt.Fprintf(w, "  run   %-28s %d line(s): %s\n", pkg, len(byPkg[pkg]), strings.Join(firstN(reasons, 2), "; "))
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
		fmt.Fprintf(w, "  skip  %s: the change reaches none of their lines (BEFORE_READY_ALL=1 runs them)\n", strings.Join(names, ", "))
	}
	if left := Packages(All(narrowed, "")); len(left) > 0 {
		fmt.Fprintf(w, "  skip  %s: selected by the change, but not matched by BEFORE_READY_ONLY '%s'\n", strings.Join(left, ", "), only)
	}
}

// outside returns the lines of the decisions whose package the pattern does not match.
func outside(ds []Decision, re *regexp.Regexp) []Line {
	if re == nil {
		return nil
	}
	var out []Line
	for _, d := range ds {
		if !re.MatchString(d.Line.Pkg) {
			out = append(out, d.Line)
		}
	}
	return out
}

// within keeps the decisions whose package the pattern matches; a nil pattern keeps all.
func within(ds []Decision, re *regexp.Regexp) []Decision {
	if re == nil {
		return ds
	}
	var out []Decision
	for _, d := range ds {
		if re.MatchString(d.Line.Pkg) {
			out = append(out, d)
		}
	}
	return out
}

func withinLines(ls []Line, re *regexp.Regexp) []Line {
	if re == nil {
		return ls
	}
	var out []Line
	for _, l := range ls {
		if re.MatchString(l.Pkg) {
			out = append(out, l)
		}
	}
	return out
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
func computeFacts(listFile, rev string) (Facts, error) {
	var f Facts
	root, err := sh("", "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return f, fmt.Errorf("not a git work tree: %w", err)
	}
	root = strings.TrimSpace(root)
	base := rev
	if base == "" {
		mb, err := sh(root, "git", "merge-base", "HEAD", "origin/main")
		if err != nil {
			return f, fmt.Errorf("no merge base with origin/main: %w", err)
		}
		base = strings.TrimSpace(mb)
	}
	f.Changed, err = changedFiles(root, base)
	if err != nil {
		return f, err
	}
	f.NewLines = newLines(root, base, listFile)
	f.PkgDirs, f.Deps, err = packageGraph(filepath.Join(root, backendDir))
	return f, err
}

// changedFiles lists the paths that differ from base in the working tree, deleted and untracked
// ones included, from the repository root.
func changedFiles(root, base string) ([]string, error) {
	diff, err := sh(root, "git", "diff", "--name-only", "-z", "--no-renames", base)
	if err != nil {
		return nil, err
	}
	untracked, err := sh(root, "git", "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	return append(nulSplit(diff), nulSplit(untracked)...), nil
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

// nulSplit splits the NUL-terminated names of git -z, which are never quoted or escaped.
func nulSplit(s string) []string {
	var out []string
	for _, n := range strings.Split(s, "\x00") {
		if n != "" {
			out = append(out, n)
		}
	}
	return out
}

// sh runs a command in dir and returns its stdout.
func sh(dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}
