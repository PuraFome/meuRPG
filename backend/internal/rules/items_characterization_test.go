package rules

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// The characterization of the magic items, the treasure tables and the
// creatures' reads: every refusal with its text, and goldens of what the SRD
// content gives.

// itemsJSONText is v as JSON, or the error.
func itemsJSONText(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

// magicItemsFixture is a content with exactly magicItemsTotal items: a family
// of varying rarity with two variants, a standalone family with one, and
// plain rings to fill the count.
func magicItemsFixture() *content {
	items := map[string]*srd51.MagicItem{
		"item:fam":  {Key: "item:fam", Category: "potion", Rarity: RarityVaries, Variants: []string{"item:v1", "item:v2"}, Desc: []string{"x"}},
		"item:v1":   {Key: "item:v1", Category: "potion", Rarity: RarityCommon, VariantOf: "item:fam", Desc: []string{"x"}},
		"item:v2":   {Key: "item:v2", Category: "potion", Rarity: RarityRare, VariantOf: "item:fam", Desc: []string{"x"}},
		"item:solo": {Key: "item:solo", Category: "wondrous-item", Rarity: RarityVeryRare, Standalone: true, Variants: []string{"item:s1"}, Desc: []string{"x"}},
		"item:s1":   {Key: "item:s1", Category: "wondrous-item", Rarity: RarityLegendary, VariantOf: "item:solo", Desc: []string{"x"}, Attunement: true, AttunementBy: "by a wizard"},
	}
	for i := len(items); i < magicItemsTotal; i++ {
		k := fmt.Sprintf("item:r%03d", i)
		items[k] = &srd51.MagicItem{Key: k, Category: "ring", Rarity: RarityUncommon, Desc: []string{"x"}}
	}
	return &content{magicItems: items}
}

func magicItemCases() map[string]func(items map[string]*srd51.MagicItem) {
	return map[string]func(items map[string]*srd51.MagicItem){
		"ok: the fixture":                      func(map[string]*srd51.MagicItem) {},
		"bad: one item fewer":                  func(items map[string]*srd51.MagicItem) { delete(items, "item:r100") },
		"bad: one item more":                   func(items map[string]*srd51.MagicItem) { items["item:zz"] = &srd51.MagicItem{} },
		"bad: an unknown category":             func(items map[string]*srd51.MagicItem) { items["item:r100"].Category = "gem" },
		"bad: an unknown rarity":               func(items map[string]*srd51.MagicItem) { items["item:r100"].Rarity = "mythic" },
		"ok: an artifact":                      func(items map[string]*srd51.MagicItem) { items["item:r100"].Rarity = RarityArtifact },
		"bad: no description":                  func(items map[string]*srd51.MagicItem) { items["item:r100"].Desc = nil },
		"bad: a restriction, no attunement":    func(items map[string]*srd51.MagicItem) { items["item:s1"].Attunement = false },
		"bad: standalone of no family":         func(items map[string]*srd51.MagicItem) { items["item:r100"].Standalone = true },
		"bad: standalone of a varying rarity":  func(items map[string]*srd51.MagicItem) { items["item:fam"].Standalone = true },
		"bad: varies with no variants":         func(items map[string]*srd51.MagicItem) { items["item:r100"].Rarity = RarityVaries },
		"bad: a variant with variants":         func(items map[string]*srd51.MagicItem) { items["item:v1"].Variants = []string{"item:v2"} },
		"bad: a variant that does not exist":   func(items map[string]*srd51.MagicItem) { items["item:fam"].Variants = []string{"item:v1", "item:nope"} },
		"bad: a variant that points elsewhere": func(items map[string]*srd51.MagicItem) { items["item:v2"].VariantOf = "item:solo" },
		"bad: a variant of no rarity":          func(items map[string]*srd51.MagicItem) { items["item:v2"].Rarity = RarityVaries },
		"bad: a family that does not exist":    func(items map[string]*srd51.MagicItem) { items["item:r100"].VariantOf = "item:nope" },
		"bad: a family that does not list it":  func(items map[string]*srd51.MagicItem) { items["item:r100"].VariantOf = "item:fam" },
		"order: category before rarity": func(items map[string]*srd51.MagicItem) {
			items["item:r100"].Category, items["item:r100"].Rarity = "gem", "x"
		},
		"order: rarity before description": func(items map[string]*srd51.MagicItem) {
			items["item:r100"].Rarity, items["item:r100"].Desc = "x", nil
		},
		"order: description before attunement": func(items map[string]*srd51.MagicItem) {
			items["item:s1"].Desc, items["item:s1"].Attunement = nil, false
		},
		"order: attunement before standalone": func(items map[string]*srd51.MagicItem) {
			items["item:r100"].AttunementBy, items["item:r100"].Standalone = "x", true
		},
		"order: standalone before varies": func(items map[string]*srd51.MagicItem) {
			items["item:r100"].Standalone, items["item:r100"].Rarity = true, RarityVaries
		},
		"order: keys in order": func(items map[string]*srd51.MagicItem) {
			items["item:r100"].Desc, items["item:r200"].Desc = nil, nil
		},
		"order: back pointer before the variant's rarity": func(items map[string]*srd51.MagicItem) {
			items["item:v1"].VariantOf, items["item:v1"].Rarity = "item:solo", RarityVaries
		},
	}
}

func TestCheckMagicItemsCharacterization(t *testing.T) {
	t.Parallel()
	got := map[string]string{}
	for name, mutate := range magicItemCases() {
		c := magicItemsFixture()
		mutate(c.magicItems)
		err := c.checkMagicItems()
		if strings.HasPrefix(name, "ok:") != (err == nil) {
			t.Errorf("%s: %v", name, err)
		}
		got[name] = errText(err)
	}
	goldenText(t, "magicitems-refusals", got)
}

func TestLoadMagicItemEffectsCharacterization(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		file  string
		names map[string]string
	}{
		"ok: categories and items":          {`{"categories":["potion"],"items":["item:r100","item:s1"]}`, nil},
		"ok: nothing":                       {`{}`, nil},
		"bad: missing":                      {"<missing>", nil},
		"bad: unknown field":                {`{"x":1}`, nil},
		"bad: an unknown category":          {`{"categories":["gem"]}`, nil},
		"bad: a category twice":             {`{"categories":["potion","potion"]}`, nil},
		"bad: an unknown item":              {`{"items":["item:nope"]}`, nil},
		"bad: an item of a listed category": {`{"categories":["potion"],"items":["item:v1"]}`, nil},
		"bad: an item twice":                {`{"items":["item:r100","item:r100"]}`, nil},
		"bad: no label for a restriction":   {`{}`, map[string]string{"attunement:by-a-wizard": " "}},
		"bad: a label of no restriction":    {`{}`, map[string]string{"attunement:by-a-bard": "x"}},
		"order: categories before items":    {`{"categories":["gem"],"items":["item:nope"]}`, nil},
		"order: items before labels":        {`{"items":["item:nope"]}`, map[string]string{"attunement:by-a-wizard": ""}},
		"order: items in the file's order":  {`{"items":["item:r200","item:nope","item:r200"]}`, nil},
	}
	got := map[string]string{}
	for name, tc := range cases {
		c := magicItemsFixture()
		c.namesPT = map[string]string{"attunement:by-a-wizard": "por um mago"}
		for k, v := range tc.names {
			c.namesPT[k] = v
		}
		fsys := fstest.MapFS{}
		if tc.file != "<missing>" {
			fsys["effects/consumables.json"] = &fstest.MapFile{Data: []byte(tc.file)}
		}
		err := c.loadMagicItemEffects(fsys)
		text := errText(err)
		if err == nil {
			text += " " + itemsJSONText(sortedKeys(c.consumables))
		}
		if strings.HasPrefix(name, "ok:") != (err == nil) {
			t.Errorf("%s: %v", name, err)
		}
		got[name] = text
	}
	goldenText(t, "magicitemeffects-refusals", got)
}

