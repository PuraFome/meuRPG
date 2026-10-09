package rules

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// The rarities of a magic item, in order. RarityVaries is not one of
// MagicRarities: it marks a family whose variants have different rarities.
const (
	RarityCommon    = "common"
	RarityUncommon  = "uncommon"
	RarityRare      = "rare"
	RarityVeryRare  = "very_rare"
	RarityLegendary = "legendary"
	RarityArtifact  = "artifact"
	RarityVaries    = "varies"
)

// MagicRarities lists the rarities an item can have, from the commonest.
func MagicRarities() []string {
	return []string{RarityCommon, RarityUncommon, RarityRare, RarityVeryRare, RarityLegendary, RarityArtifact}
}

var magicCategories = []string{"armor", "weapon", "ammunition", "potion", "scroll", "ring", "rod", "staff", "wand", "wondrous-item"}

// magicItemsTotal is the number of entries of the SRD 5.1 snapshot: 239 items
// and families, and 123 variants.
const magicItemsTotal = 362

// MagicItemEntry is a magic item as a list shows it.
type MagicItemEntry struct {
	// Key is "item:bag-of-holding"; Name the SRD's English name; NamePT ours.
	Key, Name, NamePT string
	// Category is "armor", "weapon", "ammunition", "potion", "scroll", "ring",
	// "rod", "staff", "wand" or "wondrous-item".
	Category string
	// Rarity is one of MagicRarities, or RarityVaries for a family whose
	// variants have different rarities.
	Rarity string
	// Attunement says the item requires attunement, and AttunementBy is the
	// SRD's restriction in English ("by a spellcaster"), or "".
	Attunement   bool
	AttunementBy string
	// AttunementByPT is AttunementBy in Portuguese ("por um paladino"), or "".
	AttunementByPT string
	// Standalone says a family is also an item of its own, at its Rarity (the
	// plain Crystal Ball).
	Standalone bool
	// Consumable is true for potions, scrolls and every single-use item (the
	// list in effects/consumables.json): the SRD 5.2.1 value table halves a
	// consumable's value, except a Spell Scroll's. SpellScroll is true for the
	// Spell Scroll and its variants.
	Consumable, SpellScroll bool
	// Variants lists a family's variants (keys, in the SRD's order). VariantOf
	// is the family of a variant, or "".
	Variants  []string
	VariantOf string
}

// IsFamily says the entry is a family: its variants stand for it when an item
// is chosen or rolled.
func (e MagicItemEntry) IsFamily() bool { return len(e.Variants) > 0 }

// MagicItem is a magic item with its text.
type MagicItem struct {
	MagicItemEntry
	// Desc is the SRD's text in English, one paragraph per line.
	Desc []string
	// DescPT is our Portuguese translation (effects/magic_items_pt.json), one
	// paragraph per English one; empty while the item has none (DescPTMissing).
	DescPT []string
}

// DescPTMissing says the item has no Portuguese text yet: the English is all there is.
func (m MagicItem) DescPTMissing() bool { return len(m.DescPT) == 0 }

func (c *content) checkMagicItems() error {
	if len(c.magicItems) != magicItemsTotal {
		return fmt.Errorf("data/magic-items.json: %d items, want %d", len(c.magicItems), magicItemsTotal)
	}
	for _, k := range sortedKeys(c.magicItems) {
		m := c.magicItems[k]
		fail := func(format string, a ...any) error {
			return fmt.Errorf("data/magic-items.json: %s: %s", k, fmt.Sprintf(format, a...))
		}
		if !slices.Contains(magicCategories, m.Category) {
			return fail("%q is not a category", m.Category)
		}
		if m.Rarity != RarityVaries && !slices.Contains(MagicRarities(), m.Rarity) {
			return fail("%q is not a rarity", m.Rarity)
		}
		if len(m.Desc) == 0 {
			return fail("no description")
		}
		if m.AttunementBy != "" && !m.Attunement {
			return fail("an attunement restriction without attunement")
		}
		if m.Standalone && (len(m.Variants) == 0 || m.Rarity == RarityVaries) {
			return fail("standalone, but not a family with a rarity")
		}
		if m.Rarity == RarityVaries && len(m.Variants) == 0 {
			return fail("rarity varies, but no variants")
		}
		if len(m.Variants) > 0 && m.VariantOf != "" {
			return fail("a variant with variants")
		}
		for _, v := range m.Variants {
			vi, ok := c.magicItems[v]
			if !ok || vi.VariantOf != k {
				return fail("variant %q does not point back", v)
			}
			if vi.Rarity == RarityVaries {
				return fail("variant %q has no rarity", v)
			}
		}
		if m.VariantOf != "" {
			f, ok := c.magicItems[m.VariantOf]
			if !ok || !slices.Contains(f.Variants, k) {
				return fail("family %q does not list it", m.VariantOf)
			}
		}
	}
	return nil
}

func (c *content) attunementPT(by string) string {
	if by == "" {
		return ""
	}
	return c.namesPT["attunement:"+slugOf(by)]
}

// consumablesFile is effects/consumables.json: whole categories, and the
// single-use items of the other categories, by key.
type consumablesFile struct {
	Categories []string `json:"categories"`
	Items      []string `json:"items"`
}

