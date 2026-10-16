package rules

import (
	"regexp"
	"strconv"
	"strings"
)

// CreatureCasting is the slot-based Spellcasting trait of a stat block (the Mage,
// the Archmage, the Lich, the Priest...), as the combat reads it to know whether
// a monster can cast a reaction spell (Shield, Counterspell). The stat block is
// text, so it is read here: the spells and the slots of each level the trait lists,
// the save DC, the attack bonus and the ability modifier that follows from it.
// Innate Spellcasting (at will, n/day) has no slots and is left out.
type CreatureCasting struct {
	// SaveDC and AttackBonus are the trait's.
	SaveDC, AttackBonus int
	// AbilityMod is the spellcasting ability modifier: the attack bonus minus the
	// creature's proficiency bonus (what a Counterspell ability check adds).
	AbilityMod int
	// Slots are the slots of each spell level, index 0 the 1st level.
	Slots [9]int
	// Spells are the keys of the spells it has prepared, cantrips included.
	Spells []string
}

var (
	castingDCRe    = regexp.MustCompile(`spell save DC (\d+), \+(\d+) to hit with spell attacks`)
	castingSlotsRe = regexp.MustCompile(`(?m)^- (\d+)(?:st|nd|rd|th) level \((\d+) slots?\): (.+)$`)
	castingCantrip = regexp.MustCompile(`(?m)^- Cantrips \(at will\): (.+)$`)
)

// MonsterSpellcasting reads the Spellcasting trait of a creature; false when it
// has none with slots.
func (c *Content) MonsterSpellcasting(key string) (CreatureCasting, bool) {
	m, ok := c.c.monsters[key]
	if !ok {
		return CreatureCasting{}, false
	}
	for _, a := range m.SpecialAbilities {
		if a.Name != "Spellcasting" {
			continue
		}
		if out, ok := c.c.castingTrait(m.ProficiencyBonus, a.Desc); ok {
			return out, true
		}
	}
	return CreatureCasting{}, false
}

// castingTrait reads a Spellcasting trait's text: its DC and attack bonus,
// the slots of each level and the spells it lists that the content has.
// false when it says neither a DC nor a known spell.
func (c *content) castingTrait(profBonus int, desc string) (CreatureCasting, bool) {
	out := CreatureCasting{}
	if dc := castingDCRe.FindStringSubmatch(desc); dc != nil {
		out.SaveDC, _ = strconv.Atoi(dc[1])
		out.AttackBonus, _ = strconv.Atoi(dc[2])
		out.AbilityMod = out.AttackBonus - profBonus
	}
	var spells []string
	for _, s := range castingSlotsRe.FindAllStringSubmatch(desc, -1) {
		level, _ := strconv.Atoi(s[1])
		n, _ := strconv.Atoi(s[2])
		if level >= 1 && level <= len(out.Slots) {
			out.Slots[level-1] = n
		}
		spells = c.castingSpells(spells, s[3])
	}
	if s := castingCantrip.FindStringSubmatch(desc); s != nil {
		spells = c.castingSpells(spells, s[1])
	}
	if out.SaveDC == 0 && len(spells) == 0 {
		return CreatureCasting{}, false
	}
	out.Spells = spells
	return out, true
}

// castingSpells adds the spells of a comma list ("shield, magic missile*")
// that the content has.
func (c *content) castingSpells(spells []string, list string) []string {
	for _, name := range strings.Split(list, ",") {
		name = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(name), "*"))
		if name == "" {
			continue
		}
		if k := "spell:" + slugOf(name); c.spells[k] != nil {
			spells = append(spells, k)
		}
	}
	return spells
}
