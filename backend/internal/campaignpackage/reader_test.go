package campaignpackage

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
)

var when = time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

// build writes a small package with the Writer.
func build(t *testing.T, mutate func(w *Writer)) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := NewWriter(&buf, when)
	if err := w.AddMessage(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_CAMPAIGN, CampaignEntry, &pkgv1.PackageCampaign{Name: "Mirathel"}); err != nil {
		t.Fatal(err)
	}
	if err := w.AddFile(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE, "images/1", strings.NewReader("not really an image")); err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(w)
	}
	if err := w.Close("rev-1", "Mirathel"); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func openBytes(b []byte) (*Package, []*pkgv1.PackageProblem) {
	return Open(bytes.NewReader(b), int64(len(b)))
}

func onlyReason(t *testing.T, problems []*pkgv1.PackageProblem) pkgv1.PackageProblemReason {
	t.Helper()
	if len(problems) != 1 {
		t.Fatalf("problems = %v, want exactly one", problems)
	}
	if problems[0].GetKind() != pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PACKAGE {
		t.Fatalf("kind = %v, want PACKAGE", problems[0].GetKind())
	}
	return problems[0].GetReason()
}

// rewrite builds a zip by hand, with the manifest written by manifest and the
// entries as given, so a test can break one thing at a time.
func rewrite(t *testing.T, entries map[string][]byte, manifest func(m *pkgv1.PackageManifest)) []byte {
	t.Helper()
	m := &pkgv1.PackageManifest{FormatVersion: 1, ExportedAt: nil}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range entries {
		sum := sha256.Sum256(data)
		m.Entries = append(m.Entries, &pkgv1.PackageEntry{
			Path: name, Kind: pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE, Sha256: hex.EncodeToString(sum[:]), Size: int64(len(data)),
		})
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if manifest != nil {
		manifest(m)
	}
	mj, err := protojson.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	fw, err := zw.Create(ManifestName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(mj); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestAWrittenPackageOpensWithItsEntriesAndHashes(t *testing.T) {
	t.Parallel()
	p, problems := openBytes(build(t, nil))
	if len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
	if p.Manifest.GetFormatVersion() != FormatVersion || p.Manifest.GetContentVersion() != "rev-1" || p.Manifest.GetCampaignName() != "Mirathel" {
		t.Fatalf("manifest = %v", p.Manifest)
	}
	var c pkgv1.PackageCampaign
	if reason, ok := p.ReadMessage(CampaignEntry, &c); !ok || c.GetName() != "Mirathel" {
		t.Fatalf("ReadMessage = %v, %v, %v", reason, ok, &c)
	}
	if got, _ := p.ReadBytes("images/1"); string(got) != "not really an image" {
		t.Fatalf("image bytes = %q", got)
	}
	if n := len(p.Entries(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE)); n != 1 {
		t.Fatalf("image entries = %d", n)
	}
}

func TestAPackageOfANewerFormatIsRefusedAsNewerEvenWithFieldsWeDoNotKnow(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, _ := zw.Create(ManifestName)
	_, _ = fw.Write([]byte(`{"format_version": 2, "a_field_of_version_two": true, "entries": []}`))
	_ = zw.Close()
	_, problems := openBytes(buf.Bytes())
	if got := onlyReason(t, problems); got != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NEWER_VERSION {
		t.Fatalf("reason = %v", got)
	}
}

func TestAManifestWithAnUnknownFieldIsRefused(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, _ := zw.Create(ManifestName)
	_, _ = fw.Write([]byte(`{"format_version": 1, "entries": [], "surprise": 1}`))
	_ = zw.Close()
	_, problems := openBytes(buf.Bytes())
	if got := onlyReason(t, problems); got != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNKNOWN_FIELD {
		t.Fatalf("reason = %v", got)
	}
}

func TestAnEntryWithAnUnknownFieldIsRefusedWhenItIsRead(t *testing.T) {
	t.Parallel()
	body := []byte(`{"name": "Mirathel", "owner_email": "someone@example.com"}`)
	b := rewrite(t, map[string][]byte{CampaignEntry: body}, nil)
	p, problems := openBytes(b)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	var c pkgv1.PackageCampaign
	if reason, ok := p.ReadMessage(CampaignEntry, &c); ok || reason != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNKNOWN_FIELD {
		t.Fatalf("ReadMessage = %v, %v", reason, ok)
	}
}

func TestAnEntryChangedAfterTheManifestIsRefusedByHash(t *testing.T) {
	t.Parallel()
	b := rewrite(t, map[string][]byte{"images/1": []byte("original")}, func(m *pkgv1.PackageManifest) {
		m.Entries[0].Sha256 = strings.Repeat("0", 64)
	})
	_, problems := openBytes(b)
	if got := onlyReason(t, problems); got != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_HASH_MISMATCH {
		t.Fatalf("reason = %v", got)
	}
}

func TestADifferentSizeThanTheManifestSaysIsAMismatch(t *testing.T) {
	t.Parallel()
	b := rewrite(t, map[string][]byte{"images/1": []byte("original")}, func(m *pkgv1.PackageManifest) { m.Entries[0].Size = 3 })
	_, problems := openBytes(b)
	if got := onlyReason(t, problems); got != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_HASH_MISMATCH {
		t.Fatalf("reason = %v", got)
	}
}

func TestAnEntryTheManifestNamesButTheZipLacksIsMissing(t *testing.T) {
	t.Parallel()
	b := rewrite(t, map[string][]byte{"images/1": []byte("x")}, func(m *pkgv1.PackageManifest) {
		m.Entries = append(m.Entries, &pkgv1.PackageEntry{Path: "images/2", Kind: pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE, Sha256: strings.Repeat("0", 64), Size: 1})
	})
	_, problems := openBytes(b)
	if got := onlyReason(t, problems); got != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_MISSING_ENTRY {
		t.Fatalf("reason = %v", got)
	}
}

func TestAnEntryTheManifestDoesNotListIsRefused(t *testing.T) {
	t.Parallel()
	b := rewrite(t, map[string][]byte{"images/1": []byte("x"), "images/2": []byte("y")}, func(m *pkgv1.PackageManifest) {
		m.Entries = m.Entries[:1]
	})
	_, problems := openBytes(b)
	if got := onlyReason(t, problems); got != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNEXPECTED_ENTRY {
		t.Fatalf("reason = %v", got)
	}
}

func TestAnEntryNameThatClimbsOrHidesIsRefusedWhateverTheManifestSays(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"../evil", "images/../../evil", "/etc/passwd", "images//1", "Images/1", `images\1`, ".hidden", "images/1/", "a..b"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			fw, err := zw.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = fw.Write([]byte("x"))
			_ = zw.Close()
			_, problems := openBytes(buf.Bytes())
			if got := onlyReason(t, problems); got != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_BAD_ENTRY_NAME {
				t.Fatalf("reason = %v", got)
			}
		})
	}
}