// loadMagicItemEffects reads effects/consumables.json and checks the
// Portuguese labels of the attunement restrictions. The file refuses an
// unknown category or item, a repeat, and an item its categories already cover.
func (c *content) loadMagicItemEffects(fsys fs.FS) error {
	const name = "effects/consumables.json"
	var f consumablesFile
	if err := readJSON(fsys, name, &f); err != nil {
		return err
	}
	c.consumables = map[string]bool{}
	cats := map[string]bool{}
	for _, cat := range f.Categories {
		if !slices.Contains(magicCategories, cat) || cats[cat] {
			return fmt.Errorf("%s: %q is not a magic item category, or is repeated", name, cat)
		}
		cats[cat] = true
	}
	for _, k := range sortedKeys(c.magicItems) {
		if cats[c.magicItems[k].Category] {
			c.consumables[k] = true
		}
	}
	for _, k := range f.Items {
		m, ok := c.magicItems[k]
		switch {
		case !ok:
			return fmt.Errorf("%s: %q is not a magic item", name, k)
		case c.consumables[k] && cats[m.Category]:
			return fmt.Errorf("%s: %s is already consumable through its category %s", name, k, m.Category)
		case c.consumables[k]:
			return fmt.Errorf("%s: %s is listed twice", name, k)
		}
		c.consumables[k] = true
	}
	restrictions := map[string]bool{}
	for _, m := range c.magicItems {
		if m.AttunementBy != "" {
			restrictions["attunement:"+slugOf(m.AttunementBy)] = true
		}
	}
	for k := range restrictions {
		if strings.TrimSpace(c.namesPT[k]) == "" {
			return fmt.Errorf("effects/names_pt.json: no Portuguese label %q for an attunement restriction", k)
		}
	}
	for k := range c.namesPT {
		if strings.HasPrefix(k, "attunement:") && !restrictions[k] {
			return fmt.Errorf("effects/names_pt.json: %q is not an attunement restriction of any item", k)
		}
	}
	return nil
}

// MagicItemUnit is what a treasure rolls: one standalone item (Options is
// just its key), or one family at one rarity (Options are the variants that
// have it). The caller rolls a unit, then picks one of its Options, so a family
// with many variants weighs as one item, not as many.
type MagicItemUnit struct {
	// Key is the item, or the family the Options belong to.
	Key string
	// Options are the keys to choose from; never empty.
	Options []string
}

func (c *content) buildMagicItems() {
	for _, k := range sortedKeys(c.magicItems) {
		c.magicItemEntries = append(c.magicItemEntries, c.magicItemEntry(c.magicItems[k]))
	}
	sortPT(c.magicItemEntries, func(e MagicItemEntry) string { return e.NamePT })
	c.magicUnits = map[string][]MagicItemUnit{}
	for _, e := range c.magicItemEntries {
		switch {
		case e.VariantOf != "":
			// Rolled through its family.
		case !e.IsFamily():
			c.magicUnits[e.Rarity] = append(c.magicUnits[e.Rarity], MagicItemUnit{Key: e.Key, Options: []string{e.Key}})
		default:
			if e.Standalone {
				c.magicUnits[e.Rarity] = append(c.magicUnits[e.Rarity], MagicItemUnit{Key: e.Key, Options: []string{e.Key}})
			}
			for _, r := range MagicRarities() {
				var opts []string
				for _, v := range e.Variants {
					if c.magicItems[v].Rarity == r {
						opts = append(opts, v)
					}
				}
				if len(opts) > 0 {
					c.magicUnits[r] = append(c.magicUnits[r], MagicItemUnit{Key: e.Key, Options: opts})
				}
			}
		}
	}
}

func (c *content) magicItemEntry(m *srd51.MagicItem) MagicItemEntry {
	return MagicItemEntry{
		Key: m.Key, Name: m.Name, NamePT: c.namePT(m.Key),
		Category: m.Category, Rarity: m.Rarity,
		Attunement: m.Attunement, AttunementBy: m.AttunementBy, AttunementByPT: c.attunementPT(m.AttunementBy),
		Standalone:  m.Standalone,
		Consumable:  c.consumables[m.Key],
		SpellScroll: m.Key == "item:spell-scroll" || m.VariantOf == "item:spell-scroll",
		Variants:    slices.Clone(m.Variants), VariantOf: m.VariantOf,
	}
}

// MagicItems returns all 362 SRD magic items, families and variants, sorted by
// Portuguese name. The caller gets copies.
func (c *Content) MagicItems() []MagicItemEntry {
	out := slices.Clone(c.c.magicItemEntries)
	for i := range out {
		out[i].Variants = slices.Clone(out[i].Variants)
	}
	return out
}

// MagicItem returns the item with the key ("item:bag-of-holding").
func (c *Content) MagicItem(key string) (MagicItem, bool) {
	m, ok := c.c.magicItems[key]
	if !ok {
		return MagicItem{}, false
	}
	return MagicItem{MagicItemEntry: c.c.magicItemEntry(m), Desc: slices.Clone(m.Desc), DescPT: slices.Clone(c.c.itemTextsPT[key])}, true
}

// MagicItemUnits returns the units a treasure rolls at a rarity, sorted by the
// Portuguese name of their Key. A unit is a standalone item (the plain Crystal
// Ball included), or one family at this rarity with the variants that have it:
// the Ring of Resistance is one rare unit with 10 options, and the Ioun Stone
// is one unit at each rarity it comes in. A family whose variants all share
// the family's rarity is one unit at that rarity. A family is never a unit at
// a rarity its variants do not have, unless it is Standalone. It returns nil
// for "varies" and for any name that is not a rarity. The caller gets copies.
func (c *Content) MagicItemUnits(rarity string) []MagicItemUnit {
	src := c.c.magicUnits[rarity]
	if len(src) == 0 {
		return nil
	}
	out := make([]MagicItemUnit, len(src))
	for i, u := range src {
		out[i] = MagicItemUnit{Key: u.Key, Options: slices.Clone(u.Options)}
	}
	return out
}
