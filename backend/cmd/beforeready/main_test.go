package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := sh(dir, "git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(out)
}

func write(t *testing.T, root, name, text string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// repoWithBase makes a repository whose first commit holds the files below and returns it with
// that commit as the base.
func repoWithBase(t *testing.T) (root, base string) {
	t.Helper()
	root = t.TempDir()
	git(t, root, "init", "-q")
	for _, n := range []string{"keep.txt", "gone.txt", "edit.txt", "internal/authz/old.txt"} {
		write(t, root, n, "base\n")
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-q", "-m", "base")
	return root, git(t, root, "rev-parse", "HEAD")
}

func TestBeforeReadyChangedFiles(t *testing.T) {
	root, base := repoWithBase(t)
	write(t, root, "edit.txt", "changed\n")
	if err := os.Remove(filepath.Join(root, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"new.txt", "internal/authz/tést.txt", "dir/with space.txt", "quo\"te.txt", "new\nline.txt"} {
		write(t, root, n, "x\n")
	}
	got, err := changedFiles(root, base)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"dir/with space.txt", "edit.txt", "gone.txt", "internal/authz/tést.txt", "new\nline.txt", "new.txt", "quo\"te.txt"}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("changed files = %q, want %q", got, want)
	}
}

func TestBeforeReadyChangedFilesCommittedAndClean(t *testing.T) {
	root, base := repoWithBase(t)
	got, err := changedFiles(root, base)
	if err != nil || len(got) != 0 {
		t.Errorf("clean tree: %q, %v", got, err)
	}
	write(t, root, "internal/authz/é.txt", "x\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-q", "-m", "more")
	if got, err = changedFiles(root, base); err != nil || !slices.Equal(got, []string{"internal/authz/é.txt"}) {
		t.Errorf("committed change: %q, %v", got, err)
	}
}

// With no origin/main (and no -base) the facts cannot be computed: the caller runs every line.
func TestBeforeReadyFactsWithoutMergeBase(t *testing.T) {
	root, _ := repoWithBase(t)
	t.Chdir(root)
	if _, err := computeFacts(filepath.Join(root, "list.txt"), ""); err == nil || !strings.Contains(err.Error(), "merge base") {
		t.Errorf("err = %v, want a merge base error", err)
	}
	lines, _ := ParseList(strings.NewReader("run\tinternal/x\t^TestX$\tregen\t-\n"))
	run, skipped, note := plan(lines, false, "", Facts{}, os.ErrNotExist)
	if len(run) != 1 || len(skipped) != 0 || !strings.Contains(note, "running every line") || run[0].Reason != "the selection could not be computed" {
		t.Errorf("plan = %v %v %q", run, skipped, note)
	}
}

// writeList writes the shared list of select_test.go to a temporary file.
func writeList(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "list.txt", list)
	return filepath.Join(dir, "list.txt")
}

func runMain(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var o, e bytes.Buffer
	code = realMain(args, &o, &e)
	return code, o.String(), e.String()
}

// An ONLY that RE2 cannot read exits 2 with a message, and writes no list: the filter is never
// dropped in silence (grep -E would have read some of these).
func TestBeforeReadyInvalidOnlyExits2(t *testing.T) {
	t.Parallel()
	listFile := writeList(t)
	for _, only := range []string{"(", "internal/(play", "[a-", "(?=play)", `\1`, "*play"} {
		out := filepath.Join(t.TempDir(), "out.txt")
		code, _, stderr := runMain(t, "-list", listFile, "-out", out, "-all", "-only", only)
		if code != 2 || !strings.Contains(stderr, "not a valid regex") || !strings.Contains(stderr, "BEFORE_READY_ONLY") {
			t.Errorf("only %q: exit %d, stderr %q", only, code, stderr)
		}
		if _, err := os.Stat(out); err == nil {
			t.Errorf("only %q: the list to run was written", only)
		}
	}
}

