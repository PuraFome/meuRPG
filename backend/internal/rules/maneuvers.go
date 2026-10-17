package rules

import "slices"

// Where a superiority die goes (the `applies` of a superiority_die effect).
const (
	// ManeuverDamage adds the die to the damage of a weapon attack that hit.
	ManeuverDamage = "damage"
	// ManeuverAttack adds the die to an attack roll.
	ManeuverAttack = "attack"
	// ManeuverGrapple adds the die to the Athletics check of a grapple the character starts.
	ManeuverGrapple = "grapple"
	// ManeuverReduceMelee reduces the damage of a melee attack that hit the character
	// by the die plus an ability modifier; it costs the reaction.
	ManeuverReduceMelee = "reduce_melee_damage"
	// ManeuverGoad adds the die to the damage of a weapon attack that hit, like ManeuverDamage, and
	// asks the target for a Wisdom saving throw against the maneuver's DC (8 + proficiency + the
	// modifier of Ability): on a failure it has disadvantage on its attack rolls against creatures
	// other than the attacker until the end of the attacker's next turn (EffectGoaded).
	ManeuverGoad = "goad"
)

// goadSaveAbility is the ability of the target's saving throw against a goad.
const goadSaveAbility = "wis"

// Bounds of a superiority die's sides: a formula that gives anything else is ignored.
const (
	minManeuverSides = 2
	maxManeuverSides = 100
)

// Maneuver is a superiority_die effect of a table option the character has: a die
// from a resource that goes on a roll. The engine is generic; the table names it.
type Maneuver struct {
	// Key is the feature (option) key that gives it, NamePT its name.
	Key, NamePT string
	// Resource is the resource each use spends, one use each.
	Resource string
	// Sides is the die, worked out for the character's level.
	Sides int
	// Applies is one of the Maneuver* constants.
	Applies string
	// Economy is bonus_action or reaction when using it costs one; empty when it does not.
	Economy string
	// Ability is, for ManeuverReduceMelee, the ability whose modifier is added, and for ManeuverGoad
	// the one whose modifier is in SaveDC.
	Ability string
	// SaveDC is the DC of the target's saving throw against a goad (8 + proficiency + the modifier of
	// Ability); 0 for any other maneuver.
	SaveDC int
	// TextPT is the rider the master reads ("o alvo faz um teste de Sabedoria...").
	TextPT string
}

// maneuvers fills Derived.Maneuvers from the superiority_die effects the character
// has. The resource must be one the character has at this level, as for an action.
func (x *deriver) maneuvers() {
	for _, a := range x.active {
		e := a.effect
		if e.Type != "superiority_die" || !x.applies(a) {
			continue
		}
		if !slices.ContainsFunc(x.d.Resources, func(r Resource) bool { return r.Key == e.Resource }) {
			continue
		}
		sides, err := e.value.Int(x.env)
		if err != nil || sides < minManeuverSides || sides > maxManeuverSides {
			x.issue(IssueFormula, "", "Um efeito de %s foi ignorado: o dado não é válido.", x.c.namePT(a.owner))
			continue
		}
		x.d.Maneuvers = append(x.d.Maneuvers, Maneuver{
			Key: a.owner, NamePT: x.c.namePT(a.owner), Resource: e.Resource, Sides: sides,
			Applies: e.Applies, Economy: e.Economy, Ability: e.Ability, TextPT: e.TextPT, SaveDC: x.goadDC(e),
		})
	}
}

// hasManeuver says the feature (an option) has a superiority_die effect that applies: the die
// is spent where it is added to a roll, so a grant_action of the same feature must not
// spend the same resource a second time.
func (x *deriver) hasManeuver(owner string) bool {
	return slices.ContainsFunc(x.active, func(a activeEffect) bool {
		return a.owner == owner && a.effect.Type == "superiority_die" && x.applies(a)
	})
}

// goadDC is the DC of a goad: 8 plus the proficiency bonus and the modifier of the effect's ability;
// 0 for a maneuver that asks no saving throw.
func (x *deriver) goadDC(e *Effect) int {
	if e.Applies != ManeuverGoad {
		return 0
	}
	for _, a := range x.d.Abilities {
		if string(a.Ability) == e.Ability {
			return goadBaseDC + x.d.ProficiencyBonus + a.Modifier
		}
	}
	return 0
}

// goadBaseDC is the 8 of a maneuver's save DC.
const goadBaseDC = 8
