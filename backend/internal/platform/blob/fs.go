package blob

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
)

// tmpDir holds files that are still being written. A key segment must start
// with a letter or a digit (ValidKey), so no key can ever name it.
const tmpDir = ".tmp"

// maxContentTypeLength bounds the header line of a stored file.
const maxContentTypeLength = 127

// FS is a Store on the local filesystem, rooted at one directory: each key
// is a file under it, at the key's path.
//
// A file starts with one line holding the content type, such as
// "image/jpeg\n", followed by the content. That keeps the content type and
// the content in a single file, so a Put replaces both with one atomic
// rename. Open skips the line.
//
// Every file access goes through an os.Root, which refuses any path that
// leaves the directory, even through a symbolic link. ValidKey already
// refuses such keys; the os.Root is the second lock.
type FS struct {
	root *os.Root
}

// The compiler checks that FS implements Store.
var _ Store = (*FS)(nil)

// NewFS returns a Store that keeps its files under dir, creating dir if it
// does not exist. It fails if dir cannot be written to, so a wrong BLOB_DIR
// stops the server at startup instead of failing the first upload.
//
// Files left in dir's temporary folder by a crash in the middle of a Put
// are deleted. Only one process may use dir at a time.
func NewFS(dir string) (*FS, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("blob: create %s: %w", dir, err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("blob: open %s: %w", dir, err)
	}
	// Recreating the temporary folder also proves that dir is writable.
	if err := root.RemoveAll(tmpDir); err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("blob: clean %s: %w", path.Join(dir, tmpDir), err)
	}
	if err := root.Mkdir(tmpDir, 0o700); err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("blob: %s is not writable: %w", dir, err)
	}
	return &FS{root: root}, nil
}

// Close releases the directory. The FS must not be used afterwards.
func (s *FS) Close() error {
	return s.root.Close()
}

// Put implements Store. It writes to a temporary file, flushes it to disk
// and renames it over the key, so a reader or a crash never sees half a
// file.
func (s *FS) Put(ctx context.Context, key, contentType string, r io.Reader) error {
	if !ValidKey(key) {
		return ErrInvalidKey
	}
	if !validContentType(contentType) {
		return fmt.Errorf("blob: invalid content type %q", contentType)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if dir := path.Dir(key); dir != "." {
		if err := s.root.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("blob: create the key's folder: %w", withoutPath(err))
		}
	}

	tmp := tmpDir + "/" + rand.Text()
	f, err := s.root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("blob: create a temporary file: %w", withoutPath(err))
	}
	// Until the rename succeeds, the temporary file is ours to clean up.
	// (Closing it twice is harmless: the second Close only returns an
	// error, which is ignored.)
	done := false
	defer func() {
		if !done {
			_ = f.Close()
			_ = s.root.Remove(tmp)
		}
	}()

	if _, err := io.WriteString(f, contentType+"\n"); err != nil {
		return fmt.Errorf("blob: write: %w", withoutPath(err))
	}
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("blob: write: %w", withoutPath(err))
	}
	// Sync before the rename: otherwise, after a power cut, the new name
	// could point at a file whose content never reached the disk.
	if err := f.Sync(); err != nil {
		return fmt.Errorf("blob: flush: %w", withoutPath(err))
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("blob: close: %w", withoutPath(err))
	}
	if err := s.root.Rename(tmp, key); err != nil {
		return fmt.Errorf("blob: rename into place: %w", withoutPath(err))
	}
	done = true
	return nil
}

// Open implements Store.
func (s *FS) Open(ctx context.Context, key string) (*Object, error) {
	if !ValidKey(key) {
		return nil, ErrInvalidKey
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := s.root.Open(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("blob: open: %w", withoutPath(err))
	}
	obj, err := readObject(f)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return obj, nil
}

// readObject reads the header line of an open blob file and returns the
// content that follows it.
func readObject(f *os.File) (*Object, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("blob: stat: %w", withoutPath(err))
	}
	if info.IsDir() {
		// A key that is only a prefix of other keys ("campaigns/x").
		return nil, ErrNotFound
	}
	header := make([]byte, maxContentTypeLength+1)
	n, err := f.ReadAt(header, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("blob: read: %w", withoutPath(err))
	}
	end := bytes.IndexByte(header[:n], '\n')
	if end < 0 {
		return nil, errors.New("blob: the file has no content type line")
	}
	offset := int64(end + 1)
	return &Object{
		ContentType: string(header[:end]),
		Size:        info.Size() - offset,
		Content:     io.NewSectionReader(f, offset, info.Size()-offset),
		close:       f.Close,
	}, nil
}

// Delete implements Store.
func (s *FS) Delete(ctx context.Context, key string) error {
	if !ValidKey(key) {
		return ErrInvalidKey
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	err := s.root.Remove(key)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("blob: delete: %w", withoutPath(err))
	}
	return nil
}

// withoutPath drops the file name from an error of package os. A key holds
// IDs, and IDs never go to the logs (docs/privacy.md); the error that is
// left (such as "permission denied") still says what went wrong, and
// errors.Is still sees fs.ErrNotExist.
func withoutPath(err error) error {
	if pe, ok := errors.AsType[*fs.PathError](err); ok {
		return pe.Err
	}
	if le, ok := errors.AsType[*os.LinkError](err); ok {
		return le.Err
	}
	return err
}

// validContentType accepts a short media type in printable ASCII, so it
// fits the header line: no line break, no control character.
func validContentType(ct string) bool {
	if ct == "" || len(ct) > maxContentTypeLength {
		return false
	}
	for i := range len(ct) {
		if ct[i] < 0x20 || ct[i] > 0x7e {
			return false
		}
	}
	return true
}
