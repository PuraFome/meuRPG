package rules

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
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

// monsterWith is a content with one made-up stat block (proficiency +2, all scores 10) that has the given
// traits, on the SRD's spells.
func monsterWith(t *testing.T, traits ...srd51.MonsterAbility) *Content {
	t.Helper()
	c := *loadForTest(t).c
	c.monsters = map[string]*srd51.Monster{"monster:x": {
		Key: "monster:x", ChallengeRating: "1", ProficiencyBonus: 2, SpecialAbilities: traits,
		Str: 10, Dex: 10, Con: 10, Int: 10, Wis: 10, Cha: 10,
	}}
	return &Content{c: &c}
}

// TestMonsterCastingReactionValuesGolden: what the reaction reads of every SRD caster (save DC, attack bonus,
// ability modifier, slots, spells) and what MonsterCaster reads (level, granted spells).
// A change to these values is a change to what every SRD caster does in a reaction.
func TestMonsterCastingReactionValuesGolden(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	got := map[string]string{}
	for _, e := range c.c.monsterEntries {
		r, rok := c.MonsterSpellcasting(e.Key)
		mc, mok := c.MonsterCaster(e.Key)
		if !rok && !mok {
			continue
		}
		reaction := "none"
		if rok {
			reaction = fmt.Sprintf("%+v", r)
		}
		got[e.Key] = fmt.Sprintf("reaction %s; caster %v level %d granted %d", reaction, mok, mc.Level, len(mc.Granted))
	}
	goldenText(t, "monster-casting-reactions", got)
}

// TestMonsterCasterReadsParenthesesWithCommas: a comma inside parentheses does not split the list.
func TestMonsterCasterReadsParenthesesWithCommas(t *testing.T) {
	t.Parallel()
	c := monsterWith(t,
		srd51.MonsterAbility{Name: "Spellcasting", Desc: "Its spellcasting ability is Intelligence.\n- Cantrips (at will): fire bolt (a, b), light\n- 1st level (2 slots): shield (x, y), magic missile*"},
		srd51.MonsterAbility{Name: "Innate Spellcasting", Desc: "Its spellcasting ability is Charisma.\nAt will: detect magic (a, b), misty step\n"},
	)
	mc, ok := c.MonsterCaster("monster:x")
	if !ok {
		t.Fatal("no caster")
	}
	var keys []string
	for _, s := range mc.Spells {
		keys = append(keys, s.Spell.Key)
	}
	if want := []string{"spell:shield", "spell:magic-missile", "spell:fire-bolt", "spell:light"}; !slices.Equal(keys, want) {
		t.Errorf("prepared = %v, want %v", keys, want)
	}
	var granted []string
	for _, g := range mc.Granted {
		granted = append(granted, g.Spell.Key)
	}
	if want := []string{"spell:detect-magic", "spell:misty-step"}; !slices.Equal(granted, want) {
		t.Errorf("granted = %v, want %v", granted, want)
	}
}

// TestMonsterCasterReadsAtWillInAnyCase: "At will" and the cantrips label are read whatever their case.
func TestMonsterCasterReadsAtWillInAnyCase(t *testing.T) {
	t.Parallel()
	for _, label := range []string{"At will", "at will", "AT WILL"} {
		c := monsterWith(t,
			srd51.MonsterAbility{Name: "Spellcasting", Desc: "Its spellcasting ability is Wisdom.\n- Cantrips (" + label + "): light"},
			srd51.MonsterAbility{Name: "Innate Spellcasting", Desc: "Its spellcasting ability is Wisdom.\n" + label + ": misty step"},
		)
		mc, ok := c.MonsterCaster("monster:x")
		if !ok || len(mc.Spells) != 1 || len(mc.Granted) != 1 || !mc.Granted[0].AtWill || len(mc.Resources) != 0 {
			t.Errorf("%q: caster = %+v (%v), want one cantrip and one at-will spell", label, mc, ok)
		}
	}
}

