package campaignpackage

import (
	"archive/zip"
	"bytes"
	"hash/crc32"
	"strings"
	"testing"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
)

// Characterization tests of FileName and of nameImages (the label an image's
// problem gets from images.json), frozen before both are split. They need no
// database.

func TestCharacterizeFileNameEdges(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"":                            "campanha.meurpg.zip",
		"a_.b":                        "a_b.meurpg.zip",
		"a-.b":                        "a-.b.meurpg.zip",
		"a._b":                        "a._b.meurpg.zip",
		"a__b":                        "a__b.meurpg.zip",
		"a _b":                        "a__b.meurpg.zip",
		"a _.b":                       "a__b.meurpg.zip",
		"a  b":                        "a_b.meurpg.zip",
		"a...b":                       "a...b.meurpg.zip",
		"_-._a._-_":                   "a.meurpg.zip",
		"Äpfel":                       "Apfel.meurpg.zip",
		"ﬁm":                          "m.meurpg.zip",
		"x" + "́":                     "x.meurpg.zip",
		"1d20 + 5":                    "1d20_5.meurpg.zip",
		"!!!":                         "campanha.meurpg.zip",
		"a!":                          "a.meurpg.zip",
		"Ç" + "!":                     "C.meurpg.zip",
		" a ":                         "a.meurpg.zip",
		"tab\there":                   "tab_here.meurpg.zip",
		"ABC.def-GHI_jkl":             "ABC.def-GHI_jkl.meurpg.zip",
		"é" + strings.Repeat("b", 70): "e" + strings.Repeat("b", 59) + ".meurpg.zip",
		strings.Repeat("a ", 40):      strings.Repeat("a_", 29) + "a" + ".meurpg.zip",
		// FINDING (frozen): the dots that are trimmed at the end still
		// count toward the 60 letters, so a name that starts with 60 dots
		// loses what follows and falls back to "campanha".
		strings.Repeat(".", 70) + "z": "campanha.meurpg.zip",
		strings.Repeat("_", 61) + "z": "campanha.meurpg.zip",
	} {
		if got := FileName(in); got != want {
			t.Errorf("FileName(%q) = %q, want %q", in, got, want)
		}
	}
}