func TestAManifestEntryWithAClimbingPathIsRefused(t *testing.T) {
	t.Parallel()
	b := rewrite(t, map[string][]byte{"images/1": []byte("x")}, func(m *pkgv1.PackageManifest) { m.Entries[0].Path = "../images/1" })
	_, problems := openBytes(b)
	if got := onlyReason(t, problems); got != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_BAD_ENTRY_NAME {
		t.Fatalf("reason = %v", got)
	}
}

func TestAnEntryOverTenMebibytesIsRefused(t *testing.T) {
	t.Parallel()
	b := rewrite(t, map[string][]byte{"images/1": bytes.Repeat([]byte{7}, MaxEntryBytes+1)}, nil)
	_, problems := openBytes(b)
	if got := onlyReason(t, problems); got != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_ENTRY_TOO_BIG {
		t.Fatalf("reason = %v", got)
	}
}

func TestAnEntryOfExactlyTenMebibytesIsAccepted(t *testing.T) {
	t.Parallel()
	b := rewrite(t, map[string][]byte{"images/1": bytes.Repeat([]byte{7}, MaxEntryBytes)}, nil)
	// Stored, so no compression ratio is involved.
	if _, problems := openBytes(b); len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
}

func TestACompressionBombIsRefusedByTheRatio(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, _ := zw.CreateHeader(&zip.FileHeader{Name: "images/1", Method: zip.Deflate})
	_, _ = fw.Write(make([]byte, MaxEntryBytes)) // 10 MiB of zeros: about 10 KiB compressed
	_ = zw.Close()
	_, problems := openBytes(buf.Bytes())
	if got := onlyReason(t, problems); got != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_COMPRESSION_RATIO {
		t.Fatalf("reason = %v", got)
	}
}

func TestAHeaderThatLiesAboutTheSizeCannotMakeUsReadAnEntryPastIt(t *testing.T) {
	t.Parallel()
	// The manifest and header say 4 bytes; the hash is the hash of "tiny" and
	// the stored data is longer. The reader stops at the declared size, so it
	// reads "tiny" and the hash matches only if the data was cut: the size
	// check on the directory catches the real length first.
	b := rewrite(t, map[string][]byte{"images/1": []byte("tiny and then a lot more")}, func(m *pkgv1.PackageManifest) {
		sum := sha256.Sum256([]byte("tiny"))
		m.Entries[0].Sha256, m.Entries[0].Size = hex.EncodeToString(sum[:]), 4
	})
	_, problems := openBytes(b)
	if got := onlyReason(t, problems); got != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_HASH_MISMATCH {
		t.Fatalf("reason = %v", got)
	}
}

