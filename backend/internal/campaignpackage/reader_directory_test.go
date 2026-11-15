package campaignpackage

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
)

// rawDirectory builds a zip that is nothing but directory records, one byte
// of name each, followed by an end record that claims claimed entries.
func rawDirectory(records int, claimed uint16) []byte {
	var buf bytes.Buffer
	for range records {
		rec := make([]byte, 46)
		binary.LittleEndian.PutUint32(rec, 0x02014b50)
		binary.LittleEndian.PutUint16(rec[28:], 1) // name length
		buf.Write(rec)
		buf.WriteByte('a')
	}
	dirSize := buf.Len()
	end := make([]byte, eocdSize)
	binary.LittleEndian.PutUint32(end, eocdSignature)
	binary.LittleEndian.PutUint16(end[8:], claimed)
	binary.LittleEndian.PutUint16(end[10:], claimed)
	binary.LittleEndian.PutUint32(end[12:], uint32(dirSize)) //nolint:gosec // G115: a few MB
	binary.LittleEndian.PutUint32(end[16:], 0)
	buf.Write(end)
	return buf.Bytes()
}

// zipOf writes a zip of n empty entries named name(i).
func zipOf(t *testing.T, n int, name func(i int) string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := range n {
		if _, err := zw.Create(name(i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestADirectoryWhoseCountWrapsIsRefusedBeforeItIsRead(t *testing.T) {
	// Not parallel: AllocsPerRun counts every allocation of the process.
	b := rawDirectory(65536*2+5, 5)
	var problems []*pkgv1.PackageProblem
	allocs := testing.AllocsPerRun(1, func() {
		_, problems = openBytes(b)
	})
	if got := onlyReason(t, problems); got != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_TOO_MANY_ENTRIES {
		t.Fatalf("reason = %v", got)
	}
	// Reading the directory would allocate a File for each of the 131077
	// records; the refusal allocates a handful of buffers.
	if allocs > 100 {
		t.Fatalf("allocations = %v, want the directory left unread", allocs)
	}
}

func TestADirectoryThatDoesNotEndAtTheEndRecordIsNotAPackage(t *testing.T) {
	t.Parallel()
	b := rawDirectory(5, 5)
	binary.LittleEndian.PutUint32(b[len(b)-eocdSize+12:], 1) // a size that lies
	_, problems := openBytes(b)
	if got := onlyReason(t, problems); got != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_A_PACKAGE {
		t.Fatalf("reason = %v", got)
	}
}

func TestTwoThousandEntriesPassTheDirectoryCheckAndOneMoreDoesNot(t *testing.T) {
	t.Parallel()
	// The longest names an entry may have: the biggest directory a package
	// within the limit can have.
	long := func(i int) string {
		return fmt.Sprintf("%s/%d", strings.Repeat("a", maxNameLength-8), i)
	}
	for _, c := range []struct {
		n    int
		want pkgv1.PackageProblemReason
	}{
		{MaxEntries, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNSPECIFIED},
		{MaxEntries + 1, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_TOO_MANY_ENTRIES},
	} {
		b := zipOf(t, c.n, long)
		if got, _ := checkDirectory(bytes.NewReader(b), int64(len(b))); got != c.want {
			t.Fatalf("%d entries: reason = %v, want %v", c.n, got, c.want)
		}
	}
}
