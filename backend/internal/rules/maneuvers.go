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
)

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
	// Ability is, for ManeuverReduceMelee, the ability whose modifier is added.
	Ability string
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
			Applies: e.Applies, Economy: e.Economy, Ability: e.Ability, TextPT: e.TextPT,
		})
	}
}
