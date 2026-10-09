// Package campaignpackage is the campaign package (MR-050): the file a master
// exports a campaign into and the upload that creates a new campaign from one.
//
// This package knows the container and nothing about the game: the zip and
// its manifest (format.go, writer.go, reader.go), the upload in parts
// (upload.go), the import's working state (importer.go), the export job
// (export.go) and the RPC and HTTP routes (service.go). What goes into each
// entry, and what a campaign needs to be created from them, belongs to the
// modules that own the tables: each implements Part (parts.go) and the
// service runs them in order.
//
// The format is described in proto/meurpg/campaignpackage/v1/format.proto and
// in docs/architecture.md#campaign-package.
package campaignpackage

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
)

// FormatVersion is the format this server writes and the newest it reads.
const FormatVersion = 1

// The limits of a package. A package past any of them is refused before
// anything is created, and an export that would pass them fails.
const (
	// MaxPackageBytes is the size of the zip file: 200 MiB.
	MaxPackageBytes = 200 << 20
	// MaxEntries is the number of entries of the zip, the manifest included.
	MaxEntries = 2000
	// MaxEntryBytes is the uncompressed size of one entry: 10 MiB, the size of
	// the biggest image the gallery takes.
	MaxEntryBytes = 10 << 20
	// MaxTotalBytes is the sum of the uncompressed sizes of the entries. The
	// images are already compressed, so a real package is not far from its
	// own size; this stops a package that is small only because it squeezes a
	// huge amount of repetition.
	MaxTotalBytes = 500 << 20
	// MaxRatio is how many times an entry may grow when it is decompressed.
	// Only entries over ratioFloor are judged: a small entry cannot hurt.
	MaxRatio   = 100
	ratioFloor = 1 << 20
)

// ManifestName is the manifest's entry.
const ManifestName = "manifest.json"

// maxNameLength bounds an entry's name.
const maxNameLength = 100

var nameShape = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*(/[a-z0-9][a-z0-9._-]*)*$`)

// ValidEntryName reports whether name is an entry name this format allows:
// lowercase letters, digits, ".", "-", "_" and "/" between segments, every
// segment starting with a letter or a digit, no "..". A name like this can
// never climb out of a folder, and the reader never uses it as a path anyway.
func ValidEntryName(name string) bool {
	return len(name) <= maxNameLength && nameShape.MatchString(name) && !strings.Contains(name, "..")
}

// The folders and names of the entries.
const (
	CampaignEntry = "campaign.json"
	ImagesEntry   = "images.json"
)

// EntryName is the name of the n-th entry (from 1) of a kind that has many.
func EntryName(folder string, n int, ext string) string {
	return fmt.Sprintf("%s/%d%s", folder, n, ext)
}

// FileSuffix is the end of the file name of a package.
const FileSuffix = ".meurpg.zip"

// maxFileNameLetters bounds the campaign's part of the file name.
const maxFileNameLetters = 60

// FileName is the name the download suggests: the campaign's name made safe
// for every file system and for the header that carries it. Accents are taken
// off, anything but letters, digits, dots, dashes and underscores becomes an
// underscore, and nothing is left at the edges, so the name is plain ASCII and
// cannot start a path, a hidden file or a header. A name with nothing usable
// is "campanha".
func FileName(campaign string) string {
	var b strings.Builder
	lastUnderscore := true // no underscore at the start
	for _, r := range norm.NFD.String(campaign) {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue // the accent the decomposition split off
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			lastUnderscore = false
		case r == '-' || r == '_' || r == '.':
			if !lastUnderscore || r != '.' {
				b.WriteRune(r)
			}
			lastUnderscore = r == '_'
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
		if b.Len() >= maxFileNameLetters {
			break
		}
	}
	name := strings.Trim(b.String(), "._-")
	if name == "" {
		name = "campanha"
	}
	return name + FileSuffix
}

// Problems collects what stops a package from being created, in the order it
// was found. The zero value is ready.
type Problems struct {
	List []*pkgv1.PackageProblem
}

// maxProblems bounds the list: a package with thousands of bad things says the
// first ones and that is enough to fix it.
const maxProblems = 100

// Add records a problem. Beyond maxProblems it only counts: the list is cut.
func (p *Problems) Add(kind pkgv1.PackageProblemKind, name string, reason pkgv1.PackageProblemReason, limit int64) {
	if len(p.List) >= maxProblems {
		return
	}
	p.List = append(p.List, &pkgv1.PackageProblem{Kind: kind, Name: name, Reason: reason, Limit: limit})
}

// Any reports whether a problem was recorded.
func (p *Problems) Any() bool { return len(p.List) > 0 }
