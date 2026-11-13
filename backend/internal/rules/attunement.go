package rules

import (
	"fmt"
	"slices"
	"strings"
)

// MaxAttunements is the most items a character is attuned to at once (SRD 5.1, attunement).
const MaxAttunements = 3

// Restriction says whether a character meets the restriction of an item's attunement.
type Restriction int

// The restrictions: met (the item has none, the character meets it, or it is a circumstance the server does not check,
// "outdoors at night"), not met (the character is not what the item asks for), and unknown (the sheet does not say,
// an alignment left unset; only the master decides).
const (
	RestrictionMet Restriction = iota
	RestrictionNotMet
	RestrictionUnknown
)

// AttunementFacts is what the sheet says about the character that a restriction reads and Build does not hold.
type AttunementFacts struct {
	// Alignment is the sheet's alignment as the lowercase words of its enum ("lawful_good", "neutral"), or "" when unset.
	Alignment string
}

// restrictionKind is how the text of a restriction is checked.
type restrictionKind int

const (
	kindClasses restrictionKind = iota
	kindSpellcaster
	kindDwarf
	kindGood
	kindEvil
	kindCircumstance
)

// attunementRule is a restriction of the catalog, parsed: its kind and, for kindClasses, the classes it names.
type attunementRule struct {
	kind    restrictionKind
	classes []string
}

// parseAttunementRule reads the SRD's restriction text ("by a paladin", "by a bard, cleric, or druid", "by a
// spellcaster", "by a dwarf", "by a creature of good alignment", "outdoors at night"). ok is false for any other text,
// so a new catalog entry that this table does not cover fails the load.
func parseAttunementRule(by string) (attunementRule, bool) {
	switch by {
	case "by a spellcaster":
		return attunementRule{kind: kindSpellcaster}, true
	case "by a dwarf":
		return attunementRule{kind: kindDwarf}, true
	case "by a creature of good alignment":
		return attunementRule{kind: kindGood}, true
	case "by a creature of evil alignment":
		return attunementRule{kind: kindEvil}, true
	case "outdoors at night":
		return attunementRule{kind: kindCircumstance}, true
	}
	list, ok := strings.CutPrefix(by, "by a ")
	if !ok {
		return attunementRule{}, false
	}
	list = strings.ReplaceAll(list, ", or ", ", ")
	list = strings.ReplaceAll(list, " or ", ", ")
	var classes []string
	for _, name := range strings.Split(list, ", ") {
		if name == "" || strings.ContainsAny(name, " ") {
			return attunementRule{}, false
		}
		classes = append(classes, "class:"+name)
	}
	return attunementRule{kind: kindClasses, classes: classes}, true
}

// checkAttunementRules refuses a catalog whose restriction this table cannot read; the real-catalog test checks the classes.
func (c *content) checkAttunementRules() error {
	for _, k := range sortedKeys(c.magicItems) {
		by := c.magicItems[k].AttunementBy
		if by == "" {
			continue
		}
		if _, ok := parseAttunementRule(by); !ok {
			return fmt.Errorf("data/magic-items.json: %s: no matcher for the attunement restriction %q", k, by)
		}
	}
	return nil
}

// meets says whether the character of b, derived as d, meets the rule.
func (r attunementRule) meets(b Build, d Derived, f AttunementFacts) Restriction {
	switch r.kind {
	case kindCircumstance:
		return RestrictionMet
	case kindSpellcaster:
		return verdict(len(d.Spellcasting) > 0)
	case kindDwarf:
		return verdict(b.Race == "race:dwarf")
	case kindClasses:
		return verdict(slices.ContainsFunc(b.Classes, func(cl ClassLevel) bool { return slices.Contains(r.classes, cl.Class) }))
	}
	if f.Alignment == "" {
		return RestrictionUnknown
	}
	if r.kind == kindGood {
		return verdict(strings.HasSuffix(f.Alignment, "good"))
	}
	return verdict(strings.HasSuffix(f.Alignment, "evil"))
}

func verdict(met bool) Restriction {
	if met {
		return RestrictionMet
	}
	return RestrictionNotMet
}

// AttunementOf says what attuning to the catalog item key asks: whether it asks for attunement, the restriction in
// Portuguese, and whether the character of b, derived as d, meets it. ok is false for a key that is no item of the
// catalog, or is a family (its variants stand for it).
func (c *Content) AttunementOf(key string, b Build, d Derived, f AttunementFacts) (requires bool, byPT string, met Restriction, ok bool) {
	m, found := c.c.magicItems[key]
	if !found || len(m.Variants) > 0 && !m.Standalone {
		return false, "", RestrictionMet, false
	}
	if !m.Attunement {
		return false, "", RestrictionMet, true
	}
	if m.AttunementBy == "" {
		return true, "", RestrictionMet, true
	}
	rule, _ := parseAttunementRule(m.AttunementBy)
	return true, c.c.attunementPT(m.AttunementBy), rule.meets(b, d, f), true
}

// MaxMagicItems is the most catalog magic items a sheet holds besides its +N gear.
const MaxMagicItems = 20

// checkMagicItems checks the sheet's magic items: at most MaxMagicItems, each an item of the catalog that is no
// family, none twice.
func checkMagicItems(b *Build, c *content) error {
	if len(b.MagicItems) > MaxMagicItems {
		return invalid("full.magic_items", "must have at most %d entries", MaxMagicItems)
	}
	seen := make(map[string]bool, len(b.MagicItems))
	for i, it := range b.MagicItems {
		f := fmt.Sprintf("full.magic_items[%d].key", i)
		m, ok := c.magicItems[it.Key]
		switch {
		case !ok || len(m.Variants) > 0 && !m.Standalone:
			return invalid(f, "is not a magic item of the catalog, or is a family: pick one of its variants")
		case seen[it.Key]:
			return invalid(f, "repeats an entry")
		}
		seen[it.Key] = true
	}
	return nil
}

// AttunedCount is how many items the character is attuned to: the +N gear that asks for attunement and the catalog
// magic items, counted when Attuned is set.
func AttunedCount(b Build) int {
	n := 0
	for _, ib := range b.ItemBonuses {
		if ib.Attuned && ib.RequiresAttunement {
			n++
		}
	}
	for _, it := range b.MagicItems {
		if it.Attuned {
			n++
		}
	}
	return n
}

// sameItemBonuses says the bonuses are the same pieces with the same bonus and the same ask for attunement:
// attunement itself moves during play (SetAttunement) and is not part of the sheet's lock.
func sameItemBonuses(a, b []ItemBonus) bool {
	return slices.EqualFunc(a, b, func(x, y ItemBonus) bool {
		return x.Key == y.Key && x.Bonus == y.Bonus && x.RequiresAttunement == y.RequiresAttunement
	})
}

// sameMagicItems says the sheets hold the same items, whatever their attunement.
func sameMagicItems(a, b []SheetMagicItem) bool {
	return slices.EqualFunc(a, b, func(x, y SheetMagicItem) bool { return x.Key == y.Key })
}
