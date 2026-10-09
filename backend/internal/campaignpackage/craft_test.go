package campaignpackage_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage"
)

// crafted is a package taken apart: its entries and its manifest, to be changed
// and put together again.
type crafted struct {
	t        *testing.T
	manifest *pkgv1.PackageManifest
	files    map[string][]byte
	order    []string
	methods  map[string]uint16
}

func unpack(t *testing.T, data []byte) *crafted {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	c := &crafted{t: t, files: map[string][]byte{}, methods: map[string]uint16{}}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		c.files[f.Name], c.methods[f.Name] = b, f.Method
		c.order = append(c.order, f.Name)
	}
	c.manifest = &pkgv1.PackageManifest{}
	if err := protojson.Unmarshal(c.files[campaignpackage.ManifestName], c.manifest); err != nil {
		t.Fatal(err)
	}
	return c
}

// add puts an entry in the package and in the manifest, with its true hash.
func (c *crafted) add(kind pkgv1.PackageEntryKind, name string, data []byte) {
	c.files[name], c.methods[name] = data, zip.Deflate
	c.order = append(c.order, name)
	sum := sha256.Sum256(data)
	c.manifest.Entries = append(c.manifest.Entries, &pkgv1.PackageEntry{Path: name, Kind: kind, Sha256: hex.EncodeToString(sum[:]), Size: int64(len(data))})
}

// set replaces an entry's bytes and fixes the manifest, as a careful editor would.
func (c *crafted) set(name string, data []byte) {
	c.files[name] = data
	sum := sha256.Sum256(data)
	for _, e := range c.manifest.Entries {
		if e.GetPath() == name {
			e.Sha256, e.Size = hex.EncodeToString(sum[:]), int64(len(data))
		}
	}
}

// drop removes an entry from the package and the manifest.
func (c *crafted) drop(name string) {
	delete(c.files, name)
	c.order = slicesDelete(c.order, name)
	var kept []*pkgv1.PackageEntry
	for _, e := range c.manifest.Entries {
		if e.GetPath() != name {
			kept = append(kept, e)
		}
	}
	c.manifest.Entries = kept
}

func slicesDelete(s []string, v string) []string {
	var out []string
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

// entryOf finds the entry of a kind at a position.
func (c *crafted) entryOf(kind pkgv1.PackageEntryKind, i int) string {
	n := 0
	for _, e := range c.manifest.GetEntries() {
		if e.GetKind() == kind {
			if n == i {
				return e.GetPath()
			}
			n++
		}
	}
	c.t.Fatalf("no entry %d of kind %v", i, kind)
	return ""
}

// bytes puts the package together: the manifest as it is (so a stale hash stays stale).
func (c *crafted) bytes() []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range c.order {
		if name == campaignpackage.ManifestName {
			continue
		}
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: c.methods[name]})
		if err != nil {
			c.t.Fatal(err)
		}
		_, _ = fw.Write(c.files[name])
	}
	mj, err := protojson.Marshal(c.manifest)
	if err != nil {
		c.t.Fatal(err)
	}
	fw, _ := zw.Create(campaignpackage.ManifestName)
	_, _ = fw.Write(mj)
	_ = zw.Close()
	return buf.Bytes()
}

// storedKeys lists the keys in the blob store under a prefix, for the tests that
// say nothing must be left behind.
func (h *harness) storedKeys(prefix string) []string {
	h.t.Helper()
	var keys []string
	_ = filepath.WalkDir(h.blobDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // a path that cannot be read is not a key
		}
		rel, _ := filepath.Rel(h.blobDir, path)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, prefix) && !strings.HasPrefix(rel, ".tmp") {
			keys = append(keys, rel)
		}
		return nil
	})
	return keys
}

// count is a number from a query, for "nothing was left behind".
func (h *harness) count(query string, args ...any) int {
	h.t.Helper()
	var n int
	if err := h.pool.QueryRow(h.t.Context(), query, args...).Scan(&n); err != nil {
		h.t.Fatalf("%s: %v", query, err)
	}
	return n
}

func mustRead(c *crafted, name string, m proto.Message) (string, bool) {
	if err := protojson.Unmarshal(c.files[name], m); err != nil {
		return err.Error(), false
	}
	return "", true
}

func mustMarshal(t *testing.T, m proto.Message) []byte {
	t.Helper()
	b, err := protojson.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// detailsOf is the typed details of a Connect error.
func detailsOf(err error) []*connect.ErrorDetail {
	if ce, ok := errors.AsType[*connect.Error](err); ok {
		return ce.Details()
	}
	return nil
}