func TestBeforeReadyOnlyMatchingNoPackageExits2(t *testing.T) {
	t.Parallel()
	code, _, stderr := runMain(t, "-list", writeList(t), "-out", filepath.Join(t.TempDir(), "o"), "-all", "-only", "internal/nowhere$")
	if code != 2 || !strings.Contains(stderr, "matches no package of the list") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestBeforeReadyOnlyNarrowsTheListToRun(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "out.txt")
	code, stdout, stderr := runMain(t, "-list", writeList(t), "-out", out, "-all", "-only", "internal/play$")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "regen play") || !strings.Contains(string(got), "regen stored") || strings.Contains(string(got), "regen api") || strings.Contains(string(got), "regen leak") {
		t.Errorf("list to run:\n%s", got)
	}
	if !strings.Contains(stdout, "internal/play") || strings.Contains(stdout, "cmd/api") {
		t.Errorf("stdout:\n%s", stdout)
	}
}

// A custom list runs all its lines and says so; the selection by change (which reads the
// project's own list) is not used.
func TestBeforeReadyCustomListRunsEveryLine(t *testing.T) {
	t.Chdir(t.TempDir()) // not a git tree: the facts are not even tried
	listFile := writeList(t)
	out := filepath.Join(t.TempDir(), "out.txt")
	code, stdout, stderr := runMain(t, "-list", listFile, "-out", out, "-custom")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "BEFORE_READY_LIST is set: running every line of "+listFile) {
		t.Errorf("stdout does not say so:\n%s", stdout)
	}
	got, err := os.ReadFile(out)
	if err != nil || strings.TrimSpace(string(got)) != strings.TrimSpace(list) {
		t.Errorf("list to run = %q, %v", got, err)
	}
	for _, pkg := range []string{"cmd/api", "internal/leaktest", "migrations"} {
		if !strings.Contains(stdout, pkg) {
			t.Errorf("stdout lacks %s:\n%s", pkg, stdout)
		}
	}
}

func TestPlanACustomListRunsEveryLine(t *testing.T) {
	t.Parallel()
	lines, f := setup(t)
	f.Changed = nil // a change that would select nothing
	run, skipped, note := plan(lines, false, "my.txt", f, nil)
	if len(run) != len(lines) || len(skipped) != 0 || !strings.Contains(note, "my.txt") || !strings.Contains(note, "running every line") {
		t.Errorf("run %d of %d, skipped %d, note %q", len(run), len(lines), len(skipped), note)
	}
}

func TestBeforeReadyQueries(t *testing.T) {
	t.Parallel()
	listFile := writeList(t)
	code, stdout, _ := runMain(t, "-list", listFile, "-packages")
	want := "cmd/api\ninternal/archtest\ninternal/leaktest\ninternal/play\ninternal/rules\ninternal/rules/combat\nmigrations\n"
	if code != 0 || stdout != want {
		t.Errorf("packages: exit %d, %q", code, stdout)
	}
	if _, stdout, _ = runMain(t, "-list", listFile, "-regex-of", "internal/play"); stdout != "(^TestGolden$)|(^TestStored$)\n" {
		t.Errorf("regex-of = %q", stdout)
	}
	if code, _, _ = runMain(t, "-list", listFile, "-regex-of", "internal/none"); code != 2 {
		t.Errorf("regex-of an unknown package: exit %d", code)
	}
	if _, stdout, _ = runMain(t, "-list", listFile, "-regen-of", "internal/play", "-test", "TestStored"); stdout != "regen stored\n" {
		t.Errorf("regen-of = %q", stdout)
	}
	if _, stdout, _ = runMain(t, "-list", listFile, "-regen-of", "internal/play", "-test", "TestOther"); stdout != "\n" {
		t.Errorf("regen-of with no match = %q", stdout)
	}
}

func TestRegenForReplacesTheTestName(t *testing.T) {
	t.Parallel()
	lines, err := ParseList(strings.NewReader("run\tinternal/x\t^TestA$\tgo test -run '^{test}$' -update\t-\nrun\tinternal/x\t^Test\tby hand\t-\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := regenFor(lines, "internal/x", "TestA"); got != "go test -run '^TestA$' -update" {
		t.Errorf("TestA: %q", got)
	}
	if got := regenFor(lines, "internal/x", "TestB"); got != "by hand" {
		t.Errorf("TestB (the first matching line wins): %q", got)
	}
}