// TestBuildMagicItemsCharacterization freezes the entries and the units of the
// SRD and of the fixture.
func TestBuildMagicItemsCharacterization(t *testing.T) {
	t.Parallel()
	got := map[string]string{}
	srd := loadForTest(t).c
	fixture := magicItemsFixture()
	fixture.namesPT = map[string]string{"item:fam": "Família", "item:solo": "Sozinho"}
	fixture.consumables = map[string]bool{"item:v1": true}
	fixture.buildMagicItems()
	for name, c := range map[string]*content{"srd": srd, "fixture": fixture} {
		got[name+" entries"] = itemsJSONText(c.magicItemEntries)
		for _, r := range append(MagicRarities(), RarityVaries) {
			got[name+" units "+r] = itemsJSONText(c.magicUnits[r])
		}
	}
	goldenText(t, "buildmagicitems", got)
}

// treasureFiles are the two real treasure files.
func treasureFiles(t *testing.T) (values, tables string) {
	t.Helper()
	read := func(name string) string {
		b, err := fs.ReadFile(srd51.Files, "effects/"+name)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	return read("magic_item_values.json"), read("treasure.json")
}

func treasureLoadCases(t *testing.T) map[string][2]string {
	t.Helper()
	values, tables := treasureFiles(t)
	swap := func(s, old, now string) string {
		t.Helper()
		if !strings.Contains(s, old) {
			t.Fatalf("the fixture has no %q", old)
		}
		return strings.Replace(s, old, now, 1)
	}
	v := func(body string) [2]string { return [2]string{body, tables} }
	tb := func(body string) [2]string { return [2]string{values, body} }
	hoardItems := `"items": {"count": "1d3", "chance_pct": 100`
	return map[string][2]string{
		"ok: the real files":              {values, tables},
		"bad: values missing":             {"<missing>", tables},
		"bad: tables missing":             {values, "<missing>"},
		"bad: values an artifact":         v(swap(values, `"legendary": 200000`, `"legendary": 200000, "artifact": 1`)),
		"bad: values a missing rarity":    v(swap(values, `"rare": 4000,`, ``)),
		"bad: values that do not rise":    v(swap(values, `"rare": 4000`, `"rare": 300`)),
		"bad: values equal":               v(swap(values, `"rare": 4000`, `"rare": 400`)),
		"bad: values of no rarity":        v(swap(values, `"legendary": 200000`, `"legendary": 200000, "mythic": 9`)),
		"bad: values no divisor":          v(swap(values, `"consumable_divisor": 2`, `"consumable_divisor": 0`)),
		"bad: values no source":           v(editTreasureJSON(t, values, func(m map[string]any) { m["source"] = " " })),
		"bad: values unknown field":       v(swap(values, `"consumable_divisor": 2`, `"consumable_divisor": 2, "x": 1`)),
		"bad: values before tables":       {swap(values, `"consumable_divisor": 2`, `"consumable_divisor": 0`), "<missing>"},
		"bad: a roll that does not parse": tb(swap(tables, `"3d8*4", "chance_pct": 100`, `"four dice", "chance_pct": 100`)),
		"bad: zero dice":                  tb(swap(tables, `"3d8*4", "chance_pct": 100`, `"0d6", "chance_pct": 100`)),
		"bad: a coin that does not exist": tb(swap(tables, `"coin": "pc"`, `"coin": "pz"`)),
		"bad: a coin twice":               tb(swap(tables, `"coin": "pp"`, `"coin": "pc"`)),
		"bad: a coin chance over 100":     tb(swap(tables, `"chance_pct": 60}`, `"chance_pct": 101}`)),
		"bad: a coin chance of 0":         tb(swap(tables, `"chance_pct": 60}`, `"chance_pct": 0}`)),
		"bad: a pick chance over 100":     tb(swap(tables, `"chance_pct": 70`, `"chance_pct": 101`)),
		"bad: a pick chance of 0":         tb(swap(tables, `"chance_pct": 70`, `"chance_pct": 0`)),
		"bad: a pick count that fails":    tb(swap(tables, `"count": "2d4", "chance_pct": 70`, `"count": "x", "chance_pct": 70`)),
		// Finding: a count's min() is never below 0 (it is clamped), so checkPick's
		// "count.min() < 0" never refuses: a count that may roll below 0 passes.
		"ok: a pick count that may roll below 0": tb(swap(tables, `"count": "2d4", "chance_pct": 70`, `"count": "1d4-9", "chance_pct": 70`)),
		"bad: a tier that does not exist":        tb(swap(tables, `{"value_po": 6, "weight": 50}`, `{"value_po": 7, "weight": 50}`)),
		"bad: a tier twice":                      tb(swap(tables, `{"value_po": 6, "weight": 50}`, `{"value_po": 6, "weight": 50}, {"value_po": 6, "weight": 5}`)),
		"bad: a tier with no weight":             tb(swap(tables, `{"value_po": 6, "weight": 50}`, `{"value_po": 6, "weight": 0}`)),
		"bad: gems with rarities":                tb(swap(tables, `"gems": {"count": "2d4", "chance_pct": 70, "tiers": [`, `"gems": {"count": "2d4", "chance_pct": 70, "rarities": [{"rarity": "common", "weight": 1}], "tiers": [`)),
		"bad: items with tiers":                  tb(swap(tables, hoardItems, hoardItems+`, "tiers": [{"value_po": 6, "weight": 1}]`)),
		"bad: an artifact rarity":                tb(swap(tables, `{"rarity": "rare", "weight": 5}`, `{"rarity": "artifact", "weight": 5}`)),
		"bad: the varying rarity":                tb(swap(tables, `{"rarity": "rare", "weight": 5}`, `{"rarity": "varies", "weight": 5}`)),
		"bad: an unknown rarity":                 tb(swap(tables, `{"rarity": "rare", "weight": 5}`, `{"rarity": "mythic", "weight": 5}`)),
		"bad: a rarity repeated":                 tb(swap(tables, `{"rarity": "rare", "weight": 5}`, `{"rarity": "common", "weight": 5}`)),
		"bad: a rarity of no weight":             tb(swap(tables, `{"rarity": "rare", "weight": 5}`, `{"rarity": "rare", "weight": 0}`)),
		"bad: too many gems":                     tb(swap(tables, `"count": "2d4", "chance_pct": 70`, `"count": "5d4", "chance_pct": 70`)),
		"bad: too many items":                    tb(swap(tables, `"count": "1d3", "chance_pct": 100`, `"count": "7d3", "chance_pct": 100`)),
		"bad: a hoard with no gems": tb(editTreasureJSON(t, tables, func(m map[string]any) {
			delete(m["bands"].([]any)[0].(map[string]any)["hoard"].(map[string]any), "gems")
		})),
		"bad: a hoard with no art": tb(editTreasureJSON(t, tables, func(m map[string]any) {
			delete(m["bands"].([]any)[1].(map[string]any)["hoard"].(map[string]any), "art")
		})),
		"bad: a hoard with no items": tb(editTreasureJSON(t, tables, func(m map[string]any) {
			delete(m["bands"].([]any)[2].(map[string]any)["hoard"].(map[string]any), "items")
		})),
		"bad: a name used twice":          tb(swap(tables, `"Quartzo-leitoso"`, `"Quartzo-fumê"`)),
		"bad: a name over 60 characters":  tb(swap(tables, `"Quartzo-fumê"`, `"`+strings.Repeat("x", 61)+`"`)),
		"ok: a name of 60 characters":     tb(swap(tables, `"Quartzo-fumê"`, `"`+strings.Repeat("é", 60)+`"`)),
		"bad: an empty name":              tb(swap(tables, `"Quartzo-fumê"`, `" "`)),
		"bad: a tier of no names":         tb(swap(tables, `"names_pt": ["Quartzo-fumê", "Quartzo-leitoso", "Calcedônia", "Sodalita", "Jaspe-sanguíneo"]`, `"names_pt": []`)),
		"bad: a gem tier of 0":            tb(swap(tables, `{"value_po": 6, "names_pt"`, `{"value_po": 0, "names_pt"`)),
		"bad: an art tier over 10,000 PO": tb(swap(tables, `{"value_po": 6600, "names_pt"`, `{"value_po": 10001, "names_pt"`)),
		"bad: gem tiers out of order":     tb(swap(tables, `{"value_po": 18, "names_pt": ["Ágata-de-fogo"`, `{"value_po": 3, "names_pt": ["Ágata-de-fogo"`)),
		"bad: no gem tiers":               tb(editTreasureJSON(t, tables, func(m map[string]any) { m["gems"] = []any{} })),
		"bad: no art tiers":               tb(editTreasureJSON(t, tables, func(m map[string]any) { delete(m, "art") })),
		"bad: gems in an individual":      tb(swap(tables, `"individual": {"coins": [`, `"individual": {"gems": {"count": "1d4", "chance_pct": 50, "tiers": [{"value_po": 6, "weight": 1}]}, "coins": [`)),
		"bad: an individual with no coins": tb(swap(tables, `{"coin": "pc", "dice": "2d20*3", "chance_pct": 100},
    {"coin": "pp", "dice": "3d8*4", "chance_pct": 100},
    {"coin": "po", "dice": "1d8+1", "chance_pct": 60}`, ``)),
		"bad: no bands":                  tb(editTreasureJSON(t, tables, func(m map[string]any) { m["bands"] = []any{} })),
		"bad: a gap between bands":       tb(swap(tables, `"from": 5`, `"from": 6`)),
		"bad: a band that ends early":    tb(swap(tables, `"from": 1, "to": 4`, `"from": 1, "to": 0`)),
		"bad: bands that stop short":     tb(swap(tables, `"to": 20`, `"to": 19`)),
		"bad: bands that go past 20":     tb(swap(tables, `"to": 20`, `"to": 21`)),
		"bad: an unknown field":          tb(swap(tables, `"from": 1, "to": 4,`, `"from": 1, "to": 4, "x": 1,`)),
		"bad: no source":                 tb(editTreasureJSON(t, tables, func(m map[string]any) { m["source"] = "\t" })),
		"bad: a treasure over the cap":   tb(swap(tables, `"dice": "10d20*320"`, `"dice": "10d20*9999"`)),
		"order: gems before art":         tb(swap(swap(tables, `"Quartzo-fumê"`, `" "`), `{"value_po": 6600, "names_pt"`, `{"value_po": 10001, "names_pt"`)),
		"order: individual before hoard": tb(swap(swap(tables, `"coin": "pc"`, `"coin": "pz"`), `"count": "2d4", "chance_pct": 70`, `"count": "x", "chance_pct": 70`)),
	}
}

// treasureBandsText writes the checked bands, with the picks behind their
// pointers.
func treasureBandsText(bands []treasureBand) string {
	pick := func(p *pickRoll) string {
		if p == nil {
			return "none"
		}
		return fmt.Sprintf("%+v", *p)
	}
	mode := func(m treasureMode) string {
		return fmt.Sprintf("coins %+v gems %s art %s items %s", m.coins, pick(m.gems), pick(m.art), pick(m.items))
	}
	var out []string
	for _, b := range bands {
		out = append(out, fmt.Sprintf("%d-%d individual: %s; hoard: %s", b.from, b.to, mode(b.individual), mode(b.hoard)))
	}
	return strings.Join(out, " | ")
}

// editTreasureJSON decodes a treasure file, lets edit change it and encodes
// it back.
func editTreasureJSON(t *testing.T, s string, edit func(m map[string]any)) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	edit(m)
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestLoadTreasureCharacterization freezes loadTreasure's refusals, and the
// tables a good pair of files leaves.
func TestLoadTreasureCharacterization(t *testing.T) {
	t.Parallel()
	base := loadForTest(t).c
	got := map[string]string{}
	for name, files := range treasureLoadCases(t) {
		c := *base
		c.treasure = treasureTables{}
		fsys := fstest.MapFS{}
		for i, p := range []string{"effects/magic_item_values.json", "effects/treasure.json"} {
			if files[i] != "<missing>" {
				fsys[p] = &fstest.MapFile{Data: []byte(files[i])}
			}
		}
		err := c.loadTreasure(fsys)
		text := errText(err)
		if err == nil {
			text += " " + treasureBandsText(c.treasure.bands)
		}
		if strings.HasPrefix(name, "ok:") != (err == nil) {
			t.Errorf("%s: %v", name, err)
		}
		got[name] = text
	}
	// A rarity with no items to roll.
	empty := *base
	empty.treasure = treasureTables{}
	empty.magicUnits = map[string][]MagicItemUnit{}
	got["bad: a rarity with no items"] = errText(empty.loadTreasure(srd51.Files))
	goldenText(t, "treasure-load-refusals", got)
}

// TestGenerateTreasureCharacterization freezes what GenerateTreasure gives for
// each mode, at each level (and out of range), for a few seeds.
func TestGenerateTreasureCharacterization(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	got := map[string]string{}
	for _, mode := range []string{TreasureIndividual, TreasureHoard, "cache"} {
		for level := MinTreasureLevel - 1; level <= MaxTreasureLevel+1; level++ {
			for _, seed := range []uint64{0, 7, 2209} {
				tr, err := c.GenerateTreasure(mode, level, seed)
				text := errText(err)
				if err == nil {
					tr.ContentVersion = ""
					text = itemsJSONText(tr)
				}
				got[fmt.Sprintf("%s %02d %d", mode, level, seed)] = text
			}
		}
	}
	// A coin roll of 0 (a negative bonus) leaves the coin out.
	zero := *c.c
	zero.treasure.bands = []treasureBand{{from: 1, to: 20, individual: treasureMode{coins: []coinRoll{
		{coin: "pc", dice: treasureDice{count: 1, sides: 2, mult: 1, bonus: -5}, chance: 100},
		{coin: "po", dice: treasureDice{count: 1, sides: 2, mult: 1}, chance: 100},
	}}}}
	tr, err := (&Content{c: &zero}).GenerateTreasure(TreasureIndividual, 3, 1)
	tr.ContentVersion = ""
	got["a coin roll of 0"] = errText(err) + " " + itemsJSONText(tr)
	goldenText(t, "generatetreasure", got)
}

// TestMonsterSpellcastingTextsCharacterization freezes how the Spellcasting
// trait's text is read, on stat blocks written for the test.
func TestMonsterSpellcastingTextsCharacterization(t *testing.T) {
	t.Parallel()
	base := loadForTest(t).c
	texts := map[string]string{
		"dc and slots":     "The mage has spell save DC 14, +6 to hit with spell attacks.\n- Cantrips (at will): fire bolt, light*\n- 1st level (4 slots): shield, magic missile\n- 3rd level (2 slots): counterspell, fireball",
		"slots only":       "Its spellcasting ability is Intelligence.\n- 2nd level (1 slot): misty step, unknown spell, ",
		"dc only":          "It has spell save DC 12, +4 to hit with spell attacks.",
		"nothing":          "It casts nothing.",
		"level 10":         "Its spellcasting ability is Intelligence.\n- 10th level (1 slot): wish",
		"cantrips only":    "Its spellcasting ability is Intelligence.\n- Cantrips (at will): fire bolt",
		"unknown cantrips": "- Cantrips (at will): nope",
		"no ability":       "- 2nd level (1 slot): misty step",
	}
	got := map[string]string{}
	for name, text := range texts {
		c := *base
		c.monsters = map[string]*srd51.Monster{"monster:x": {Key: "monster:x", ProficiencyBonus: 2, SpecialAbilities: []srd51.MonsterAbility{
			{Name: "Innate Spellcasting", Desc: "spell save DC 9, +1 to hit with spell attacks."},
			{Name: "Spellcasting", Desc: text},
		}}}
		cast, ok := (&Content{c: &c}).MonsterSpellcasting("monster:x")
		got[name] = fmt.Sprint(ok, " ", itemsJSONText(cast))
	}
	goldenText(t, "monsterspellcasting-texts", got)
}

// TestListCreaturesCharacterization freezes the keys ListCreatures returns for
// each filter, and its refusals.
func TestListCreaturesCharacterization(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	filters := map[string]CreatureFilter{
		"everything":               {},
		"query":                    {Query: "  LOBO "},
		"query en":                 {Query: "wolf"},
		"type":                     {Type: "dragon"},
		"size":                     {Size: "Tiny"},
		"min cr":                   {MinCR: "20"},
		"max cr":                   {MaxCR: "0"},
		"min and max":              {MinCR: "1/4", MaxCR: "1/2"},
		"same min and max":         {MinCR: "5", MaxCR: "5"},
		"no fly":                   {NoFly: true, Type: "dragon"},
		"no swim":                  {NoSwim: true, Type: "beast", MaxCR: "1/8"},
		"together":                 {Query: "a", Type: "humanoid", Size: "Medium", MinCR: "1", MaxCR: "3", NoFly: true, NoSwim: true},
		"bad max":                  {MaxCR: "1/3"},
		"bad min":                  {MinCR: "x"},
		"min above max":            {MinCR: "2", MaxCR: "1"},
		"bad size":                 {Size: "Colossal"},
		"order: max before min":    {MinCR: "x", MaxCR: "y"},
		"order: range before size": {MinCR: "2", MaxCR: "1", Size: "x"},
	}
	got := map[string]string{}
	for name, f := range filters {
		list, err := c.ListCreatures(f)
		if err != nil {
			got[name] = err.Error()
			continue
		}
		var keys []string
		for _, e := range list {
			keys = append(keys, e.Key)
		}
		got[name] = strings.Join(keys, " ")
	}
	goldenText(t, "listcreatures", got)
}
