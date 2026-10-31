package main

import (
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
	run, skipped, note := plan(lines, false, Facts{}, os.ErrNotExist)
	if len(run) != 1 || len(skipped) != 0 || !strings.Contains(note, "running every line") || run[0].Reason != "the selection could not be computed" {
		t.Errorf("plan = %v %v %q", run, skipped, note)
	}
}
