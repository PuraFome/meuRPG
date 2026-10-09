package campaignpackage

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/platform/blob"
)

func TestADocumentLinkGetsTheNewIdOfWhatItNames(t *testing.T) {
	t.Parallel()
	ids := NewIDs()
	newMap, _ := ids.Define("map", "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001")
	newImage, _ := ids.Define("image", "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e002")
	newChar, _ := ids.Define("character", "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e003")
	body := "Vá à [vila](map:6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001), fale com [Lúcia](character:6F1C7A52-3B5E-4C55-9D0B-2A51F0C1E003)\n" +
		"![mapa](image:6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e002) e [lugar nenhum](map:6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e099) [site](https://example.com/map:6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001)"
	got := ids.RewriteDocument(body)
	for _, want := range []string{"(map:" + newMap + ")", "(character:" + newChar + ")", "(image:" + newImage + ")"} {
		if !strings.Contains(got, want) {
			t.Errorf("the document lacks %s:\n%s", want, got)
		}
	}
	if strings.Contains(got, "e001)") && !strings.Contains(got, "https://example.com/map:6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001") {
		t.Errorf("an old id was kept:\n%s", got)
	}
	// A link to something the package lacks names nothing on this server: it gets an id of its own.
	if strings.Contains(got, "e099") || len(got) != len(body) {
		t.Errorf("a dead link kept its old id, or the text changed length:\n%s", got)
	}
	// Plain text that merely looks like a link is left alone.
	if !strings.Contains(got, "https://example.com/map:6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001") {
		t.Errorf("text outside a link was changed:\n%s", got)
	}
}

func TestIDsRefuseWhatIsNotAUUIDAndAnIdTwice(t *testing.T) {
	t.Parallel()
	ids := NewIDs()
	if _, ok := ids.Define("map", "not-a-uuid"); ok {
		t.Error("a bad id was accepted")
	}
	if _, ok := ids.Define("map", "6F1C7A52-3B5E-4C55-9D0B-2A51F0C1E001"); ok {
		t.Error("an id that is not in canonical form was accepted")
	}
	if _, ok := ids.Define("map", "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001"); !ok {
		t.Fatal("a good id was refused")
	}
	if _, ok := ids.Define("map", "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001"); ok {
		t.Error("the same id twice was accepted")
	}
	if _, ok := ids.Define("image", "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001"); !ok {
		t.Error("the same text in another namespace is another thing")
	}
}

func TestAReferenceToWhatThePackageLacksIsReported(t *testing.T) {
	t.Parallel()
	ids := NewIDs()
	ids.Define("image", "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001")
	ids.Ref("image", "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001", 4, "Vila") // fine
	ids.Ref("image", "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e777", 4, "Torre")
	ids.Ref("image", "garbage", 4, "Ponte")
	var pr Problems
	ids.Unresolved(&pr)
	if len(pr.List) != 2 || pr.List[0].GetName() != "Torre" || pr.List[1].GetName() != "Ponte" {
		t.Fatalf("problems = %v", pr.List)
	}
}

// memStore is a blob store in memory, for the tests of the parts' reader.
type memStore struct{ m map[string][]byte }

func (s memStore) Put(_ context.Context, key, ct string, r io.Reader) error {
	b, err := io.ReadAll(r)
	s.m[key] = b
	_ = ct
	return err
}

func (s memStore) Open(_ context.Context, key string) (*blob.Object, error) {
	b, ok := s.m[key]
	if !ok {
		return nil, blob.ErrNotFound
	}
	return &blob.Object{Size: int64(len(b)), Content: bytes.NewReader(b)}, nil
}

func (s memStore) Delete(_ context.Context, key string) error { delete(s.m, key); return nil }

func TestThePartsReadAsOneFileAcrossTheirEdges(t *testing.T) {
	t.Parallel()
	total := int64(2*PartSize + 12345)
	data := make([]byte, total)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
	store := memStore{m: map[string][]byte{}}
	for n := 1; n <= PartCount(total); n++ {
		lo := int64(n-1) * PartSize
		hi := min(lo+PartSize, total)
		store.m[partKey("imp", n)] = data[lo:hi]
	}
	r := newPartsReader(t.Context(), store, "imp", total)
	defer func() { _ = r.Close() }()
	for _, tc := range []struct{ off, n int64 }{
		{0, 10}, {PartSize - 5, 10}, {PartSize - readBlock - 3, readBlock + 9}, {2*PartSize - 1, 2}, {total - 4, 4},
		{PartSize, PartSize + 100}, {0, total},
	} {
		buf := make([]byte, tc.n)
		n, err := r.ReadAt(buf, tc.off)
		if err != nil || int64(n) != tc.n || !bytes.Equal(buf, data[tc.off:tc.off+tc.n]) {
			t.Errorf("ReadAt(%d bytes at %d) = %d, %v", tc.n, tc.off, n, err)
		}
	}
	// Past the end it reads what is there and says EOF, as an io.ReaderAt must.
	buf := make([]byte, 10)
	n, err := r.ReadAt(buf, total-4)
	if n != 4 || err != io.EOF {
		t.Errorf("ReadAt across the end = %d, %v, want 4, EOF", n, err)
	}
	if _, err := r.ReadAt(buf, total); err != io.EOF {
		t.Errorf("ReadAt at the end error = %v, want EOF", err)
	}
	if _, err := r.ReadAt(buf, -1); err == nil {
		t.Error("a negative offset was accepted")
	}
}

func TestPartCountsFollowTheSize(t *testing.T) {
	t.Parallel()
	for total, want := range map[int64]int{1: 1, PartSize: 1, PartSize + 1: 2, 2 * PartSize: 2, MaxPackageBytes: 40} {
		if got := PartCount(total); got != want {
			t.Errorf("PartCount(%d) = %d, want %d", total, got, want)
		}
	}
	if partLength(PartSize+7, 2) != 7 || partLength(PartSize+7, 1) != PartSize || partLength(3, 1) != 3 {
		t.Error("partLength is wrong")
	}
}
