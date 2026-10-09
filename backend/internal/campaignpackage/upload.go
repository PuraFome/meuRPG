package campaignpackage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/PuraFome/meuRPG/backend/internal/platform/blob"
)

// The upload goes up in parts of this size (the last one is shorter): small
// enough for Cloud Run's 32 MiB request limit with a wide margin, big enough
// that a 200 MiB package is about forty requests.
const PartSize = 5 << 20

// partKey is where part n (from 1) of an upload lives in the blob store.
func partKey(importID string, n int) string {
	return fmt.Sprintf("imports/%s/parts/%d", importID, n)
}

// PartCount is how many parts a file of total bytes goes up in.
func PartCount(total int64) int {
	return int((total + PartSize - 1) / PartSize)
}

// partLength is the length part n of an upload of total bytes must have.
func partLength(total int64, n int) int64 {
	if n < PartCount(total) {
		return PartSize
	}
	return total - int64(PartCount(total)-1)*PartSize
}

// readBlock is the size of the reads from the store. The zip reader asks for a
// few kilobytes at a time; asking the store for that much each time would be
// thousands of calls on a bucket.
const readBlock = 256 << 10

// partsReader reads the parts of an upload as one file (an io.ReaderAt), so
// the zip is read where it lies and the whole package is never assembled
// anywhere. It keeps one block of one part, which is what the sequential
// reads of a zip entry need.
type partsReader struct {
	ctx      context.Context
	store    blob.Store
	importID string
	total    int64

	mu       sync.Mutex
	obj      *blob.Object
	objPart  int
	block    []byte
	blockOff int64
}

func newPartsReader(ctx context.Context, store blob.Store, importID string, total int64) *partsReader {
	return &partsReader{ctx: ctx, store: store, importID: importID, total: total, blockOff: -1}
}

// Close releases the part that is open.
func (r *partsReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closeObj()
}

func (r *partsReader) closeObj() error {
	if r.obj == nil {
		return nil
	}
	err := r.obj.Close()
	r.obj, r.objPart = nil, 0
	return err
}

// ReadAt implements io.ReaderAt.
func (r *partsReader) ReadAt(p []byte, off int64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if off < 0 {
		return 0, errors.New("campaignpackage: negative offset")
	}
	n := 0
	for n < len(p) {
		pos := off + int64(n)
		if pos >= r.total {
			return n, io.EOF
		}
		if err := r.fill(pos); err != nil {
			return n, err
		}
		n += copy(p[n:], r.block[pos-r.blockOff:])
	}
	return n, nil
}

// fill makes the block that holds pos the current one.
func (r *partsReader) fill(pos int64) error {
	if r.blockOff >= 0 && pos >= r.blockOff && pos < r.blockOff+int64(len(r.block)) {
		return nil
	}
	part := int(pos/PartSize) + 1
	inPart := pos % PartSize
	start := inPart - inPart%readBlock
	if r.obj == nil || r.objPart != part {
		_ = r.closeObj()
		obj, err := r.store.Open(r.ctx, partKey(r.importID, part))
		if err != nil {
			return fmt.Errorf("open part %d: %w", part, err)
		}
		r.obj, r.objPart = obj, part
	}
	if _, err := r.obj.Content.Seek(start, io.SeekStart); err != nil {
		return fmt.Errorf("seek in part %d: %w", part, err)
	}
	want := min(int64(readBlock), partLength(r.total, part)-start)
	if int64(cap(r.block)) < want {
		r.block = make([]byte, want)
	}
	r.block = r.block[:want]
	if _, err := io.ReadFull(r.obj.Content, r.block); err != nil {
		return fmt.Errorf("read part %d: %w", part, err)
	}
	r.blockOff = int64(part-1)*PartSize + start
	return nil
}
