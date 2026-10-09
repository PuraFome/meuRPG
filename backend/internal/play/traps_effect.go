package play

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// What a fired trap does to the creatures it caught (MR-035, Etapa 9, D5): the
// server rolls the trap's attack, its damage and each creature's saving throw, as
// it does for the saves of a spell, and this file is the arithmetic of it: no
// database, no combatant, only dice and numbers. Where the results go (a
// combatant's hit points, a pending damage, the vitals) is combat_traps.go's and
// traps_damage.go's.
//
// The rules the table can read in docs/architecture.md:
//   - The trap's attacks go round the caught creatures in order (attack 1 at the
//     first, 2 at the second, wrapping), each against the creature's armor class
//     with the trap's bonus; a hit rolls the attack's damage, doubled dice on a
//     natural 20, and a natural 1 misses.
//   - Damage that "always lands" is rolled for every creature caught.
//   - The saving throw is asked of every creature caught, or, for a trap that asks it
//     of the creatures it hit, once for each hit (each dart), with the creature's own bonus; on a failure its damage parts and its
//     condition apply, on a pass half the damage (rounded down) or none.
//   - Conditions of the effect go to every creature caught.
//   - Each damage part of each creature is its own roll.

// trapTarget is a creature the trap caught, with what the arithmetic needs of it.
type trapTarget struct {
	// id is the combatant (in a combat) or the character (outside one).
	id string
	// armorClass is its AC for the trap's attacks (0 when the effect has none).
	armorClass int
	// save is its saving throw bonus against the trap's ability, and saveKnown says
	// its sheet has one (a basic-sheet NPC does not: d20 + 0).
	save      int
	saveKnown bool
	// attackMode and saveMode are the modes the trap's attack at this creature and its
	// saving throw roll with (SRD 5.1): the creature's conditions and states, and its
	// Danger Sense against a trap it knew. saveAuto says it fails the save without
	// rolling (stunned, paralyzed, unconscious or petrified: Strength and Dexterity).
	// The sources are the circumstances, kept for the master's and the creature's player's line.
	attackMode, saveMode       combat.RollMode
	attackSources, saveSources []combat.Source
	saveAuto                   bool
}

// trapDamageRoll is one damage part rolled for one creature.
type trapDamageRoll struct {
	count, sides, bonus int // the dice rolled (count is doubled for a critical hit); 0 dice is a flat number
	faces               []int
	damageType          string
	rollTotal           int // the roll
	amount              int // what lands: the roll, or half of it for a creature that saved
	half                bool
	critical            bool
	// criticalMax is what the table's critical rule adds without rolling (the dice's
	// maximum, "máximo mais uma rolagem"), already in rollTotal; maxRule says the
	// table had that rule when it fired.
	criticalMax int
	maxRule     bool
}

// trapAttackRoll is one attack of the trap at a creature.
type trapAttackRoll struct {
	d20, bonus, total int
	armorClass        int
	hit, critical     bool
	// other is the d20 that did not count when the roll had advantage or disadvantage
	// (0 for a normal roll), counted which of the two came first, and mode the mode.
	other, counted int
	mode           combat.RollMode
}

// trapSaveRoll is a creature's saving throw against the trap.
type trapSaveRoll struct {
	d20, bonus, total, dc int
	saved, known          bool
	// other, counted and mode: see trapAttackRoll. auto says the creature failed
	// without rolling.
	other, counted int
	mode           combat.RollMode
	auto           bool
}

// trapOutcome is what the trap did to one creature.
type trapOutcome struct {
	target     trapTarget
	attacks    []trapAttackRoll
	saves      []trapSaveRoll // one, or one for each hit (applies_to HIT)
	damages    []trapDamageRoll
	conditions []string // the keys the trap gives, each once
}

// abilityKeys are the abilities as the sheets name them for a saving throw.
var trapAbilityKeys = map[rulesv1.Ability]string{
	rulesv1.Ability_ABILITY_STRENGTH: "str", rulesv1.Ability_ABILITY_DEXTERITY: "dex", rulesv1.Ability_ABILITY_CONSTITUTION: "con",
	rulesv1.Ability_ABILITY_INTELLIGENCE: "int", rulesv1.Ability_ABILITY_WISDOM: "wis", rulesv1.Ability_ABILITY_CHARISMA: "cha",
}

// trapSaveAbility is the ability key of the trap's saving throw, "" when it has none.
func trapSaveAbility(e *rulesv1.TrapEffect) string {
	if e.GetSave() == nil {
		return ""
	}
	return trapAbilityKeys[e.GetSave().GetAbility()]
}

// trapNeedsAC says the effect makes attack rolls.
func trapNeedsAC(e *rulesv1.TrapEffect) bool { return e.GetAttack() != nil }

// parseTrapDice reads a trap's damage, "2d6" or a flat "1": the dice, and the
// flat number as a bonus on no dice.
func parseTrapDice(s string) (dice.Expr, error) {
	if n, err := strconv.Atoi(s); err == nil {
		if n < 0 || n > 1000 {
			return dice.Expr{}, errors.New("a flat trap damage is 0 to 1000")
		}
		return dice.Expr{Modifier: n}, nil
	}
	return dice.Parse(s)
}

