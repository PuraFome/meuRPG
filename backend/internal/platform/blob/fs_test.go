package blob

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func newFS(t *testing.T) (*FS, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := NewFS(dir)
	if err != nil {
		t.Fatalf("NewFS() error = %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, dir
}

func read(t *testing.T, s Store, key string) (string, []byte) {
	t.Helper()
	obj, err := s.Open(t.Context(), key)
	if err != nil {
		t.Fatalf("Open(%q) error = %v", key, err)
	}
	defer func() { _ = obj.Close() }()
	content, err := io.ReadAll(obj.Content)
	if err != nil {
		t.Fatalf("read %q: %v", key, err)
	}
	if int64(len(content)) != obj.Size {
		t.Errorf("Open(%q).Size = %d, but the content has %d bytes", key, obj.Size, len(content))
	}
	return obj.ContentType, content
}

func TestPutOpenDelete(t *testing.T) {
	t.Parallel()
	s, dir := newFS(t)
	ctx := t.Context()
	key := "campaigns/c1/images/i1"
	content := []byte("\xff\xd8\xff not really a JPEG\nwith a line break")

	if err := s.Put(ctx, key, "image/jpeg", bytes.NewReader(content)); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if ct, got := read(t, s, key); ct != "image/jpeg" || !bytes.Equal(got, content) {
		t.Errorf("Open() = %q, %q; want image/jpeg, %q", ct, got, content)
	}

	// Seeking works, for http.ServeContent's range requests.
	obj, err := s.Open(ctx, key)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := obj.Content.Seek(4, io.SeekStart); err != nil {
		t.Fatalf("Seek() error = %v", err)
	}
	rest, _ := io.ReadAll(obj.Content)
	_ = obj.Close()
	if !bytes.Equal(rest, content[4:]) {
		t.Errorf("after Seek(4), content = %q, want %q", rest, content[4:])
	}

	// Put replaces.
	if err := s.Put(ctx, key, "image/png", strings.NewReader("new")); err != nil {
		t.Fatalf("second Put() error = %v", err)
	}
	if ct, got := read(t, s, key); ct != "image/png" || string(got) != "new" {
		t.Errorf("after replacing, Open() = %q, %q; want image/png, new", ct, got)
	}

	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := s.Open(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Errorf("Open() after Delete error = %v, want ErrNotFound", err)
	}
	// Deleting again is not an error.
	if err := s.Delete(ctx, key); err != nil {
		t.Errorf("second Delete() error = %v, want nil", err)
	}

	// Nothing is left in the temporary folder.
	if entries, err := os.ReadDir(filepath.Join(dir, tmpDir)); err != nil || len(entries) != 0 {
		t.Errorf("temporary folder = %v, %v; want empty", entries, err)
	}
}

func TestOpenMissing(t *testing.T) {
	t.Parallel()
	s, _ := newFS(t)
	if err := s.Put(t.Context(), "campaigns/c1/images/i1", "image/png", strings.NewReader("x")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	for _, key := range []string{"campaigns/c1/images/i2", "campaigns/c1", "nothing"} {
		if _, err := s.Open(t.Context(), key); !errors.Is(err, ErrNotFound) {
			t.Errorf("Open(%q) error = %v, want ErrNotFound", key, err)
		}
	}
}

func TestInvalidKeysAreRefused(t *testing.T) {
	t.Parallel()
	s, dir := newFS(t)
	ctx := t.Context()
	for _, key := range []string{
		"",
		"/etc/passwd",
		"../outside",
		"a/../../outside",
		"a/./b",
		"a//b",
		"a/b/",
		".tmp/x",
		".hidden",
		"Upper",
		`a\b`,
		"a\x00b",
		"a b",
		"ção",
		strings.Repeat("a", maxKeyLength+1),
	} {
		if ValidKey(key) {
			t.Errorf("ValidKey(%q) = true, want false", key)
		}
		if err := s.Put(ctx, key, "image/png", strings.NewReader("x")); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Put(%q) error = %v, want ErrInvalidKey", key, err)
		}
		if _, err := s.Open(ctx, key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Open(%q) error = %v, want ErrInvalidKey", key, err)
		}
		if err := s.Delete(ctx, key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Delete(%q) error = %v, want ErrInvalidKey", key, err)
		}
	}
	for _, key := range []string{"a", "campaigns/6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001/images/x.thumb", "a_b-c.d/0"} {
		if !ValidKey(key) {
			t.Errorf("ValidKey(%q) = false, want true", key)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "outside")); !os.IsNotExist(err) {
		t.Errorf("a file was written outside the store: %v", err)
	}
}

// TestSymlinksCannotEscape: even a symbolic link planted inside the store
// (by something other than this package) cannot lead a key out of it.
func TestSymlinksCannotEscape(t *testing.T) {
	t.Parallel()
	s, dir := newFS(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "campaigns")); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}
	if err := s.Put(t.Context(), "campaigns/c1/images/i1", "image/png", strings.NewReader("x")); err == nil {
		t.Error("Put() through a symlink that leaves the store succeeded")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Errorf("the folder outside the store has %v", entries)
	}
}

