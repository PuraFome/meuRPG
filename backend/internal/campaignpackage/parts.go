package campaignpackage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"uuid"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/blob"
)

// ErrReplayed is what a part's Apply returns when the create's idempotency key
// already made a campaign: another call with the same key won the race. The
// import ends without error for the caller, who reads that campaign.
var ErrReplayed = errors.New("campaignpackage: the campaign of this key was already made")

// Part is one module's share of the package. The module that owns a table is
// the one that reads and writes it, so each module implements Part for its own
// kinds, and the service runs them in a fixed order.
type Part interface {
	// Export reads the module's rows of the campaign and adds their entries to
	// the snapshot. It runs inside the export's read transaction, which can run
	// more than once: it must only read through tx and only add to s.
	Export(ctx context.Context, tx pgx.Tx, campaignID string, s *Snapshot) error

	// Stage reads the module's entries from the package, checks every one with
	// the rules the app applies to what the master makes by hand, and prepares
	// what Apply needs, in memory (Import.Set). It records each thing that is
	// wrong as a problem and goes on, so one preview lists them all. It writes
	// nothing to the database. When in.Commit is true it also writes the files
	// the module owns to the blob store (recording them in in.Written, which the
	// service deletes if the import does not end); a preview does not.
	Stage(ctx context.Context, in *Import) error

	// Apply writes what Stage prepared, inside tx, into the new campaign
	// (in.CampaignID). It can run more than once (the database retries a
	// transaction): it must only write through tx, and make the same rows each
	// time. An error rolls the whole import back.
	Apply(ctx context.Context, tx pgx.Tx, in *Import) error
}

// Estimator is a Part that can say how many bytes it will add to a package,
// before the export runs (the export page shows "cerca de 84 MB").
type Estimator interface {
	EstimateBytes(ctx context.Context, campaignID string) (int64, error)
}

// Snapshot is what an export collects inside the read transaction. The JSON
// entries are kept as bytes (they are small); the files are named by their key
// in the blob store and copied through, one at a time, when the zip is
// written, so no image is held in memory.
type Snapshot struct {
	CampaignID string
	// CampaignName is set by the campaign's part: it names the file and goes in
	// the manifest.
	CampaignName string
	entries      []snapshotEntry
	counters     map[string]int
}

type snapshotEntry struct {
	kind pkgv1.PackageEntryKind
	name string
	data []byte
	blob string // the key of a file in the blob store; data is empty
	size int64  // the file's size, for the progress
}

// NewSnapshot starts an empty snapshot of a campaign.
func NewSnapshot(campaignID string) *Snapshot {
	return &Snapshot{CampaignID: campaignID, counters: map[string]int{}}
}

// Next numbers the entries of a folder: 1, 2, 3...
func (s *Snapshot) Next(folder string) int {
	s.counters[folder]++
	return s.counters[folder]
}

// AddMessage adds a JSON entry.
func (s *Snapshot) AddMessage(kind pkgv1.PackageEntryKind, name string, m proto.Message) error {
	b, err := jsonOut.Marshal(m)
	if err != nil {
		return fmt.Errorf("encode %s: %w", name, err)
	}
	s.entries = append(s.entries, snapshotEntry{kind: kind, name: name, data: b})
	return nil
}

// AddBlob adds a file that is in the blob store under key, size bytes long.
func (s *Snapshot) AddBlob(kind pkgv1.PackageEntryKind, name, key string, size int64) {
	s.entries = append(s.entries, snapshotEntry{kind: kind, name: name, blob: key, size: size})
}

// Entries is how many entries the snapshot holds.
func (s *Snapshot) Entries() int { return len(s.entries) }

// BlobBytes is the total size of the files the snapshot names.
func (s *Snapshot) BlobBytes() int64 {
	var n int64
	for _, e := range s.entries {
		n += e.size
	}
	return n
}

// Import is the working state of one read of a package: the package, the new
// ids, the problems and what each part prepared for its Apply. A preview and
// an import make one each; nothing in it outlives the call.
type Import struct {
	Pkg *Package
	// CampaignID is the id the new campaign will have, UserID the caller (its
	// master).
	CampaignID string
	UserID     string
	// CreateKey and CreateHash are the idempotency key of the create (scoped to
	// the caller) and the hash of its request; the campaign is stored with
	// them, so a retry finds it.
	CreateKey, CreateHash *string
	// Commit is false in a preview: nothing is written anywhere.
	Commit bool

	IDs      *IDs
	Problems Problems
	Counts   *pkgv1.PackageCounts
	// Facts are what the parts have found out about the package for the parts
	// after them.
	Facts Facts
	// Written lists the keys of the files put in the blob store so far.
	Written []string
	// Blobs is where files are written when Commit is true.
	Blobs blob.Store
	// CampaignName is the name of the campaign in the package, set by the
	// campaign part for the others to use in the messages.
	CampaignName string

	stash map[string]any
}

// Facts are what an earlier part tells the later ones, in the ids of the
// package, so the puzzle part can check what a puzzle names without the map
// part's types. A part that finds a problem in a thing leaves it out: a later
// part that names it is told it is not there.
type Facts struct {
	// DoorAt says whether the square of a map has a door.
	DoorAt func(mapID string, col, row int) bool
	// PointMap and PointKind give the map and the kind ("battle", "trap"...)
	// of each point.
	PointMap  map[string]string
	PointKind map[string]string
	// Players are the ids of the players' characters, not the NPCs.
	Players map[string]bool
}