// resolveTrap rolls the trap's effect against the creatures it caught, in the
// order given. rule is the table's critical rule (RN-24), which a trap's critical
// hit follows like any other. d20 rolls one d20 and says its face; rollDice rolls
// damage dice.
func resolveTrap(e *rulesv1.TrapEffect, targets []trapTarget, rule combat.CriticalRule, d20 func() (int, error), rollDice func(dice.Expr) (dice.Result, error)) ([]trapOutcome, error) {
	out := make([]trapOutcome, len(targets))
	for i, t := range targets {
		out[i].target = t
	}
	if len(targets) == 0 {
		return out, nil
	}
	rollDamage := func(d *rulesv1.TrapDamage, critical, half bool) (trapDamageRoll, error) {
		expr, err := parseTrapDice(d.GetDice())
		if err != nil {
			return trapDamageRoll{}, fmt.Errorf("read the trap's damage %q: %w", d.GetDice(), err)
		}
		r := trapDamageRoll{damageType: d.GetDamageTypeKey(), critical: critical, half: half, bonus: expr.Modifier, maxRule: critical && rule == combat.CriticalMaxPlusRoll}
		if expr.Count > 0 {
			count, fixed := combat.CriticalDice(rules.DiceFormula{Count: expr.Count, Sides: expr.Sides}, critical, rule)
			expr.Count = count
			res, err := rollDice(expr)
			if err != nil {
				return trapDamageRoll{}, err
			}
			r.count, r.sides, r.faces, r.criticalMax, r.rollTotal = expr.Count, expr.Sides, res.Faces, fixed, res.Total+fixed
		} else {
			r.rollTotal = expr.Modifier
		}
		r.amount = r.rollTotal
		if half {
			r.amount = combat.HalfDamage(r.rollTotal)
		}
		return r, nil
	}

	// The attacks, round the creatures.
	hits := make([]int, len(targets)) // how many attacks hit each creature
	if a := e.GetAttack(); a != nil {
		for n := range int(a.GetCount()) {
			i := n % len(targets)
			face, other, counted, err := rollModed(d20, targets[i].attackMode)
			if err != nil {
				return nil, err
			}
			r := combat.ResolveAttack(int(a.GetBonus()), targets[i].armorClass, face)
			out[i].attacks = append(out[i].attacks, trapAttackRoll{
				d20: face, bonus: int(a.GetBonus()), total: r.Total, armorClass: targets[i].armorClass, hit: r.Hit, critical: r.Critical,
				other: other, counted: counted, mode: targets[i].attackMode,
			})
			if !r.Hit {
				continue
			}
			hits[i]++
			dmg, err := rollDamage(a.GetDamage(), r.Critical, false)
			if err != nil {
				return nil, err
			}
			out[i].damages = append(out[i].damages, dmg)
		}
	}
	// Damage that always lands, and the conditions of every creature caught.
	for i := range out {
		for _, d := range e.GetDamage() {
			dmg, err := rollDamage(d, false, false)
			if err != nil {
				return nil, err
			}
			out[i].damages = append(out[i].damages, dmg)
		}
		for _, c := range e.GetConditions() {
			out[i].conditions = appendOnce(out[i].conditions, c.GetConditionKey())
		}
	}
	// The saving throw: once for each creature caught, or, when the trap asks it of the
	// creatures it hit, once for each hit (each dart is its own).
	if sv := e.GetSave(); sv != nil {
		for i := range out {
			asks := 1
			if sv.GetAppliesTo() == rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_HIT {
				asks = hits[i]
			}
			for range asks {
				t := out[i].target
				var roll trapSaveRoll
				if t.saveAuto {
					roll = trapSaveRoll{bonus: t.save, dc: int(sv.GetDc()), known: t.saveKnown, auto: true}
				} else {
					face, other, counted, err := rollModed(d20, t.saveMode)
					if err != nil {
						return nil, err
					}
					roll = trapSaveRoll{d20: face, bonus: t.save, total: face + t.save, dc: int(sv.GetDc()), known: t.saveKnown, other: other, counted: counted, mode: t.saveMode}
					roll.saved = combat.SaveSucceeded(roll.total, roll.dc)
				}
				out[i].saves = append(out[i].saves, roll)
				switch {
				case !roll.saved:
					for _, d := range sv.GetOnFail().GetDamage() {
						dmg, err := rollDamage(d, false, false)
						if err != nil {
							return nil, err
						}
						out[i].damages = append(out[i].damages, dmg)
					}
					if c := sv.GetOnFail().GetCondition(); c != nil {
						out[i].conditions = appendOnce(out[i].conditions, c.GetConditionKey())
					}
				case sv.GetOnPass() == rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_HALF:
					for _, d := range sv.GetOnFail().GetDamage() {
						dmg, err := rollDamage(d, false, true)
						if err != nil {
							return nil, err
						}
						out[i].damages = append(out[i].damages, dmg)
					}
				}
			}
		}
	}
	return out, nil
}

func appendOnce(list []string, key string) []string {
	if slices.Contains(list, key) {
		return list
	}
	return append(list, key)
}

// clampInt32 is a number as an int32 for the API's and the events' fields.
func clampInt32(n int) int32 { return clamp32(n, math.MinInt32, math.MaxInt32) }

// rollModed rolls the d20 of a roll with the mode: one die, or two with advantage or
// disadvantage. It returns the die that counts, the other one (0 when there was one) and
// which of the two was rolled first.
func rollModed(d20 func() (int, error), mode combat.RollMode) (face, other, counted int, err error) {
	faces := make([]int, mode.Dice())
	for i := range faces {
		if faces[i], err = d20(); err != nil {
			return 0, 0, 0, err
		}
	}
	counted = mode.Pick(faces)
	face = faces[counted]
	if len(faces) == 2 {
		other = faces[1-counted]
	}
	return face, other, counted, nil
}
