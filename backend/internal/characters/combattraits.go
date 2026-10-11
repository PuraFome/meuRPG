package characters

import (
	"slices"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The features a combat reads from a sheet while it rolls attacks and damage
// (advantage and disadvantage, the extras of a hit, Rage, the resistances). The
// rules engine does the numbers; this file only reads which features the derived
// sheet has.

// greatWeaponFighting are the keys of the fighting style, which the fighter, the
// paladin and the ranger spell apart.
var greatWeaponFighting = []string{
	"feature:fighter-fighting-style-great-weapon-fighting", "feature:fighting-style-great-weapon-fighting",
}

// packTacticsSuffix ends the key of a monster's Pack Tactics trait
// ("monster:wolf#pack-tactics").
const packTacticsSuffix = "#pack-tactics"

// creatureTypePrefix starts the key of the pseudo feature that carries a monster's
// type ("creature-type:undead"): a combat reads it with Traits.CreatureType (Divine
// Smite does extra damage to an undead or a fiend).
const creatureTypePrefix = "creature-type:"

// withCreatureType adds the SRD type of the monster to its features.
func withCreatureType(content *rules.Content, monsterKey string, features []rules.Feature) []rules.Feature {
	c, ok := content.CreatureByKey(monsterKey)
	if !ok || c.Type == "" {
		return features
	}
	return append(slices.Clone(features), rules.Feature{Key: creatureTypePrefix + strings.ToLower(c.Type), Name: c.Type, Source: monsterKey})
}

// traitsOf reads the combat traits of a derived sheet or stat block.
func traitsOf(content *rules.Content, d rules.Derived) link.Traits {
	t := link.Traits{
		BarbarianLevel: rules.LevelIn(d, "class:barbarian"),
		RogueLevel:     rules.LevelIn(d, "class:rogue"),
		Rage:           rules.HasFeature(d, "feature:rage"),
		RecklessAttack: rules.HasFeature(d, "feature:reckless-attack"),
		DangerSense:    rules.HasFeature(d, "feature:danger-sense"),
		Frenzy:         rules.HasFeature(d, "feature:frenzy"),
		SculptSpells:   rules.HasFeature(d, "feature:sculpt-spells"),
		DivineSmite:    rules.HasFeature(d, "feature:divine-smite"),
		ColossusSlayer: rules.HasFeature(d, "feature:hunters-prey-colossus-slayer"),
		HeavyArmor:     d.ArmorCategory == "heavy",
		HuntersMark: slices.ContainsFunc(d.Spells, func(s rules.CharacterSpell) bool {
			return s.Spell.Key == "spell:hunters-mark"
		}),
	}
	// The armor's "Stealth: Disadvantage" is a hint on the sheet; the Hide roll reads it here.
	t.StealthDisadvantage = slices.ContainsFunc(d.Hints, func(h rules.Hint) bool {
		return h.Target == "skill:stealth" && h.Mode == "disadvantage"
	})
	t.ImprovedDivineSmite = rules.HasFeature(d, "feature:improved-divine-smite")
	t.OpenHand = rules.HasFeature(d, "feature:open-hand-technique")
	t.StunningStrike = rules.HasFeature(d, "feature:stunning-strike")
	if rules.LevelIn(d, "class:monk") > 0 {
		for _, a := range d.Abilities {
			if a.Ability == rules.WIS {
				t.KiSaveDC = 8 + d.ProficiencyBonus + a.Modifier // the ki save DC (SRD 5.1, Monk)
			}
		}
	}
	if rules.HasFeature(d, "feature:sneak-attack") {
		t.SneakAttackDice = combat.SneakAttackDice(t.RogueLevel)
	}
	t.GreatWeaponFighting = slices.ContainsFunc(greatWeaponFighting, func(k string) bool { return rules.HasFeature(d, k) })
	t.PackTactics = slices.ContainsFunc(d.Features, func(f rules.Feature) bool { return strings.HasSuffix(f.Key, packTacticsSuffix) })
	for _, f := range d.Features {
		if typ, ok := strings.CutPrefix(f.Key, creatureTypePrefix); ok {
			t.CreatureType = typ
		}
	}
	for _, r := range content.Resistances(d) {
		t.Resistances = append(t.Resistances, link.Resistance{Source: r.Source, NamePT: r.NamePT, DamageTypes: r.DamageTypes, While: r.While})
	}
	return t
}