// NewImport starts the state of a read of pkg.
func NewImport(pkg *Package, campaignID, userID string, commit bool, blobs blob.Store) *Import {
	return &Import{
		Pkg: pkg, CampaignID: campaignID, UserID: userID, Commit: commit, Blobs: blobs,
		IDs: NewIDs(), Counts: &pkgv1.PackageCounts{}, stash: map[string]any{},
		Facts: Facts{PointMap: map[string]string{}, PointKind: map[string]string{}, Players: map[string]bool{}},
	}
}

// Set keeps what a part prepared for its Apply.
func (in *Import) Set(key string, v any) { in.stash[key] = v }

// Get returns what Set kept under key, or nil.
func (in *Import) Get(key string) any { return in.stash[key] }

// Problem records a problem.
func (in *Import) Problem(kind pkgv1.PackageProblemKind, name string, reason pkgv1.PackageProblemReason, limit int64) {
	in.Problems.Add(kind, name, reason, limit)
}

// Read decodes a JSON entry into m. When it cannot be read it records a problem
// (about the file as a whole, named by the entry) and says false.
func (in *Import) Read(path string, m proto.Message) bool {
	reason, ok := in.Pkg.ReadMessage(path, m)
	if !ok {
		in.Problem(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE, printable(path), reason, 0)
	}
	return ok
}

// WriteBlob puts content in the blob store and remembers the key. In a preview
// it does nothing.
func (in *Import) WriteBlob(ctx context.Context, key, contentType string, r io.Reader) error {
	if !in.Commit {
		return nil
	}
	in.Written = append(in.Written, key)
	if err := in.Blobs.Put(ctx, key, contentType, r); err != nil {
		return fmt.Errorf("store %s: %w", key, err)
	}
	return nil
}

// IDs maps the ids of a package to the ids of the new campaign. A package id
// is only a handle: it is checked to be a UUID, and the new thing gets a new
// one, so nothing of a package's ids is ever used on a server.
//
// Ids live in namespaces ("image", "map", "point", "clue", "character",
// "puzzle"): the same text in two namespaces is two things.
type IDs struct {
	mapped  map[string]string
	defined map[string]bool
	refs    []idRef
}

type idRef struct {
	ns, old string
	kind    pkgv1.PackageProblemKind
	subject string
}

// NewIDs returns an empty map.
func NewIDs() *IDs { return &IDs{mapped: map[string]string{}, defined: map[string]bool{}} }

func idKey(ns, old string) string { return ns + "\x00" + old }

// ValidID reports whether s is a canonical UUID (lowercase, with dashes).
func ValidID(s string) bool {
	u, err := uuid.Parse(s)
	return err == nil && u.String() == s
}

// Define declares that the thing with this package id exists in the package
// and returns its new id. It says false for an id that is not a UUID or that
// the namespace already has.
func (ids *IDs) Define(ns, old string) (string, bool) {
	if !ValidID(old) || ids.defined[idKey(ns, old)] {
		return "", false
	}
	ids.defined[idKey(ns, old)] = true
	return ids.alloc(ns, old), true
}

func (ids *IDs) alloc(ns, old string) string {
	k := idKey(ns, old)
	if id, ok := ids.mapped[k]; ok {
		return id
	}
	id := uuid.New().String()
	ids.mapped[k] = id
	return id
}

// Ref is a use of an id: it returns the new id of the thing and records that
// the package must define it (Unresolved lists the ones it does not). An empty
// old id is no reference and returns "".
func (ids *IDs) Ref(ns, old string, kind pkgv1.PackageProblemKind, subject string) string {
	if old == "" {
		return ""
	}
	ids.refs = append(ids.refs, idRef{ns: ns, old: old, kind: kind, subject: subject})
	if !ValidID(old) {
		return ""
	}
	return ids.alloc(ns, old)
}

// Get returns the new id of a defined thing.
func (ids *IDs) Get(ns, old string) (string, bool) {
	if !ids.defined[idKey(ns, old)] {
		return "", false
	}
	return ids.mapped[idKey(ns, old)], true
}

// Unresolved records, as problems, every reference to a thing the package
// does not define.
func (ids *IDs) Unresolved(pr *Problems) {
	for _, r := range ids.refs {
		if !ids.defined[idKey(r.ns, r.old)] {
			pr.Add(r.kind, r.subject, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_IN_PACKAGE, 0)
		}
	}
}

var documentLink = regexp.MustCompile(`\((map|character):([0-9a-fA-F-]{36})\)|\(image:([0-9a-fA-F-]{36})\)`)

// RewriteDocument gives the links of the campaign document (map:<id>,
// character:<id> and image:<id>) the new ids. A link to something the package
// does not hold would point at an id of the old server; it is given a fresh id
// that names nothing, which the app shows as "apagado" like any link to a
// thing that was deleted. The fresh ids differ on each call, so the caller
// keeps the result (Import.Set) instead of rewriting again on a retry.
func (ids *IDs) RewriteDocument(body string) string {
	return documentLink.ReplaceAllStringFunc(body, func(m string) string {
		inner := strings.TrimSuffix(strings.TrimPrefix(m, "("), ")")
		scheme, old, _ := strings.Cut(inner, ":")
		ns := scheme
		if id, ok := ids.Get(ns, strings.ToLower(old)); ok {
			return "(" + scheme + ":" + id + ")"
		}
		return "(" + scheme + ":" + uuid.New().String() + ")"
	})
}
