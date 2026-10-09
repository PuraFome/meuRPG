package campaignpackage

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
)

// Package is a package opened for reading: the zip checked against the
// manifest, every entry's SHA-256 proven. The entries are read on demand, one
// at a time; nothing but the zip's directory and the manifest is in memory.
type Package struct {
	Manifest *pkgv1.PackageManifest

	files   map[string]*zip.File
	entries map[string]*pkgv1.PackageEntry
}

// The end-of-central-directory record, and the longest comment a zip may have
// after it.
const (
	eocdSize      = 22
	maxZipComment = 1<<16 - 1
	eocdSignature = 0x06054b50
	zip64Marker   = 0xFFFF
)

// entryCount reads the number of entries the zip's end record claims, without
// building the directory: archive/zip allocates a File for every entry, and a
// file made of nothing but directory records could ask for a lot of memory.
// ok is false when no end record is found.
func entryCount(ra io.ReaderAt, size int64) (n int, ok bool) {
	tail := int64(eocdSize + maxZipComment)
	if size < tail {
		tail = size
	}
	buf := make([]byte, tail)
	if _, err := ra.ReadAt(buf, size-tail); err != nil && !errors.Is(err, io.EOF) {
		return 0, false
	}
	for i := len(buf) - eocdSize; i >= 0; i-- {
		if binary.LittleEndian.Uint32(buf[i:]) != eocdSignature {
			continue
		}
		return int(binary.LittleEndian.Uint16(buf[i+10:])), true
	}
	return 0, false
}

// Open checks a package: the zip, the manifest and the hash of every entry. It
// returns the problems it finds; with any, the Package is nil.
func Open(ra io.ReaderAt, size int64) (*Package, []*pkgv1.PackageProblem) {
	var pr Problems
	fail := func(reason pkgv1.PackageProblemReason, name string, limit int64) (*Package, []*pkgv1.PackageProblem) {
		pr.Add(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE, name, reason, limit)
		return nil, pr.List
	}
	if size > MaxPackageBytes {
		return fail(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_PACKAGE_TOO_BIG, "", size)
	}
	count, ok := entryCount(ra, size)
	if !ok {
		return fail(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_A_PACKAGE, "", 0)
	}
	if count > MaxEntries || count == zip64Marker {
		return fail(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_TOO_MANY_ENTRIES, "", int64(count))
	}
	zr, err := zip.NewReader(ra, size)
	if err != nil {
		return fail(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_A_PACKAGE, "", 0)
	}
	p := &Package{files: map[string]*zip.File{}, entries: map[string]*pkgv1.PackageEntry{}}
	var total uint64
	for _, f := range zr.File {
		if !p.checkFile(f, &pr) {
			return nil, pr.List
		}
		total += f.UncompressedSize64
	}
	if pr.Any() {
		// An entry that is too big or squeezes too much is told as it is, by the
		// image's name when it is an image, and the rest of the package is not read.
		p.nameImages(zr, &pr)
		return nil, pr.List
	}
	if total > MaxTotalBytes {
		return fail(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_PACKAGE_TOO_BIG, "", int64(total)) //nolint:gosec // G115: at most 2,000 entries of MaxEntryBytes
	}
	mf, ok := p.files[ManifestName]
	if !ok {
		return fail(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_A_PACKAGE, "", 0)
	}
	manifest, reason := readManifest(mf)
	if reason != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNSPECIFIED {
		return fail(reason, "", 0)
	}
	p.Manifest = manifest
	if !p.matchManifest(&pr) || !p.verifyHashes(&pr) {
		return nil, pr.List
	}
	return p, nil
}

