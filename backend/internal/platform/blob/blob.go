// Package blob stores files, such as the gallery's images, by key.
//
// Store is the whole interface: Put, Open and Delete. The filesystem
// implementation (NewFS) keeps each file under a directory, for the local
// stack and the tests. Production will use a private Cloud Storage bucket
// behind the same interface (docs/operacao.md, "A definir"); nothing
// outside this package needs to change for that.
//
// A key is a path made of lowercase letters, digits, dots, dashes and
// underscores, such as "campaigns/<id>/images/<id>.thumb" (ValidKey). The
// callers choose their keys; this package only refuses the ones that could
// escape the store.
package blob

import (
	"context"
	"errors"
	"io"
	"strings"
)

// ErrNotFound means there is no blob with that key.
var ErrNotFound = errors.New("blob: not found")

// ErrInvalidKey means the key breaks the rules of ValidKey.
var ErrInvalidKey = errors.New("blob: invalid key")

// Store keeps blobs. Its methods are safe to call from several goroutines.
type Store interface {
	// Put stores the content read from r under key, with its media type
	// (such as "image/jpeg"). It replaces a blob with the same key. The
	// write is atomic: Open sees the old blob or the new one, never half of
	// one.
	Put(ctx context.Context, key, contentType string, r io.Reader) error

	// Open opens the blob for reading, or returns ErrNotFound. The caller
	// must close the Object.
	Open(ctx context.Context, key string) (*Object, error)

	// Delete removes the blob. Deleting a key that does not exist is not an
	// error, so a retry is safe.
	Delete(ctx context.Context, key string) error
}

// Object is a blob opened for reading.
type Object struct {
	// ContentType is the media type given to Put.
	ContentType string
	// Size is the length of the content, in bytes.
	Size int64
	// Content reads the content. It can seek, so http.ServeContent can
	// answer range requests with it.
	Content io.ReadSeeker

	close func() error
}

// Close releases the blob.
func (o *Object) Close() error {
	if o.close == nil {
		return nil
	}
	return o.close()
}

// maxKeyLength keeps keys short enough for any filesystem and for Cloud
// Storage (object names up to 1,024 bytes).
const maxKeyLength = 512

// ValidKey reports whether key is a valid blob key: 1 to 512 bytes,
// segments separated by "/", each segment made of lowercase ASCII letters,
// digits, ".", "-" and "_", starting with a letter or a digit. So a key
// can never be absolute, empty in the middle ("a//b"), "." or "..", or
// carry a backslash or a NUL: nothing that could climb out of the store.
func ValidKey(key string) bool {
	if key == "" || len(key) > maxKeyLength {
		return false
	}
	for segment := range strings.SplitSeq(key, "/") {
		if segment == "" || !isAlnum(segment[0]) {
			return false
		}
		for i := range len(segment) {
			if c := segment[i]; !isAlnum(c) && c != '.' && c != '-' && c != '_' {
				return false
			}
		}
	}
	return true
}

func isAlnum(c byte) bool {
	return ('a' <= c && c <= 'z') || ('0' <= c && c <= '9')
}
