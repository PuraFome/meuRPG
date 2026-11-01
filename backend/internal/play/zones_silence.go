package play

import (
	"context"
	"errors"
	"slices"

	"connectrpc.com/connect"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// What Silence does (SRD 5.1) to a creature entirely inside its sphere: it is immune to thunder damage, it is
// deafened, and it cannot cast a spell with a verbal component. Every combatant occupies one square, so
// "entirely inside" is that square being in the zone; occupiedSquares is the one place that says which squares a
// creature covers. The zone asks nobody and has no trigger: the three effects are read where they matter, from the
// zones of the combat. The deafened condition is derived, never stored, so it ends with the zone or the step out.

const (
	// silenceImmune is the damage type a creature in Silence is immune to.
	silenceImmune = "damage-type:thunder"
	// silenceSource names the immunity in the steps of a damage.
	silenceSource = "spell:silence"
	// conditionDeafened is the condition a creature in Silence has.
	conditionDeafened = "condition:deafened"
)

// occupiedSquares are the squares the combatant covers: none off the map, one on it.
func occupiedSquares(c playdb.Combatant) []grid.Square {
	if !placed(c) {
		return nil
	}
	return []grid.Square{squareOfCombatant(c)}
}

// holds says every square of the combatant is in the zone.
func (z *zone) holds(c playdb.Combatant) bool {
	sq := occupiedSquares(c)
	if len(sq) == 0 {
		return false
	}
	for _, s := range sq {
		if !z.has(s) {
			return false
		}
	}
	return true
}

// silencedBy says the combatant is entirely inside a Silence among the zones.
func silencedBy(zs []*zone, c playdb.Combatant) bool {
	for _, z := range zs {
		if z.def.silences && z.holds(c) {
			return true
		}
	}
	return false
}

// silenceImmunity is the immunity to thunder damage that a Silence gives, as a step of the damage.
func silenceImmunity() combat.Modifier {
	return combat.Modifier{Kind: combat.StepImmunity, Source: silenceSource, DamageTypes: []string{silenceImmune}}
}

// withSilenceDeaf gives each combatant entirely inside a Silence the deafened condition, for the view. The slice
// is a copy where it changes, so nothing stored is touched; a creature already deafened is left as it is.
func withSilenceDeaf(cs []playdb.Combatant, zs []*zone) []playdb.Combatant {
	var out []playdb.Combatant
	for i, c := range cs {
		if c.Defeated || !silencedBy(zs, c) || slices.Contains(c.Conditions, conditionDeafened) {
			continue
		}
		if out == nil {
			out = append([]playdb.Combatant(nil), cs...)
		}
		out[i].Conditions = append(append([]string(nil), c.Conditions...), conditionDeafened)
	}
	if out == nil {
		return cs
	}
	return out
}

// checkSilence refuses a spell with a verbal component to a caster entirely inside a Silence.
func (f *castFlow) checkSilence(ctx context.Context) error {
	if !f.sp.Verbal || !placed(f.caster) {
		return nil
	}
	zs, err := listZones(ctx, f.c)
	if err != nil {
		return err
	}
	if silencedBy(zs, f.caster) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("the caster is inside a Silence: a spell with a verbal component cannot be cast there"))
	}
	return nil
}
