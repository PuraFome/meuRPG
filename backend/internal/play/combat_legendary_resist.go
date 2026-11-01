package play

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Legendary Resistance (SRD 5.1: "If the creature fails a saving throw, it can choose to
// succeed instead", 3/Day). The trait is the master's alone, works while the creature is
// incapacitated, and is counted in creature_spent ("legendary_resistance"). The saves the master
// rolls for the creature (an effect's and a concentration's) take the choice with the roll: the
// request names it, and a roll that fails is a success that spent one use, in the same transaction
// as the roll. A roll that succeeds spends nothing and the choice is ignored, so the master need
// not guess before the dice. The event keeps the roll that failed with Resisted, and says
// "succeeded" in everything a player reads.

// legendaryResist is the master's choice for one save, once it is allowed.
type legendaryResist struct {
	// use says the master asked for the trait on this save.
	use bool
	// perDay is how many uses a day the creature's stat block gives.
	perDay int
}

// refuseResistance refuses a choice that is not the master's, before anything is read.
func refuseResistance(m authz.Membership, use bool) error {
	if use && m.Role != authz.RoleMaster {
		return connect.NewError(connect.CodePermissionDenied, errors.New("only the master uses Legendary Resistance"))
	}
	return nil
}

// legendaryResistOpen checks the choice before the roll: the creature must have the trait.
func (s *Service) legendaryResistOpen(ctx context.Context, c *combatTx, who playdb.Combatant, use bool) (legendaryResist, error) {
	if !use {
		return legendaryResist{}, nil
	}
	p, err := s.legendaryProfileOf(ctx, c, who)
	if err != nil {
		return legendaryResist{}, err
	}
	if who.Kind != kindNPC || p.ResistancePerDay == 0 {
		return legendaryResist{}, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_LEGENDARY_RESISTANCE_UNAVAILABLE, "the creature has no Legendary Resistance")
	}
	return legendaryResist{use: true, perDay: p.ResistancePerDay}, nil
}

// resists spends one use for a save that failed and says how many are left. It does nothing, and
// answers false, for a save that did not fail or a choice that was not made; it refuses with nothing
// spent when no use is left.
func (r legendaryResist) resists(ctx context.Context, c *combatTx, who playdb.Combatant, saved bool) (resisted bool, left int32, err error) {
	if saved || !r.use {
		return false, 0, nil
	}
	spent, err := readCreatureSpent(ctx, c.q, who.ID)
	if err != nil {
		return false, 0, err
	}
	if usesLeft(r.perDay, spent.count(spentLegendaryResistance)) == 0 {
		return false, 0, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_LEGENDARY_RESISTANCE_NONE_LEFT, "the creature has no Legendary Resistance left")
	}
	now, err := bumpCreatureSpent(ctx, c.q, who.ID, spentLegendaryResistance, 1)
	if err != nil {
		return false, 0, err
	}
	return true, conv.Int32(usesLeft(r.perDay, now[spentLegendaryResistance])), nil
}

// resistFailure turns the failed save of an effect into a success when the master chose to.
func (x *effectSaveStep) resistFailure(ctx context.Context, c *combatTx, saved bool) (bool, error) {
	if x.call.skip {
		return saved, nil
	}
	resisted, left, err := x.legendary.resists(ctx, c, x.reactor, saved)
	if err != nil || !resisted {
		return saved, err
	}
	x.le.Change = "saved"
	x.ev.Resisted, x.ev.ResistLeft = true, left
	x.run.result.Saved, x.run.result.Resisted, x.run.result.ResistedLeft = true, true, left
	return true, nil
}

// resistConcentration turns the failed save of a concentration into a success when the master chose to.
func (r legendaryResist) resistConcentration(ctx context.Context, c *combatTx, who playdb.Combatant, kept bool, ev *actionEvent, run *concentrationRun) (bool, error) {
	resisted, left, err := r.resists(ctx, c, who, kept)
	if err != nil || !resisted {
		return kept, err
	}
	ev.Resisted, ev.ResistLeft = true, left
	ev.Reaction.Kept, ev.Reaction.Saved = true, true
	run.result.Kept, run.result.Resisted, run.result.ResistedLeft = true, true, left
	return true, nil
}
