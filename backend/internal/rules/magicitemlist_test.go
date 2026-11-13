package rules

import (
	"slices"
	"testing"
)

func magicRowKeys(l []MagicItemOption) []string {
	var out []string
	for _, o := range l {
		out = append(out, o.Key)
	}
	return out
}

// The picker lists items and families, never a variant on its own, and leaves the +N armor and weapon families to the
// piece's own menu.
func TestListMagicItemsHasItemsAndFamiliesButNoVariantRowsNorPlusGear(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	all := c.ListMagicItems("")
	keys := magicRowKeys(all)
	for _, k := range []string{"item:armor", "item:weapon", "item:ammunition-1", "item:weapon-1"} {
		if slices.Contains(keys, k) {
			t.Errorf("%s is a row of the picker", k)
		}
	}
	for _, k := range []string{"item:ring-of-protection", "item:ammunition", "item:defender"} {
		if !slices.Contains(keys, k) {
			t.Errorf("%s is no row of the picker", k)
		}
	}
	rows := 0
	for _, o := range all {
		rows += 1 + len(o.Options)
		for _, v := range o.Options {
			if len(v.Options) != 0 || v.NamePT == "" {
				t.Errorf("variant %s: %+v", v.Key, v)
			}
		}
	}
	if rows < 300 {
		t.Errorf("%d items and variants, want most of the 362", rows)
	}
}

// The search finds an item by a variant's name too, ignoring case and accents, and a row says what it asks.
func TestListMagicItemsSearchAndRestriction(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	got := magicRowKeys(c.ListMagicItems("PROTEÇÃO"))
	if !slices.Contains(got, "item:ring-of-protection") || len(got) == len(c.ListMagicItems("")) {
		t.Errorf("a search for PROTEÇÃO found %v", got)
	}
	holy := c.ListMagicItems("holy avenger")
	if len(holy) != 1 || !holy[0].Attunement || holy[0].RestrictionPT != "Só para um paladino" {
		t.Errorf("Holy Avenger: %+v", holy)
	}
	if got := c.ListMagicItems("zzzzzz"); len(got) != 0 {
		t.Errorf("nonsense found %v", magicRowKeys(got))
	}
}