// bombedPackage is a zip whose images/1 squeezes too much (a problem that
// sends Open to nameImages) followed by an images.json written by index.
func bombedPackage(t *testing.T, index func(zw *zip.Writer)) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, err := zw.CreateHeader(&zip.FileHeader{Name: "images/1", Method: zip.Deflate})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(make([]byte, MaxEntryBytes)); err != nil {
		t.Fatal(err)
	}
	if index != nil {
		index(zw)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// storedIndex writes data as images.json, stored, with crc as its CRC-32.
func storedIndex(t *testing.T, zw *zip.Writer, data []byte, crc uint32) {
	t.Helper()
	fw, err := zw.CreateRaw(&zip.FileHeader{
		Name: ImagesEntry, Method: zip.Store, CRC32: crc,
		CompressedSize64: uint64(len(data)), UncompressedSize64: uint64(len(data)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
}

// breakLocalHeader overwrites the signature of the local header of the entry
// called name, so the directory still lists it but opening it fails.
func breakLocalHeader(t *testing.T, b []byte, name string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	const localHeaderLen = 30 // the fixed part, before the name and the extra field
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		off, err := f.DataOffset()
		if err != nil {
			t.Fatal(err)
		}
		start := off - localHeaderLen - int64(len(f.Name)) - int64(len(f.Extra))
		out := bytes.Clone(b)
		out[start] = 'X'
		return out
	}
	t.Fatalf("no entry %q", name)
	return nil
}

func TestCharacterizeNameImages(t *testing.T) {
	t.Parallel()
	good := []byte(`{"images":[{"id":"x","name":"Mapa\nantigo.png","file":"images/1"},{"name":"Outra","file":"images/2"}]}`)
	ratio := pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_COMPRESSION_RATIO
	tooBig := pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_ENTRY_TOO_BIG
	pkgKind := pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE
	imageKind := pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_IMAGE
	type problem struct {
		kind   pkgv1.PackageProblemKind
		name   string
		reason pkgv1.PackageProblemReason
	}
	cases := []struct {
		name string
		zip  func(t *testing.T) []byte
		want []problem
	}{
		{
			"no index: the entry's name", func(t *testing.T) []byte {
				t.Helper()
				return bombedPackage(t, nil)
			},
			[]problem{{pkgKind, "images/1", ratio}},
		},
		{"a good index: the image's name, made printable", func(t *testing.T) []byte {
			t.Helper()
			return bombedPackage(t, func(zw *zip.Writer) { storedIndex(t, zw, good, crc32.ChecksumIEEE(good)) })
		}, []problem{{imageKind, "Mapa?antigo.png", ratio}}},
		{"an index with unknown fields is still read", func(t *testing.T) []byte {
			t.Helper()
			data := []byte(`{"images":[{"name":"Mapa","file":"images/1","later":true}],"more":1}`)
			return bombedPackage(t, func(zw *zip.Writer) { storedIndex(t, zw, data, crc32.ChecksumIEEE(data)) })
		}, []problem{{imageKind, "Mapa", ratio}}},
		{"an index that is not JSON", func(t *testing.T) []byte {
			t.Helper()
			data := []byte(`{"images":[`)
			return bombedPackage(t, func(zw *zip.Writer) { storedIndex(t, zw, data, crc32.ChecksumIEEE(data)) })
		}, []problem{{pkgKind, "images/1", ratio}}},
		{"an index that fails its checksum", func(t *testing.T) []byte {
			t.Helper()
			return bombedPackage(t, func(zw *zip.Writer) { storedIndex(t, zw, good, crc32.ChecksumIEEE(good)+1) })
		}, []problem{{pkgKind, "images/1", ratio}}},
		{"an index that cannot be opened", func(t *testing.T) []byte {
			t.Helper()
			b := bombedPackage(t, func(zw *zip.Writer) { storedIndex(t, zw, good, crc32.ChecksumIEEE(good)) })
			return breakLocalHeader(t, b, ImagesEntry)
		}, []problem{{pkgKind, "images/1", ratio}}},
		{"an index over the entry limit is not read", func(t *testing.T) []byte {
			t.Helper()
			data := append(bytes.Clone(good), bytes.Repeat([]byte(" "), MaxEntryBytes)...)
			return bombedPackage(t, func(zw *zip.Writer) { storedIndex(t, zw, data, crc32.ChecksumIEEE(data)) })
		}, []problem{{pkgKind, "images/1", ratio}, {pkgKind, ImagesEntry, tooBig}}},
		{"an image the index does not name keeps its entry name", func(t *testing.T) []byte {
			t.Helper()
			data := []byte(`{"images":[{"name":"Outra","file":"images/2"}]}`)
			return bombedPackage(t, func(zw *zip.Writer) { storedIndex(t, zw, data, crc32.ChecksumIEEE(data)) })
		}, []problem{{pkgKind, "images/1", ratio}}},
		{"a long name is cut", func(t *testing.T) []byte {
			t.Helper()
			data := []byte(`{"images":[{"name":"` + strings.Repeat("é", 70) + `","file":"images/1"}]}`)
			return bombedPackage(t, func(zw *zip.Writer) { storedIndex(t, zw, data, crc32.ChecksumIEEE(data)) })
		}, []problem{{imageKind, strings.Repeat("é", 60), ratio}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, problems := openBytes(tc.zip(t))
			if p != nil {
				t.Fatalf("Open() = a package, want problems")
			}
			if len(problems) != len(tc.want) {
				t.Fatalf("problems = %v, want %v", problems, tc.want)
			}
			for i, w := range tc.want {
				got := problems[i]
				if got.GetKind() != w.kind || got.GetName() != w.name || got.GetReason() != w.reason {
					t.Errorf("problem %d = %v, want %+v", i, got, w)
				}
			}
		})
	}
}