// TestMonsterCasterNeedsAnAbilityOrADC: a trait that writes neither a casting ability nor a DC gives no caster,
// not one with a DC worked out from an ability that is not there.
func TestMonsterCasterNeedsAnAbilityOrADC(t *testing.T) {
	t.Parallel()
	c := monsterWith(t,
		srd51.MonsterAbility{Name: "Spellcasting", Desc: "- 1st level (2 slots): shield"},
		srd51.MonsterAbility{Name: "Innate Spellcasting", Desc: "At will: misty step"},
	)
	if mc, ok := c.MonsterCaster("monster:x"); ok {
		t.Errorf("caster = %+v, want none", mc)
	}
	if got, ok := c.MonsterSpellcasting("monster:x"); ok {
		t.Errorf("MonsterSpellcasting = %+v, want none", got)
	}
	// With only a DC the attack bonus follows from it.
	c = monsterWith(t, srd51.MonsterAbility{Name: "Innate Spellcasting", Desc: "It has spell save DC 13.\nAt will: misty step"})
	mc, ok := c.MonsterCaster("monster:x")
	if !ok || mc.Spellcasting.SaveDC != 13 || mc.Spellcasting.AttackBonus != 5 {
		t.Errorf("DC only: %+v (%v), want DC 13 and +5", mc.Spellcasting, ok)
	}
}

// TestMonsterProblemReportsAnUnreadableCaster: a spell the content does not have, or a list with no ability nor DC,
// is a problem of the stat block, not a spell that silently never casts.
func TestMonsterProblemReportsAnUnreadableCaster(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		trait srd51.MonsterAbility
		want  string
	}{
		"ok":                    {srd51.MonsterAbility{Name: "Innate Spellcasting", Desc: "Its spellcasting ability is Charisma.\nAt will: misty step (a, b)"}, ""},
		"innate unknown spell":  {srd51.MonsterAbility{Name: "Innate Spellcasting", Desc: "Its spellcasting ability is Charisma.\n3/day each: misty step, no such spell"}, `Innate Spellcasting lists "no such spell", which is not a spell`},
		"slots unknown spell":   {srd51.MonsterAbility{Name: "Spellcasting", Desc: "Its spellcasting ability is Charisma.\n- 1st level (2 slots): shield, no such spell"}, `Spellcasting lists "no such spell", which is not a spell`},
		"cantrip unknown spell": {srd51.MonsterAbility{Name: "Spellcasting", Desc: "Its spellcasting ability is Charisma.\n- Cantrips (at will): nope"}, `Spellcasting lists "nope", which is not a spell`},
		"no ability nor dc":     {srd51.MonsterAbility{Name: "Innate Spellcasting", Desc: "At will: misty step"}, "Innate Spellcasting lists spells but gives no spellcasting ability, save DC or attack bonus"},
		"no frequency":          {srd51.MonsterAbility{Name: "Innate Spellcasting", Desc: "The mephit can innately cast sleep, requiring no components."}, ""},
	}
	for name, tc := range cases {
		c := monsterWith(t, tc.trait)
		if got := c.c.monsterProblem(c.c.monsters["monster:x"]); got != tc.want {
			t.Errorf("%s: problem = %q, want %q", name, got, tc.want)
		}
	}
}

// TestSRDCreaturesCastingIsReadable: every SRD stat block passes the casting check, and every spell an Innate
// Spellcasting trait lists by frequency becomes a granted spell (the innate twin of
// TestMonsterSpellcastingKnowsEveryListedSpell).
func TestSRDCreaturesCastingIsReadable(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	innate := 0
	for _, e := range c.c.monsterEntries {
		m := c.c.monsters[e.Key]
		if problem := c.c.casterProblem(m); problem != "" {
			t.Errorf("%s: %s", e.Key, problem)
		}
		for _, a := range m.SpecialAbilities {
			names := innateListNames(a.Desc)
			if a.Name != "Innate Spellcasting" || len(names) == 0 {
				continue
			}
			innate++
			mc, ok := c.MonsterCaster(e.Key)
			if !ok || len(mc.Granted) != len(names) {
				t.Errorf("%s: %d spells listed, %d granted", e.Key, len(names), len(mc.Granted))
			}
		}
	}
	if innate < 5 {
		t.Errorf("%d SRD creatures with innate lists, want at least 5", innate)
	}
}