func TestInvalidContentTypesAreRefused(t *testing.T) {
	t.Parallel()
	s, _ := newFS(t)
	for _, ct := range []string{"", "image/png\nX-Evil: 1", "image/\x00png", strings.Repeat("a", maxContentTypeLength+1)} {
		if err := s.Put(t.Context(), "k", ct, strings.NewReader("x")); err == nil {
			t.Errorf("Put(content type %q) succeeded", ct)
		}
	}
}

// TestConcurrentPutsNeverShowHalfAFile writes two different contents to the
// same key over and over while reading it: every read must see one of the
// two whole contents.
func TestConcurrentPutsNeverShowHalfAFile(t *testing.T) {
	t.Parallel()
	s, _ := newFS(t)
	ctx := t.Context()
	a, b := bytes.Repeat([]byte("a"), 64<<10), bytes.Repeat([]byte("b"), 64<<10)
	if err := s.Put(ctx, "k", "text/plain", bytes.NewReader(a)); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	var wg sync.WaitGroup
	for _, content := range [][]byte{a, b} {
		wg.Go(func() {
			for range 20 {
				if err := s.Put(ctx, "k", "text/plain", bytes.NewReader(content)); err != nil {
					t.Errorf("Put() error = %v", err)
					return
				}
			}
		})
	}
	wg.Go(func() {
		for range 40 {
			_, got := read(t, s, "k")
			if !bytes.Equal(got, a) && !bytes.Equal(got, b) {
				t.Errorf("read %d bytes that are neither content", len(got))
				return
			}
		}
	})
	wg.Wait()
}

func TestNewFSCleansTheTemporaryFolder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, tmpDir), 0o700); err != nil {
		t.Fatal(err)
	}
	leftover := filepath.Join(dir, tmpDir, "LEFTOVER")
	if err := os.WriteFile(leftover, []byte("half a file"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := NewFS(dir)
	if err != nil {
		t.Fatalf("NewFS() error = %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, err := os.Stat(leftover); !os.IsNotExist(err) {
		t.Errorf("the leftover temporary file is still there: %v", err)
	}
}

func TestNewFSFailsOnAReadOnlyFolder(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil { //nolint:gosec // G302: read-only on purpose
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: a folder needs x to be listed
	if s, err := NewFS(dir); err == nil {
		_ = s.Close()
		t.Error("NewFS() on a read-only folder succeeded")
	}
}

// TestErrorsHaveNoPaths: a key holds IDs, which never go to the logs.
func TestErrorsHaveNoPaths(t *testing.T) {
	t.Parallel()
	s, dir := newFS(t)
	key := "campaigns/6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001/images/x"
	if err := s.Put(t.Context(), key, "image/png", strings.NewReader("x")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	// Make the key's folder read-only, so replacing the file fails.
	folder := filepath.Join(dir, "campaigns", "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001", "images")
	if err := os.Chmod(folder, 0o500); err != nil { //nolint:gosec // G302: read-only on purpose
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(folder, 0o700) }) //nolint:gosec // G302: a folder needs x to be listed
	err := s.Delete(t.Context(), key)
	if os.Geteuid() != 0 && err == nil {
		t.Fatal("Delete() in a read-only folder succeeded")
	}
	if err != nil && strings.Contains(err.Error(), "6f1c7a52") {
		t.Errorf("error %q has the key's IDs", err)
	}
}
