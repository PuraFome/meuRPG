package rules

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

func TestMagicItemsTotalAndFamilies(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	items := c.MagicItems()
	if len(items) != 362 {
		t.Fatalf("%d magic items, want 362", len(items))
	}
	variants, varies := 0, 0
	for _, e := range items {
		if e.VariantOf != "" {
			variants++
		}
		if e.Rarity == RarityVaries {
			varies++
			if !e.IsFamily() {
				t.Errorf("%s: rarity varies without variants", e.Key)
			}
		}
	}
	if variants != 123 || varies != 11 {
		t.Errorf("%d variants and %d families with varying rarity, want 123 and 11", variants, varies)
	}
	// Sorted by Portuguese name.
	for i := 1; i < len(items); i++ {
		if comparePT(items[i-1].NamePT, items[i].NamePT) > 0 {
			t.Errorf("%s before %s: not sorted by Portuguese name", items[i-1].NamePT, items[i].NamePT)
		}
	}
}

func TestMagicItemSpotChecks(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	get := func(key string) MagicItem {
		t.Helper()
		m, ok := c.MagicItem(key)
		if !ok {
			t.Fatalf("%s is missing", key)
		}
		return m
	}
	cases := []struct {
		key, rarity, category string
		attune, consumable    bool
	}{
		{"item:potion-of-healing", RarityVaries, "potion", false, true},
		{"item:potion-of-healing-common", RarityCommon, "potion", false, true},
		{"item:potion-of-healing-supreme", RarityVeryRare, "potion", false, true},
		{"item:spell-scroll-3rd", RarityUncommon, "scroll", false, true},
		{"item:bag-of-holding", RarityUncommon, "wondrous-item", false, false},
		{"item:ring-of-protection", RarityRare, "ring", true, false},
		{"item:weapon-1", RarityUncommon, "weapon", false, false},
		{"item:weapon-3", RarityVeryRare, "weapon", false, false},
		{"item:cloak-of-elvenkind", RarityUncommon, "wondrous-item", true, false},
		{"item:wand-of-magic-missiles", RarityUncommon, "wand", false, false},
		{"item:orb-of-dragonkind", RarityArtifact, "wondrous-item", true, false},
		{"item:hammer-of-thunderbolts", RarityLegendary, "weapon", false, false},
	}
	for _, tc := range cases {
		m := get(tc.key)
		if m.Rarity != tc.rarity || m.Category != tc.category || m.Attunement != tc.attune || m.Consumable != tc.consumable {
			t.Errorf("%s = %s/%s attune %v consumable %v, want %s/%s %v %v", tc.key, m.Rarity, m.Category, m.Attunement, m.Consumable, tc.rarity, tc.category, tc.attune, tc.consumable)
		}
		if len(m.Desc) < 2 {
			t.Errorf("%s: no description text", tc.key)
		}
	}
	if m := get("item:spell-scroll"); !m.SpellScroll || !m.IsFamily() || len(m.Variants) != 10 {
		t.Errorf("Spell Scroll family: %+v", m.MagicItemEntry)
	}
	if m := get("item:spell-scroll-9th"); !m.SpellScroll || m.VariantOf != "item:spell-scroll" {
		t.Errorf("Spell Scroll 9th: %+v", m.MagicItemEntry)
	}
	if m := get("item:bag-of-holding"); m.SpellScroll {
		t.Error("a Bag of Holding is not a spell scroll")
	}
	if m := get("item:weapon"); !slices.Equal(m.Variants, []string{"item:weapon-1", "item:weapon-2", "item:weapon-3"}) {
		t.Errorf("weapon family variants = %v", m.Variants)
	}
	if got := get("item:holy-avenger").AttunementBy; got != "by a paladin" {
		t.Errorf("Holy Avenger attunement by %q", got)
	}
	if got := get("item:amulet-of-health"); got.NamePT != "Amuleto da saúde" || got.Name != "Amulet of Health" {
		t.Errorf("Amulet of Health names: %q, %q", got.Name, got.NamePT)
	}
	if _, ok := c.MagicItem("item:nope"); ok {
		t.Error("an unknown item was found")
	}
}