// checkFile judges one entry of the zip's directory. It reports whether to go
// on; a false answer has recorded the problem.
func (p *Package) checkFile(f *zip.File, pr *Problems) bool {
	reason := func(r pkgv1.PackageProblemReason, limit int64) bool {
		pr.Add(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE, printable(f.Name), r, limit)
		return false
	}
	switch {
	case f.Name != ManifestName && !ValidEntryName(f.Name), !f.Mode().IsRegular():
		return reason(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_BAD_ENTRY_NAME, 0)
	case f.Flags&1 != 0, f.Method != zip.Store && f.Method != zip.Deflate:
		return reason(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_A_PACKAGE, 0)
	}
	if _, dup := p.files[f.Name]; dup {
		return reason(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNEXPECTED_ENTRY, 0)
	}
	// Too big and too squeezed are recorded and the scan goes on, so a package
	// with two oversize images lists both.
	switch {
	case f.UncompressedSize64 > MaxEntryBytes:
		reason(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_ENTRY_TOO_BIG, int64(min(f.UncompressedSize64, 1<<62))) //nolint:gosec // G115: capped here
	case f.UncompressedSize64 > ratioFloor && f.UncompressedSize64 > MaxRatio*max(f.CompressedSize64, 1):
		reason(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_COMPRESSION_RATIO, MaxRatio)
	}
	p.files[f.Name] = f
	return true
}

// nameImages gives the problems about image files the image's name (from
// images.json, read as it is: this is only a label on a refusal): the master
// knows "Mapa antigo.png", not "images/7".
func (p *Package) nameImages(zr *zip.Reader, pr *Problems) {
	var index *zip.File
	for _, f := range zr.File {
		if f.Name == ImagesEntry && f.UncompressedSize64 <= MaxEntryBytes {
			index = f
		}
	}
	if index == nil {
		return
	}
	rc, err := index.Open()
	if err != nil {
		return
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(io.LimitReader(rc, MaxEntryBytes+1))
	if err != nil || len(data) > MaxEntryBytes {
		return
	}
	var list pkgv1.PackageImages
	if (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &list) != nil {
		return
	}
	for _, img := range list.GetImages() {
		for _, prob := range pr.List {
			if prob.GetKind() == pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE && prob.GetName() == img.GetFile() {
				prob.Kind, prob.Name = pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_IMAGE, printable(img.GetName())
			}
		}
	}
}

// printable makes an entry's name safe to put in a problem: it is the file's
// own text, so it is cut and its control characters go.
func printable(s string) string {
	const longest = 120
	s = strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f {
			return '?'
		}
		return r
	}, strings.ToValidUTF8(s, "?"))
	if len(s) > longest {
		s = strings.ToValidUTF8(s[:longest], "")
	}
	return s
}

// readManifest decodes the manifest. It looks at the format version before it
// is strict about the fields, so a newer package, whose manifest may carry
// fields this reader does not know, is told apart from a damaged one. A zero
// reason means the manifest is good.
func readManifest(f *zip.File) (*pkgv1.PackageManifest, pkgv1.PackageProblemReason) {
	notAPackage := pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_A_PACKAGE
	rc, err := f.Open()
	if err != nil {
		return nil, notAPackage
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(io.LimitReader(rc, MaxEntryBytes+1))
	if err != nil || len(data) > MaxEntryBytes {
		return nil, notAPackage
	}
	var peek pkgv1.PackageManifest
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &peek); err != nil || peek.GetFormatVersion() < 1 {
		return nil, notAPackage
	}
	if peek.GetFormatVersion() > FormatVersion {
		return nil, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NEWER_VERSION
	}
	m := &pkgv1.PackageManifest{}
	if err := (protojson.UnmarshalOptions{}).Unmarshal(data, m); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return nil, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNKNOWN_FIELD
		}
		return nil, notAPackage
	}
	return m, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNSPECIFIED
}

