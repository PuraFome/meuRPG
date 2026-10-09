package campaignpackage

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
)

// ErrTooBig means the package being written passes a limit of the format.
var ErrTooBig = errors.New("campaignpackage: the package passes a limit of the format")

// jsonOut writes a message the way the format wants it: the field names of
// the .proto.
var jsonOut = protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: false}

// Writer writes a package to w as a zip, one entry at a time, never holding
// more than the entry being written: a JSON entry is in memory (it is small,
// and the limit is 10 MiB), a file is copied through. It counts every byte so
// that the package stays under the limits, and writes the manifest last, with
// the SHA-256 of what went in.
type Writer struct {
	zw      *zip.Writer
	counter *countingWriter
	entries []*pkgv1.PackageEntry
	names   map[string]bool
	total   int64
	when    time.Time
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// NewWriter starts a package on w. when is the time every entry is stamped
// with (the export's).
func NewWriter(w io.Writer, when time.Time) *Writer {
	cw := &countingWriter{w: w}
	return &Writer{zw: zip.NewWriter(cw), counter: cw, names: map[string]bool{}, when: when.UTC()}
}

// AddMessage writes m as a JSON entry.
func (w *Writer) AddMessage(kind pkgv1.PackageEntryKind, name string, m proto.Message) error {
	b, err := jsonOut.Marshal(m)
	if err != nil {
		return fmt.Errorf("encode %s: %w", name, err)
	}
	return w.AddBytes(kind, name, b, zip.Deflate)
}

// AddBytes writes data as an entry.
func (w *Writer) AddBytes(kind pkgv1.PackageEntryKind, name string, data []byte, method uint16) error {
	if int64(len(data)) > MaxEntryBytes {
		return ErrTooBig
	}
	return w.add(kind, name, method, func(dst io.Writer) (int64, error) {
		n, err := dst.Write(data)
		return int64(n), err
	})
}

// AddFile copies r into an entry. The entry is stored as it is (images are
// compressed already). It fails with ErrTooBig as soon as r passes the limit.
func (w *Writer) AddFile(kind pkgv1.PackageEntryKind, name string, r io.Reader) error {
	return w.add(kind, name, zip.Store, func(dst io.Writer) (int64, error) {
		n, err := io.Copy(dst, io.LimitReader(r, MaxEntryBytes+1))
		if err == nil && n > MaxEntryBytes {
			err = ErrTooBig
		}
		return n, err
	})
}

func (w *Writer) add(kind pkgv1.PackageEntryKind, name string, method uint16, fill func(io.Writer) (int64, error)) error {
	if !ValidEntryName(name) || name == ManifestName {
		return fmt.Errorf("campaignpackage: %q is not a name for an entry", name)
	}
	if w.names[name] {
		return fmt.Errorf("campaignpackage: the entry %q is written twice", name)
	}
	if len(w.entries)+2 > MaxEntries { // this one, and the manifest
		return ErrTooBig
	}
	ew, err := w.zw.CreateHeader(&zip.FileHeader{Name: name, Method: method, Modified: w.when})
	if err != nil {
		return fmt.Errorf("start %s: %w", name, err)
	}
	h := sha256.New()
	n, err := fill(io.MultiWriter(ew, h))
	if err != nil {
		return err
	}
	w.total += n
	// The package is the compressed bytes; the uncompressed sum is checked as
	// well, because the reader judges both.
	if w.total > MaxTotalBytes || w.counter.n > MaxPackageBytes {
		return ErrTooBig
	}
	w.names[name] = true
	w.entries = append(w.entries, &pkgv1.PackageEntry{Path: name, Kind: kind, Sha256: hex.EncodeToString(h.Sum(nil)), Size: n})
	return nil
}

// Entries is how many entries have been written, the manifest not counted.
func (w *Writer) Entries() int { return len(w.entries) }

// Written is how many bytes of the zip have been written so far.
func (w *Writer) Written() int64 { return w.counter.n }

// Close writes the manifest and the end of the zip. contentVersion and
// campaignName go into the manifest.
func (w *Writer) Close(contentVersion, campaignName string) error {
	m := &pkgv1.PackageManifest{
		FormatVersion:  FormatVersion,
		ContentVersion: contentVersion,
		ExportedAt:     timestamppb.New(w.when),
		CampaignName:   campaignName,
		Entries:        w.entries,
	}
	b, err := jsonOut.Marshal(m)
	if err != nil {
		return fmt.Errorf("encode the manifest: %w", err)
	}
	if int64(len(b)) > MaxEntryBytes {
		return ErrTooBig
	}
	mw, err := w.zw.CreateHeader(&zip.FileHeader{Name: ManifestName, Method: zip.Deflate, Modified: w.when})
	if err != nil {
		return fmt.Errorf("start the manifest: %w", err)
	}
	if _, err := mw.Write(b); err != nil {
		return fmt.Errorf("write the manifest: %w", err)
	}
	if err := w.zw.Close(); err != nil {
		return fmt.Errorf("finish the zip: %w", err)
	}
	if w.counter.n > MaxPackageBytes {
		return ErrTooBig
	}
	return nil
}