func TestMagicItemUnits(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	units := map[string][]MagicItemUnit{}
	for _, r := range MagicRarities() {
		units[r] = c.MagicItemUnits(r)
	}
	if len(units[RarityArtifact]) != 1 || len(units[RarityCommon]) == 0 {
		t.Errorf("artifact units: %d", len(units[RarityArtifact]))
	}
	find := func(r, key string) *MagicItemUnit {
		for i, u := range units[r] {
			if u.Key == key {
				return &units[r][i]
			}
		}
		return nil
	}
	// A family with one rarity is one unit with all its variants.
	if u := find(RarityRare, "item:ring-of-resistance"); u == nil || len(u.Options) != 10 {
		t.Errorf("Ring of Resistance at rare: %+v", u)
	}
	// A family that spans rarities is one unit at each.
	if u := find(RarityRare, "item:ioun-stone"); u == nil || len(u.Options) != 4 {
		t.Errorf("Ioun Stone at rare: %+v", u)
	}
	if u := find(RarityVeryRare, "item:ioun-stone"); u == nil || len(u.Options) != 7 {
		t.Errorf("Ioun Stone at very rare: %+v", u)
	}
	if u := find(RarityLegendary, "item:ioun-stone"); u == nil || len(u.Options) != 3 {
		t.Errorf("Ioun Stone at legendary: %+v", u)
	}
	// A family that varies is never a unit by itself, and a plain item is
	// a unit of one.
	if find(RarityUncommon, "item:potion-of-healing") == nil || find(RarityCommon, "item:potion-of-healing") == nil {
		t.Error("the Potion of Healing is a unit at each rarity it comes in")
	}
	if u := find(RarityUncommon, "item:bag-of-holding"); u == nil || !slices.Equal(u.Options, []string{"item:bag-of-holding"}) {
		t.Errorf("Bag of Holding: %+v", u)
	}
	// Every entry is reachable once: families through their rarity groups,
	// and nothing is a unit twice.
	reach := map[string]int{}
	total := 0
	for _, r := range MagicRarities() {
		seen := map[string]bool{}
		for _, u := range units[r] {
			total++
			if len(u.Options) == 0 {
				t.Errorf("%s: a unit of %s with no options", r, u.Key)
			}
			id := u.Key + "/" + strings.Join(u.Options, ",")
			if seen[id] {
				t.Errorf("%s: unit %s twice", r, id)
			}
			seen[id] = true
			for _, o := range u.Options {
				reach[o]++
				if e, _ := c.MagicItem(o); e.Rarity != r {
					t.Errorf("%s in a %s unit, but it is %s", o, r, e.Rarity)
				}
			}
		}
	}
	for _, e := range c.MagicItems() {
		switch {
		case e.IsFamily() && !e.Standalone:
			if reach[e.Key] != 0 {
				t.Errorf("family %s is an option", e.Key)
			}
		case reach[e.Key] != 1:
			t.Errorf("%s is an option %d times, want 1", e.Key, reach[e.Key])
		}
	}
	// 362 entries: 21 families (1 of them also an item), 239 entries that are not
	// variants, so 218 standalone items. Their variants group by
	// (family, rarity); every other entry is a unit of its own.
	groups := 0
	for _, e := range c.MagicItems() {
		if !e.IsFamily() {
			continue
		}
		rs := map[string]bool{}
		for _, v := range e.Variants {
			m, _ := c.MagicItem(v)
			rs[m.Rarity] = true
		}
		groups += len(rs)
	}
	if standalone := 239 - 21 + 1; total != standalone+groups {
		t.Errorf("%d units, want %d standalone + %d (family, rarity) groups", total, standalone, groups)
	}
	if c.MagicItemUnits("varies") != nil || c.MagicItemUnits("mythic") != nil {
		t.Error("varies and unknown names have no units")
	}
	// The caller gets copies.
	if u := c.MagicItemUnits(RarityRare); len(u) > 0 {
		u[0].Options[0] = "changed"
		if c.MagicItemUnits(RarityRare)[0].Options[0] == "changed" {
			t.Error("MagicItemUnits hands out the shared Options")
		}
	}
	if m := c.MagicItems(); len(m) > 0 {
		for i := range m {
			if len(m[i].Variants) > 0 {
				m[i].Variants[0] = "changed"
			}
		}
		for _, e := range c.MagicItems() {
			if len(e.Variants) > 0 && e.Variants[0] == "changed" {
				t.Fatal("MagicItems hands out the shared Variants")
			}
		}
	}
}