// matchManifest checks that the manifest and the zip describe the same
// entries.
func (p *Package) matchManifest(pr *Problems) bool {
	if len(p.Manifest.GetEntries()) >= MaxEntries { // the manifest is an entry too
		pr.Add(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_TOO_MANY_ENTRIES, int64(len(p.Manifest.GetEntries())))
		return false
	}
	for _, e := range p.Manifest.GetEntries() {
		reason := func(r pkgv1.PackageProblemReason) bool {
			pr.Add(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE, printable(e.GetPath()), r, 0)
			return false
		}
		switch {
		case !ValidEntryName(e.GetPath()) || e.GetPath() == ManifestName:
			return reason(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_BAD_ENTRY_NAME)
		case e.GetKind() == pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_UNSPECIFIED:
			return reason(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_A_PACKAGE)
		}
		if _, dup := p.entries[e.GetPath()]; dup {
			return reason(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNEXPECTED_ENTRY)
		}
		f, ok := p.files[e.GetPath()]
		if !ok {
			return reason(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_MISSING_ENTRY)
		}
		if int64(f.UncompressedSize64) != e.GetSize() { //nolint:gosec // G115: at most MaxEntryBytes, checked
			return reason(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_HASH_MISMATCH)
		}
		p.entries[e.GetPath()] = e
	}
	for name := range p.files {
		if _, ok := p.entries[name]; !ok && name != ManifestName {
			pr.Add(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE, printable(name), pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNEXPECTED_ENTRY, 0)
			return false
		}
	}
	return true
}

// verifyHashes reads every entry once and compares its SHA-256 with the
// manifest's. It reads no more than the size the entry declares plus one byte,
// so a header that lies about the size cannot make it decompress a bomb.
func (p *Package) verifyHashes(pr *Problems) bool {
	for _, e := range p.Manifest.GetEntries() {
		rc, err := p.files[e.GetPath()].Open()
		if err != nil {
			pr.Add(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE, printable(e.GetPath()), pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_A_PACKAGE, 0)
			return false
		}
		h := sha256.New()
		n, err := io.Copy(h, io.LimitReader(rc, e.GetSize()+1))
		_ = rc.Close()
		if err != nil || n != e.GetSize() || hex.EncodeToString(h.Sum(nil)) != e.GetSha256() {
			pr.Add(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE, printable(e.GetPath()), pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_HASH_MISMATCH, 0)
			return false
		}
	}
	return true
}

// Entries lists the manifest's entries of a kind, in the manifest's order.
func (p *Package) Entries(kind pkgv1.PackageEntryKind) []*pkgv1.PackageEntry {
	var out []*pkgv1.PackageEntry
	for _, e := range p.Manifest.GetEntries() {
		if e.GetKind() == kind {
			out = append(out, e)
		}
	}
	return out
}

// Entry returns the manifest's entry of a path, if the package has it.
func (p *Package) Entry(path string) (*pkgv1.PackageEntry, bool) {
	e, ok := p.entries[path]
	return e, ok
}

// Open opens an entry for reading. The reader stops at the entry's declared
// size, which Open already proved.
func (p *Package) Open(path string) (io.ReadCloser, error) {
	e, ok := p.entries[path]
	if !ok {
		return nil, fmt.Errorf("campaignpackage: no entry %q", path)
	}
	rc, err := p.files[path].Open()
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return &limitedReadCloser{Reader: io.LimitReader(rc, e.GetSize()), closer: rc}, nil
}

type limitedReadCloser struct {
	io.Reader
	closer io.Closer
}

func (l *limitedReadCloser) Close() error { return l.closer.Close() }

// ReadBytes reads a whole entry (at most MaxEntryBytes).
func (p *Package) ReadBytes(path string) ([]byte, error) {
	rc, err := p.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	b, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return b, nil
}

// ReadMessage decodes a JSON entry into m, refusing a field m does not have.
// A false answer carries the reason the entry cannot be read.
func (p *Package) ReadMessage(path string, m proto.Message) (pkgv1.PackageProblemReason, bool) {
	b, err := p.ReadBytes(path)
	if err != nil {
		return pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_MISSING_ENTRY, false
	}
	if err := (protojson.UnmarshalOptions{}).Unmarshal(bytes.TrimSpace(b), m); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNKNOWN_FIELD, false
		}
		return pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, false
	}
	return 0, true
}
