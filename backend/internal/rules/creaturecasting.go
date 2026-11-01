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
	castingSlotsRe = regexp.MustCompile(`(?mi)^- (\d+)(?:st|nd|rd|th) level \((\d+) slots?\): (.+)$`)
	castingCantrip = regexp.MustCompile(`(?mi)^- Cantrips \(at will\): (.+)$`)
)

// MonsterSpellcasting is the reaction view of MonsterCaster: the slots, the prepared spells and the numbers of the
// Spellcasting trait; false when the creature has none with slots.
func (c *Content) MonsterSpellcasting(key string) (CreatureCasting, bool) {
	mc, ok := c.MonsterCaster(key)
	if !ok || mc.Slots == nil {
		return CreatureCasting{}, false
	}
	out := CreatureCasting{SaveDC: mc.Spellcasting.SaveDC, AttackBonus: mc.Spellcasting.AttackBonus}
	out.AbilityMod = out.AttackBonus - c.c.monsters[key].ProficiencyBonus
	copy(out.Slots[:], mc.Slots)
	for _, s := range mc.Spells {
		out.Spells = append(out.Spells, s.Spell.Key)
	}
	return out, true
}

// castingTrait reads a Spellcasting trait's lists: the slots of each level and the spells it lists that the content
// has. The numbers are the text reader's (SpellcastingNumbers).
func (c *content) castingTrait(desc string) (slots [9]int, spells []string) {
	for _, s := range castingSlotsRe.FindAllStringSubmatch(desc, -1) {
		level, _ := strconv.Atoi(s[1])
		n, _ := strconv.Atoi(s[2])
		if level >= 1 && level <= len(slots) {
			slots[level-1] = n
		}
		spells = c.castingSpells(spells, s[3])
	}
	if s := castingCantrip.FindStringSubmatch(desc); s != nil {
		spells = c.castingSpells(spells, s[1])
	}
	return slots, spells
}

// castingSpells adds the spells of a comma list ("shield, magic missile*")
// that the content has.
func (c *content) castingSpells(spells []string, list string) []string {
	for _, name := range listedSpellNames(list) {
		if k := "spell:" + slugOf(name); c.spells[k] != nil {
			spells = append(spells, k)
		}
	}
	return spells
}

// listedSpellNames are the names of a comma list of spells ("shield, magic missile*, a (b, c)"): the
// parentheses go first, as their commas are no separator, then the "*" of a spell cast without material components.
func listedSpellNames(list string) []string {
	var names []string
	for _, name := range strings.Split(parenthesisRe.ReplaceAllString(list, ""), ",") {
		if name = strings.Trim(name, "*;. \t"); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// castingListNames are the spell names a Spellcasting trait's text lists: the slot lines and the cantrips.
func castingListNames(desc string) []string {
	var names []string
	for _, s := range castingSlotsRe.FindAllStringSubmatch(desc, -1) {
		names = append(names, listedSpellNames(s[3])...)
	}
	if s := castingCantrip.FindStringSubmatch(desc); s != nil {
		names = append(names, listedSpellNames(s[1])...)
	}
	return names
}
