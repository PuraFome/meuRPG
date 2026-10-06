package main

import "testing"

func magicRow(index, rarity string, variant bool, variants ...string) magicItemsSource {
	r := magicItemsSource{Index: index, Name: index, Variant: variant, Desc: []string{"Ring, " + rarity + " (requires attunement by a bard)", "Text."}}
	r.EquipmentCategory.Index = "ring"
	r.Rarity.Name = rarity
	for _, v := range variants {
		r.Variants = append(r.Variants, ref{Index: v})
	}
	return r
}

func TestConvertMagicItemRows(t *testing.T) {
	t.Parallel()
	out, err := convertMagicItemRows([]magicItemsSource{
		magicRow("b-2", "Rare", true), magicRow("b", "Varies", false, "b-1", "b-2"), magicRow("b-1", "Uncommon", true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 || out[0].Key != "item:b" || out[1].VariantOf != "item:b" || out[2].VariantOf != "item:b" {
		t.Errorf("families not linked, or not sorted: %+v", out)
	}
	if !out[0].Attunement || out[0].AttunementBy != "by a bard" {
		t.Errorf("attunement: %+v", out[0])
	}
	noDesc := magicRow("a", "Rare", false)
	noDesc.Desc = nil
	badCat := magicRow("a", "Rare", false)
	badCat.EquipmentCategory.Index = "gadget"
	notVariant := magicRow("v", "Rare", false)
	crystal := magicRow("crystal-ball", "Very Rare", false)
	for name, rows := range map[string][]magicItemsSource{
		"an unknown category":                    {badCat},
		"an empty description":                   {noDesc},
		"a repeated index":                       {magicRow("a", "Rare", false), magicRow("a", "Rare", false)},
		"listed as a variant but not one":        {magicRow("f", "Rare", false, "v"), notVariant},
		"a variant that varies":                  {magicRow("f", "Rare", false, "v"), magicRow("v", "Varies", true)},
		"a variant with variants":                {magicRow("v", "Rare", true, "w"), magicRow("w", "Rare", true)},
		"a family that is also an item, no kids": {crystal},
		"a missing variant":                      {magicRow("b", "Varies", false, "b-1")},
		"a variant of nobody":                    {magicRow("b-1", "Rare", true)},
		"two families":                           {magicRow("a", "Rare", false, "v"), magicRow("b", "Rare", false, "v"), magicRow("v", "Rare", true)},
		"an unknown rarity":                      {magicRow("a", "Mythic", false)},
		"varies alone":                           {magicRow("a", "Varies", false)},
	} {
		if _, err := convertMagicItemRows(rows); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestConvertMagicItemRowsKeepsWhatIsRight(t *testing.T) {
	t.Parallel()
	row := magicRow("a", "Rare", false)
	row.Desc = []string{"Ring, rare (requires attunement)", "Text."}
	crystal := magicRow("crystal-ball", "Very Rare", false, "crystal-ball-x")
	out, err := convertMagicItemRows([]magicItemsSource{row, crystal, magicRow("crystal-ball-x", "Legendary", true)})
	if err != nil {
		t.Fatal(err)
	}
	if !out[0].Attunement || out[0].AttunementBy != "" {
		t.Errorf("attunement without a restriction: %+v", out[0])
	}
	if !out[1].Standalone || out[1].Rarity != "very_rare" || out[2].Rarity != "legendary" {
		t.Errorf("the Crystal Ball and its variant: %+v %+v", out[1], out[2])
	}
}
