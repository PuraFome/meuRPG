package rules

import (
	"slices"
	"strings"
	"testing"
)

// TestMonsterSpellcastingReadsTheMage: the stat block's Spellcasting trait gives
// the slots by level, the prepared spells and the numbers a Counterspell check
// needs (SRD 5.1, Mage: save DC 14, +6 to hit, Intelligence +3 with proficiency +3).
func TestMonsterSpellcastingReadsTheMage(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	got, ok := c.MonsterSpellcasting("monster:mage")
	if !ok {
		t.Fatal("MonsterSpellcasting(mage) = none")
	}
	if got.SaveDC != 14 || got.AttackBonus != 6 || got.AbilityMod != 3 {
		t.Errorf("mage numbers = DC %d, +%d, mod %d; want 14, 6, 3", got.SaveDC, got.AttackBonus, got.AbilityMod)
	}
	if want := [9]int{4, 3, 3, 3, 1, 0, 0, 0, 0}; got.Slots != want {
		t.Errorf("mage slots = %v, want %v", got.Slots, want)
	}
	for _, k := range []string{"spell:shield", "spell:counterspell", "spell:fireball", "spell:fire-bolt", "spell:mage-armor"} {
		if !slices.Contains(got.Spells, k) {
			t.Errorf("mage spells = %v, want %s among them", got.Spells, k)
		}
	}
}

// TestMonsterSpellcastingIgnoresWhatHasNoSlots: a creature without the trait, and
// innate spellcasting, have none.
func TestMonsterSpellcastingIgnoresWhatHasNoSlots(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, key := range []string{"monster:goblin", "monster:wolf", "monster:not-a-creature"} {
		if got, ok := c.MonsterSpellcasting(key); ok {
			t.Errorf("MonsterSpellcasting(%s) = %+v, want none", key, got)
		}
	}
}

// TestMonsterSpellcastingKnowsEveryListedSpell: each spell a creature's trait
// names becomes a spell of the SRD, so a name the parser could not read is seen
// here, not in a fight. The reaction spells appear where the SRD lists them.
func TestMonsterSpellcastingKnowsEveryListedSpell(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	casters := map[string]bool{}
	for _, e := range c.c.monsterEntries {
		m := c.c.monsters[e.Key]
		for _, a := range m.SpecialAbilities {
			if a.Name != "Spellcasting" || !strings.Contains(a.Desc, "slots") {
				continue
			}
			got, ok := c.MonsterSpellcasting(e.Key)
			if !ok {
				t.Errorf("%s: its Spellcasting trait was not read", e.Key)
				continue
			}
			casters[e.Key] = true
			listed := strings.Count(a.Desc, "\n- ") // one line per spell level and the cantrips
			if lines := len(castingSlotsRe.FindAllString(a.Desc, -1)) + len(castingCantrip.FindAllString(a.Desc, -1)); lines != listed {
				t.Errorf("%s: %d lines of spells, %d read", e.Key, listed, lines)
			}
			names := 0
			for _, l := range append(castingSlotsRe.FindAllStringSubmatch(a.Desc, -1), castingCantrip.FindAllStringSubmatch(a.Desc, -1)...) {
				names += strings.Count(l[len(l)-1], ",") + 1
			}
			if names != len(got.Spells) {
				t.Errorf("%s: %d spell names in the trait, %d are SRD spells: %v", e.Key, names, len(got.Spells), got.Spells)
			}
		}
	}
	for _, want := range []string{"monster:mage", "monster:archmage", "monster:lich"} {
		if !casters[want] {
			t.Errorf("%s is not among the casters read: %v", want, casters)
		}
	}
	lich, _ := c.MonsterSpellcasting("monster:lich")
	if !slices.Contains(lich.Spells, "spell:counterspell") || !slices.Contains(lich.Spells, "spell:shield") {
		t.Errorf("lich spells = %v, want Shield and Counterspell", lich.Spells)
	}
}
