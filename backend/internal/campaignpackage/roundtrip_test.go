package campaignpackage_test

import (
	"strings"
	"testing"
)

// diffLines names the first lines that differ, for a readable failure.
func diffLines(a, b string) string {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := range max(len(la), len(lb)) {
		var x, y string
		if i < len(la) {
			x = la[i]
		}
		if i < len(lb) {
			y = lb[i]
		}
		if x != y {
			lo := max(i-6, 0)
			return "line " + strings.TrimSpace(strings.Join([]string{itoa(i + 1)}, "")) + "\n--- source\n" + strings.Join(la[lo:min(i+4, len(la))], "\n") + "\n--- imported\n" + strings.Join(lb[lo:min(i+4, len(lb))], "\n")
		}
	}
	return ""
}

func itoa(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{digits[n%10]}, b...)
	}
	return string(b)
}

// A campaign with one of everything goes out as a package and comes back as a
// new campaign that reads the same, field by field, ids aside.
func TestExportThenImportMakesTheSameCampaign(t *testing.T) {
	h := newHarness(t)
	master := h.newUser("Mestre")
	f := h.buildFixture(master)
	zipped := master.export(f.campaign)
	got := master.importPackage("Mirathel.meurpg.zip", zipped)
	if got.GetCampaign().GetId() == f.campaign {
		t.Fatal("the import reused the campaign id")
	}
	before, after := h.dump(master, f.campaign), h.dump(master, got.GetCampaign().GetId())
	if before != after {
		t.Fatalf("the imported campaign differs:\n%s", diffLines(before, after))
	}
	if !strings.Contains(before, "Taverna do Corvo") || !strings.Contains(before, "Gruk") || !strings.Contains(before, "Charada da lareira") {
		t.Fatalf("the fixture is missing what the test is about:\n%.400s", before)
	}
	c := got.GetCounts()
	if c.GetMaps() != 2 || c.GetNpcs() != 2 || c.GetCharacters() != 1 || c.GetScenes() != 1 || c.GetPuzzles() != 3 || c.GetBattlePoints() != 1 || c.GetTreasurePoints() != 1 || c.GetImages() != 3 || c.GetContentEntries() != 4 {
		t.Fatalf("counts = %v", c)
	}
}