func TestMoreThanTwoThousandEntriesAreRefusedBeforeTheDirectoryIsBuilt(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := range MaxEntries + 1 {
		fw, _ := zw.Create(fmt.Sprintf("images/%d", i))
		_, _ = fw.Write([]byte{1})
	}
	_ = zw.Close()
	_, problems := openBytes(buf.Bytes())
	if got := onlyReason(t, problems); got != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_TOO_MANY_ENTRIES {
		t.Fatalf("reason = %v", got)
	}
}

func TestAFileThatIsNotAZipIsNotAPackage(t *testing.T) {
	t.Parallel()
	_, problems := openBytes([]byte("this is just text, not a zip"))
	if got := onlyReason(t, problems); got != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_A_PACKAGE {
		t.Fatalf("reason = %v", got)
	}
}

func TestAZipWithoutAManifestIsNotAPackage(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, _ := zw.Create("images/1")
	_, _ = fw.Write([]byte("x"))
	_ = zw.Close()
	_, problems := openBytes(buf.Bytes())
	if got := onlyReason(t, problems); got != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_A_PACKAGE {
		t.Fatalf("reason = %v", got)
	}
}

func TestAPackageOverTwoHundredMebibytesIsRefusedWithoutBeingRead(t *testing.T) {
	t.Parallel()
	_, problems := Open(strings.NewReader(""), MaxPackageBytes+1)
	if got := onlyReason(t, problems); got != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_PACKAGE_TOO_BIG {
		t.Fatalf("reason = %v", got)
	}
}

func TestTheWriterRefusesToGoPastTheLimits(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := NewWriter(&buf, when)
	if err := w.AddFile(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_IMAGE_FILE, "images/1", bytes.NewReader(make([]byte, MaxEntryBytes+1))); !errors.Is(err, ErrTooBig) {
		t.Fatalf("a file over 10 MiB: err = %v", err)
	}
	if err := w.AddBytes(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_MAP, "maps/1.json", make([]byte, MaxEntryBytes+1), zip.Deflate); !errors.Is(err, ErrTooBig) {
		t.Fatalf("bytes over 10 MiB: err = %v", err)
	}
	if err := w.AddBytes(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_MAP, "../maps.json", []byte("x"), zip.Deflate); err == nil {
		t.Fatal("a name that climbs was accepted")
	}
}

func TestTheWriterRefusesAnEntryTwiceAndTooManyEntries(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := NewWriter(&buf, when)
	if err := w.AddBytes(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_MAP, "maps/1.json", []byte("{}"), zip.Deflate); err != nil {
		t.Fatal(err)
	}
	if err := w.AddBytes(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_MAP, "maps/1.json", []byte("{}"), zip.Deflate); err == nil {
		t.Fatal("the same entry twice was accepted")
	}
	for i := 2; i < MaxEntries; i++ {
		if err := w.AddBytes(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_MAP, fmt.Sprintf("maps/%d.json", i), []byte("{}"), zip.Store); err != nil {
			t.Fatalf("entry %d: %v", i, err)
		}
	}
	if err := w.AddBytes(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_MAP, "maps/last.json", []byte("{}"), zip.Store); !errors.Is(err, ErrTooBig) {
		t.Fatalf("the 2,000th entry besides the manifest: err = %v", err)
	}
}

func TestFileNamesAreSafe(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"Mirathel":              "Mirathel.meurpg.zip",
		"A Maldição de Strahd":  "A_Maldicao_de_Strahd.meurpg.zip",
		"../../etc/hosts":       "etc_hosts.meurpg.zip",
		"  ":                    "campanha.meurpg.zip",
		"Olá\r\nSet-Cookie: x":  "Ola_Set-Cookie_x.meurpg.zip",
		".hidden":               "hidden.meurpg.zip",
		"a/b\\c":                "a_b_c.meurpg.zip",
		"日本語":                   "campanha.meurpg.zip",
		strings.Repeat("a", 90): strings.Repeat("a", 60) + ".meurpg.zip",
	} {
		if got := FileName(in); got != want {
			t.Errorf("FileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEntryNamesFollowTheShape(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{
		"campaign.json": true, "maps/12.json": true, "images/3": true, "manifest.json": true,
		"": false, "/a": false, "a/": false, "a//b": false, "../a": false, "a/../b": false, "A": false, "a b": false, "a\x00": false, ".a": false,
		strings.Repeat("a", 101): false,
	} {
		if got := ValidEntryName(name); got != want {
			t.Errorf("ValidEntryName(%q) = %v, want %v", name, got, want)
		}
	}
}
