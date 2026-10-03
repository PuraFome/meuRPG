package combat

import (
	"slices"
)

// Spells that read hit points (MR-014, Etapa 8). The kinds are in
// rules/spelleffects.go and effects/spells.json; here is the arithmetic, pure:
// the creatures' current hit points and the rolled dice come in, who is
// affected comes out. Nothing here reads a database or rolls a die.

// HPCreature is one creature a spell may touch.
type HPCreature struct {
	// HP is its current hit points.
	HP int
	// Unconscious says it already carries the Unconscious condition: Sono and
	// Borrifo de Cores skip it.
	Unconscious bool
}

// PoolReason says why a creature of a pool was not affected.
const (
	// PoolAffected: it was affected.
	PoolAffected = ""
	// PoolSkipped: already unconscious, or at 0 hit points: the pool goes past
	// it without spending anything.
	PoolSkipped = "skipped"
	// PoolTooHigh: its hit points are more than what is left of the pool.
	PoolTooHigh = "above_pool"
)

// PoolStep is what the pool did to one creature.
type PoolStep struct {
	// Index is the creature's position in the list given to ResolvePool.
	Index int
	// HP is its hit points before the spell.
	HP int
	// Affected says the spell reached it.
	Affected bool
	// Reason is PoolSkipped or PoolTooHigh for a creature that was not affected.
	Reason string
	// Left is what is left of the pool after this creature.
	Left int
}

// ResolvePool goes through the creatures in ascending order of current hit
// points (a tie keeps the order given) and affects each one whose hit points
// fit in what is left of the total, which then goes down by them (Sono, Borrifo
// de Cores). A creature that is unconscious or at 0 hit points is skipped. The
// result has one step per creature, in the order the pool went through them.
func ResolvePool(total int, creatures []HPCreature) []PoolStep {
	order := make([]int, len(creatures))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return creatures[a].HP - creatures[b].HP })
	left := max(total, 0)
	steps := make([]PoolStep, 0, len(creatures))
	for _, i := range order {
		c := creatures[i]
		step := PoolStep{Index: i, HP: c.HP}
		switch {
		case c.Unconscious || c.HP <= 0:
			step.Reason = PoolSkipped
		case c.HP > left:
			step.Reason = PoolTooHigh
		default:
			step.Affected = true
			left -= c.HP
		}
		step.Left = left
		steps = append(steps, step)
	}
	return steps
}

// ResolveThreshold says whether a creature with hp hit points is affected by a
// spell that needs it at threshold or below (Palavra de Poder: Atordoar at 150,
// Matar at 100): exactly the threshold is still affected.
func ResolveThreshold(threshold, hp int) bool {
	return hp <= threshold
}

// ResolveZeroHP says whether a creature with hp hit points is affected by a
// spell that works only at 0 (Poupar os Moribundos).
func ResolveZeroHP(hp int) bool {
	return hp == 0
}

// FlatHealResult is what a fixed heal did.
type FlatHealResult struct {
	// HP is the hit points after, and Healed how many it regained.
	HP, Healed int
	// Conditions are the creature's conditions after, in their order, and Ended
	// the ones the spell took away.
	Conditions, Ended []string
}

// ResolveFlatHeal heals amount up to the maximum and ends the conditions in
// ends that the creature has (Cura Completa).
func ResolveFlatHeal(amount, hp, maxHP int, conditions, ends []string) FlatHealResult {
	h := ApplyHeal(hp, maxHP, amount)
	out := FlatHealResult{HP: h.HP, Healed: h.Healed, Conditions: []string{}}
	for _, c := range conditions {
		if slices.Contains(ends, c) {
			out.Ended = append(out.Ended, c)
			continue
		}
		out.Conditions = append(out.Conditions, c)
	}
	return out
}