// The plain Crystal Ball is "a very rare item" in the SRD, and its three
// variants are legendary: it is rolled at very rare.
func TestCrystalBallIsRolledAtVeryRare(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	m, _ := c.MagicItem("item:crystal-ball")
	if !m.Standalone || !m.IsFamily() || m.Rarity != RarityVeryRare {
		t.Fatalf("Crystal Ball: %+v", m.MagicItemEntry)
	}
	var found bool
	for _, u := range c.MagicItemUnits(RarityVeryRare) {
		if u.Key == "item:crystal-ball" && slices.Equal(u.Options, []string{"item:crystal-ball"}) {
			found = true
		}
	}
	if !found {
		t.Error("the plain Crystal Ball is not a very rare unit")
	}
	for _, u := range c.MagicItemUnits(RarityLegendary) {
		if u.Key == "item:crystal-ball" && len(u.Options) != 3 {
			t.Errorf("the legendary Crystal Balls: %v", u.Options)
		}
	}
	if w, _ := c.MagicItem("item:armor"); w.Standalone {
		t.Error("only the Crystal Ball is a family that is also an item")
	}
}

func TestMagicItemRaritiesAndAttunement(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for key, want := range map[string]string{"item:armor-1": RarityRare, "item:armor-2": RarityVeryRare, "item:armor-3": RarityLegendary} {
		if m, _ := c.MagicItem(key); m.Rarity != want {
			t.Errorf("%s is %s, want %s", key, m.Rarity, want)
		}
	}
	// Attunement of a single property stays in the text (the Giant's Bane).
	if m, _ := c.MagicItem("item:hammer-of-thunderbolts"); m.Attunement {
		t.Error("the Hammer of Thunderbolts needs no attunement as an item")
	}
	// Every restriction has a Portuguese label.
	n := 0
	for _, e := range c.MagicItems() {
		if e.AttunementBy != "" {
			n++
			if e.AttunementByPT == "" || e.AttunementByPT == e.AttunementBy {
				t.Errorf("%s: attunement %q has no Portuguese label", e.Key, e.AttunementBy)
			}
		}
	}
	if n == 0 {
		t.Error("no item has an attunement restriction")
	}
	if m, _ := c.MagicItem("item:holy-avenger"); m.AttunementByPT != "por um paladino" {
		t.Errorf("Holy Avenger: %q", m.AttunementByPT)
	}
}

func TestMagicItemConsumables(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	yes := []string{
		"item:potion-of-healing", "item:potion-of-healing-common", "item:oil-of-sharpness", "item:spell-scroll", "item:spell-scroll-9th",
		"item:ammunition-1", "item:arrow-of-slaying", "item:dust-of-disappearance", "item:dust-of-dryness", "item:dust-of-sneezing-and-choking",
		"item:elemental-gem-fire", "item:bead-of-force", "item:feather-token-tree", "item:universal-solvent", "item:sovereign-glue", "item:restorative-ointment",
	}
	no := []string{"item:bag-of-holding", "item:ring-of-protection", "item:weapon-1", "item:wand-of-magic-missiles", "item:cloak-of-elvenkind", "item:eversmoking-bottle"}
	for _, k := range yes {
		if m, ok := c.MagicItem(k); !ok || !m.Consumable {
			t.Errorf("%s should be consumable", k)
		}
	}
	for _, k := range no {
		if m, ok := c.MagicItem(k); !ok || m.Consumable {
			t.Errorf("%s should not be consumable", k)
		}
	}
	// Every potion and scroll is consumable.
	for _, e := range c.MagicItems() {
		if (e.Category == "potion" || e.Category == "scroll") && !e.Consumable {
			t.Errorf("%s: a potion or scroll that is not consumable", e.Key)
		}
	}
}

func magicContent() *content {
	c := &content{
		magicItems: map[string]*srd51.MagicItem{
			"item:a": {Key: "item:a", Category: "potion", AttunementBy: "by a paladin", Attunement: true},
			"item:b": {Key: "item:b", Category: "ring"},
		},
		namesPT: map[string]string{"attunement:by-a-paladin": "por um paladino"},
	}
	return c
}

func loadConsumables(c *content, body string) error {
	return c.loadMagicItemEffects(fstest.MapFS{"effects/consumables.json": {Data: []byte(body)}})
}

