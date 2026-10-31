package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// MonsterCaster is what the combat needs to let a stat block cast its own spells (MR-042): the
// Spellcasting trait (slots, prepared spells, save DC, attack bonus) and the Innate Spellcasting
// trait (at will, N/day), read from the SRD text. Innate spells are granted spells: at will, or
// with a resource that holds the uses a day.
type MonsterCaster struct {
	// Spellcasting is the creature's casting; its Class is the creature's key.
	Spellcasting Spellcasting
	// Slots are the slots of each level, index 0 the 1st level; nil without a Spellcasting trait.
	Slots []int
	// Spells are the prepared spells of the Spellcasting trait, cantrips included.
	Spells []CharacterSpell
	// Granted are the innate spells, and Resources the uses a day they spend.
	Granted   []GrantedSpell
	Resources []Resource
}

var (
	casterAbilityRe = regexp.MustCompile(`(?i)spellcasting ability is (Intelligence|Wisdom|Charisma)`)
	innateLineRe    = regexp.MustCompile(`(?m)^(?:[^\n]*\. )?(At will|(\d+)/day( each)?): (.+)$`)
	parenthesisRe   = regexp.MustCompile(`\s*\([^)]*\)`)
)

var casterAbilities = map[string]Ability{"intelligence": INT, "wisdom": WIS, "charisma": CHA}

// innateSourceSuffix ends the key of the pseudo source of a creature's innate spells.
const innateSourceSuffix = "#innate-spellcasting"

// MonsterCaster reads the casting of a creature; false when it has no Spellcasting or Innate
// Spellcasting trait with a spell the content knows.
func (c *Content) MonsterCaster(key string) (MonsterCaster, bool) {
	m, ok := c.c.monsters[key]
	if !ok {
		return MonsterCaster{}, false
	}
	scores := monsterScores(m)
	var out MonsterCaster
	for _, a := range m.SpecialAbilities {
		switch a.Name {
		case "Spellcasting":
			if sc, ok := c.c.castingTrait(m.ProficiencyBonus, a.Desc); ok {
				c.c.addSlotCasting(&out, key, m.ProficiencyBonus, scores, a.Desc, sc)
			}
		case "Innate Spellcasting":
			c.c.addInnateCasting(&out, key, m.ProficiencyBonus, scores, a.Desc)
		}
	}
	if len(out.Spells) == 0 && len(out.Granted) == 0 {
		return MonsterCaster{}, false
	}
	return out, true
}

// numbersOf are the casting ability, the save DC and the attack bonus a trait's text gives: the numbers
// it lists, and, for what it leaves out, 8 or nothing plus the proficiency bonus and the ability modifier.
func numbersOf(profBonus int, scores map[Ability]int, desc string) (ability Ability, dc, attack int) {
	if m := casterAbilityRe.FindStringSubmatch(desc); m != nil {
		ability = casterAbilities[strings.ToLower(m[1])]
	}
	mod := modifier(scores[ability])
	dc, attack = 8+profBonus+mod, profBonus+mod
	if m := castingDCRe.FindStringSubmatch(desc); m != nil {
		dc, _ = strconv.Atoi(m[1])
		attack, _ = strconv.Atoi(m[2])
	} else if m := innateDCRe.FindStringSubmatch(desc); m != nil {
		dc, _ = strconv.Atoi(m[1])
	}
	return ability, dc, attack
}

var innateDCRe = regexp.MustCompile(`spell save DC (\d+)`)

// addSlotCasting adds the Spellcasting trait: the slots, the prepared spells and the numbers.
func (c *content) addSlotCasting(out *MonsterCaster, key string, profBonus int, scores map[Ability]int, desc string, sc CreatureCasting) {
	ability, dc, attack := numbersOf(profBonus, scores, desc)
	if sc.SaveDC > 0 {
		dc, attack = sc.SaveDC, sc.AttackBonus
	}
	out.Spellcasting = Spellcasting{Class: key, ClassNamePT: c.namePT(key), Ability: ability, SaveDC: dc, AttackBonus: attack}
	out.Slots = sc.Slots[:]
	for _, k := range sc.Spells {
		if e, ok := c.spellEntries[k]; ok {
			out.Spells = append(out.Spells, CharacterSpell{Spell: e, Prepared: true})
		}
	}
}

// addInnateCasting adds the innate spells of the trait as granted spells: "At will" ones at will, "N/day each"
// ones with a resource of N uses a spell, and "N/day" ones with one resource of N uses for the whole list.
func (c *content) addInnateCasting(out *MonsterCaster, key string, profBonus int, scores map[Ability]int, desc string) {
	ability, dc, attack := numbersOf(profBonus, scores, desc)
	source := key + innateSourceSuffix
	for _, line := range innateLineRe.FindAllStringSubmatch(desc, -1) {
		perDay, _ := strconv.Atoi(line[2])
		shared := fmt.Sprintf("innate_%d_day", perDay)
		for _, name := range strings.Split(line[4], ",") {
			name = strings.TrimSpace(parenthesisRe.ReplaceAllString(name, ""))
			entry, ok := c.spellEntries["spell:"+slugOf(name)]
			if !ok {
				continue
			}
			g := GrantedSpell{
				Spell: entry, Source: source, SourcePT: "Conjuração inata", Ability: ability, AttackBonus: attack, SaveDC: dc,
				AtWill: line[1] == "At will",
			}
			if !g.AtWill {
				g.Resource = shared
				if line[3] != "" { // "each": the uses are the spell's own
					g.Resource = fmt.Sprintf("innate_%d_day_%s", perDay, slugOf(name))
				}
				if !hasCasterResource(out.Resources, g.Resource) {
					out.Resources = append(out.Resources, Resource{Key: g.Resource, NamePT: entry.NamePT, Max: perDay, Recharge: RechargeLongRest, Source: source})
				}
			}
			out.Granted = append(out.Granted, g)
		}
	}
	if len(out.Granted) > 0 && out.Spellcasting.Class == "" {
		out.Spellcasting = Spellcasting{Class: key, ClassNamePT: c.namePT(key), Ability: ability, SaveDC: dc, AttackBonus: attack}
	}
}

func hasCasterResource(rs []Resource, key string) bool {
	for _, r := range rs {
		if r.Key == key {
			return true
		}
	}
	return false
}
