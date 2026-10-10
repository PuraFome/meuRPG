package rules

import (
	"strings"
	"testing"
	"testing/fstest"
)

// TestTextsPTCoverEverySpellAndItem asks for the Portuguese text of every SRD spell
// (description, "At Higher Levels" and material) and magic item. It fails while a
// translation is missing: the app falls back to the English one for the missing entry,
// and this test is what keeps that fallback from becoming the rule.
func TestTextsPTCoverEverySpellAndItem(t *testing.T) {
	t.Parallel()
	c := loadForTest(t).c
	var missing []string
	for _, k := range sortedKeys(c.spells) {
		if _, ok := c.spellTextsPT[k]; !ok {
			missing = append(missing, k)
		}
	}
	for _, k := range sortedKeys(c.magicItems) {
		if _, ok := c.itemTextsPT[k]; !ok {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		show := missing
		if len(show) > 10 {
			show = show[:10]
		}
		t.Errorf("%d spells and items have no Portuguese text (effects/spells_pt.json, effects/magic_items_pt.json), such as %s", len(missing), strings.Join(show, ", "))
	}
}

func TestTextsPTAreExposed(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	d, ok := c.SpellDetails("spell:fireball")
	if !ok || d.TextPTMissing || d.TextPTOnly {
		t.Fatalf("fireball: ok=%v missing=%v only=%v", ok, d != nil && d.TextPTMissing, d != nil && d.TextPTOnly)
	}
	if len(d.DescriptionPT) != len(d.Description) || len(d.HigherLevelPT) != len(d.HigherLevel) {
		t.Errorf("fireball: %d/%d paragraphs, want %d/%d", len(d.DescriptionPT), len(d.HigherLevelPT), len(d.Description), len(d.HigherLevel))
	}
	if d.Components.MaterialTextPT == "" || d.Components.MaterialText == "" {
		t.Errorf("fireball material: %q / %q", d.Components.MaterialText, d.Components.MaterialTextPT)
	}
	m, ok := c.MagicItem("item:bag-of-holding")
	if !ok || m.DescPTMissing() || len(m.DescPT) != len(m.Desc) {
		t.Errorf("bag of holding: ok=%v, %d paragraphs for %d", ok, len(m.DescPT), len(m.Desc))
	}
}

// TestAMissingTextPTFallsBack loads a content with no Portuguese texts: every SRD
// spell and item has one, so the fallback is only reachable this way.
func TestAMissingTextPTFallsBack(t *testing.T) {
	t.Parallel()
	base := loadForTest(t).c
	fresh := &content{spells: base.spells, magicItems: base.magicItems}
	if err := fresh.loadTextsPT(fstest.MapFS{
		"effects/spells_pt.json":      {Data: []byte(`{}`)},
		"effects/magic_items_pt.json": {Data: []byte(`{}`)},
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok := fresh.spellTextsPT["spell:light"]; ok {
		t.Errorf("light has a Portuguese text in an empty file")
	}
	if _, ok := fresh.itemTextsPT["item:ring-of-protection"]; ok {
		t.Errorf("ring of protection has a Portuguese text in an empty file")
	}
}

func TestTextsPTRefusals(t *testing.T) {
	t.Parallel()
	c := loadForTest(t).c
	ok := `{"spell:fireball":{"desc":["a","b"],"higher_level":["c"],"material":"m"}}`
	cases := []struct {
		name, spells, items, want string
	}{
		{"fine", ok, `{}`, ""},
		{"unknown spell", `{"spell:nope":{"desc":["a"]}}`, `{}`, "not an SRD spell"},
		{"unknown item", `{}`, `{"item:nope":{"desc":["a"]}}`, "not an SRD magic item"},
		{"fewer paragraphs", `{"spell:fireball":{"desc":["a"],"higher_level":["c"],"material":"m"}}`, `{}`, "desc has 1 paragraphs, the English has 2"},
		{"more higher level", `{"spell:fireball":{"desc":["a","b"],"higher_level":["c","d"],"material":"m"}}`, `{}`, "higher_level has 2"},
		{"empty paragraph", `{"spell:fireball":{"desc":["a"," "],"higher_level":["c"],"material":"m"}}`, `{}`, "desc[1] is empty"},
		{"missing material", `{"spell:fireball":{"desc":["a","b"],"higher_level":["c"]}}`, `{}`, "material is empty or missing"},
		{"material without component", `{"spell:prismatic-spray":{"desc":["1","2","3","4","5","6","7","8","9"],"material":"m"}}`, `{}`, "no material component"},
		{"unknown field", `{"spell:fireball":{"desc":["a","b"],"higher_level":["c"],"material":"m","name":"x"}}`, `{}`, "unknown field"},
		{"unknown item field", `{}`, `{"item:bag-of-holding":{"desc":["a","b","c","d"],"higher_level":[]}}`, "unknown field"},
		{"item paragraphs", `{}`, `{"item:bag-of-holding":{"desc":["a"]}}`, "desc has 1 paragraphs, the English has 4"},
		{"item empty", `{}`, `{"item:bag-of-holding":{"desc":["a","","c","d"]}}`, "desc[1] is empty"},
		{"empty where the SRD is empty", `{"spell:scrying":{"desc":["1","2","3","4","5","6","","8","9","10","11","12","13","14","15"],"material":"m"}}`, `{}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fresh := &content{spells: c.spells, magicItems: c.magicItems}
			err := fresh.loadTextsPT(fstest.MapFS{
				"effects/spells_pt.json":      {Data: []byte(tc.spells)},
				"effects/magic_items_pt.json": {Data: []byte(tc.items)},
			})
			switch {
			case tc.want == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
				t.Errorf("error %v, want one with %q", err, tc.want)
			}
		})
	}
}