// The loader refuses what the closed schema does not allow.
func TestLoadMagicItemEffectsRefuses(t *testing.T) {
	t.Parallel()
	if err := loadConsumables(magicContent(), `{"categories":["potion"],"items":["item:b"]}`); err != nil {
		t.Fatalf("a good file: %v", err)
	}
	for name, body := range map[string]string{
		"an unknown field":            `{"categories":[],"items":[],"x":1}`,
		"an unknown category":         `{"categories":["gadget"],"items":[]}`,
		"a repeated category":         `{"categories":["potion","potion"],"items":[]}`,
		"an unknown item":             `{"categories":[],"items":["item:nope"]}`,
		"an item twice":               `{"categories":[],"items":["item:b","item:b"]}`,
		"an item its category covers": `{"categories":["potion"],"items":["item:a"]}`,
		"a key with no prefix":        `{"categories":[],"items":["b"]}`,
	} {
		if err := loadConsumables(magicContent(), body); err == nil {
			t.Errorf("%s: the loader accepted it", name)
		}
	}
	good := `{"categories":[],"items":[]}`
	c := magicContent()
	delete(c.namesPT, "attunement:by-a-paladin")
	if err := loadConsumables(c, good); err == nil {
		t.Error("an attunement restriction with no Portuguese label must be refused")
	}
	c = magicContent()
	c.namesPT["attunement:by-a-wizard"] = "por um mago"
	if err := loadConsumables(c, good); err == nil {
		t.Error("a label of a restriction no item has must be refused")
	}
}

func TestCheckMagicItemsRefuses(t *testing.T) {
	t.Parallel()
	c := loadForTest(t).c
	mutate := map[string]func(m map[string]*srd51.MagicItem){
		"an unknown category":     func(m map[string]*srd51.MagicItem) { m["item:bag-of-holding"].Category = "gadget" },
		"an unknown rarity":       func(m map[string]*srd51.MagicItem) { m["item:bag-of-holding"].Rarity = "mythic" },
		"no description":          func(m map[string]*srd51.MagicItem) { m["item:bag-of-holding"].Desc = nil },
		"varies without variants": func(m map[string]*srd51.MagicItem) { m["item:bag-of-holding"].Rarity = RarityVaries },
		"a missing variant": func(m map[string]*srd51.MagicItem) {
			m["item:weapon"].Variants = append(m["item:weapon"].Variants, "item:nope")
		},
		"a variant of nobody":        func(m map[string]*srd51.MagicItem) { m["item:weapon-1"].VariantOf = "item:armor" },
		"a variant that varies":      func(m map[string]*srd51.MagicItem) { m["item:weapon-1"].Rarity = RarityVaries },
		"standalone on a plain item": func(m map[string]*srd51.MagicItem) { m["item:bag-of-holding"].Standalone = true },
		"attunement_by alone":        func(m map[string]*srd51.MagicItem) { m["item:bag-of-holding"].AttunementBy = "by a dwarf" },
		"a variant with variants":    func(m map[string]*srd51.MagicItem) { m["item:weapon-1"].Variants = []string{"item:weapon-2"} },
		"a missing item":             func(m map[string]*srd51.MagicItem) { delete(m, "item:bag-of-holding") },
	}
	for name, f := range mutate {
		cp := map[string]*srd51.MagicItem{}
		for k, v := range c.magicItems {
			x := *v
			x.Variants = slices.Clone(v.Variants)
			cp[k] = &x
		}
		f(cp)
		bad := &content{magicItems: cp}
		if err := bad.checkMagicItems(); err == nil {
			t.Errorf("%s: the check accepted it", name)
		}
	}
	if err := c.checkMagicItems(); err != nil {
		t.Errorf("the real data: %v", err)
	}
}

// TestMagicItemNamesPT: every item has its own Portuguese name (never an
// empty one, never one for an item that does not exist), and two items share
// a name.
func TestMagicItemNamesPT(t *testing.T) {
	t.Parallel()
	c := loadForTest(t).c
	n := 0
	seen := map[string]string{}
	for key, name := range c.namesPT {
		if !strings.HasPrefix(key, "item:") {
			continue
		}
		n++
		if _, ok := c.magicItems[key]; !ok || strings.TrimSpace(name) == "" {
			t.Errorf("%s: a Portuguese name for an item that does not exist, or an empty one", key)
		}
		if other, dup := seen[name]; dup {
			t.Errorf("%s and %s share the name %q", key, other, name)
		}
		seen[name] = key
	}
	if n != 362 {
		t.Errorf("%d item names in names_pt.json, want 362", n)
	}
}
