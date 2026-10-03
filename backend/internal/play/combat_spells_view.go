package play

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// How a cast is shown (RN-20): the caster's player and the master get the d20 of
// a spell attack and the DC; a target's saving throw dice go to the master and
// the target's own player only, everyone else gets the outcome as a word (an
// NPC's dice never reach a player); whether an NPC's save bonus is known is the
// master's.

func slotProto(s *slotRef) *playv1.SpellSlot {
	if s == nil {
		return nil
	}
	return &playv1.SpellSlot{Level: s.Level, Pact: s.Pact}
}

// saveView is a target's saving throw as the viewer may see it.
func saveView(sr *saveRoll, v combatViewer, caster, target playdb.Combatant) *playv1.SaveResult {
	if sr == nil {
		return nil
	}
	out := &playv1.SaveResult{Outcome: playv1.SaveOutcome_SAVE_OUTCOME_FAILED}
	if sr.Saved {
		out.Outcome = playv1.SaveOutcome_SAVE_OUTCOME_SAVED
	}
	if v.master || v.owns(target) {
		out.Roll = diceRoll(1, 20, []int32{sr.D20}, sr.Bonus, sr.Total, false)
	}
	if v.master {
		out.BonusKnown = !sr.Unknown
	}
	if v.master || v.owns(caster) {
		out.Dc = sr.DC
	}
	return out
}

// attackRollView is a spell attack's d20 for the master and the caster's player.
func attackRollView(h castHit, v combatViewer, caster playdb.Combatant) *playv1.DiceRoll {
	if h.Outcome == "" || (!v.master && !v.owns(caster)) {
		return nil
	}
	return diceRoll(1, 20, []int32{h.D20}, h.Modifier, h.Total, h.Physical)
}

// castProto builds the SpellCast a cast event tells, for the viewer, who is the
// caster's player or the master: the pending damages of the cast, with their
// current state.
func (s *Service) castProto(ctx context.Context, res combatResult, ev actionEvent, v combatViewer) (*playv1.SpellCast, error) {
	cs, err := s.queries.ListCombatants(ctx, res.encounterID)
	if err != nil {
		return nil, s.dbError(ctx, "list the combatants", err)
	}
	byID := make(map[string]playdb.Combatant, len(cs))
	for _, c := range cs {
		byID[c.ID] = c
	}
	caster := byID[ev.Actor]
	out := &playv1.SpellCast{
		CastId: ev.CastID, SpellKey: ev.Key, Slot: slotProto(ev.Slot), Concentrating: ev.Concentrate,
		ConcentrationEndedSpellKey: ev.ConcEnded,
	}
	for _, h := range ev.Hits {
		target := byID[h.Target]
		r := &playv1.SpellTargetResult{
			CombatantId: h.Target, Darts: h.Darts, Outcome: outcomeToProto[h.Outcome],
			AttackRoll: attackRollView(h, v, caster), Save: saveView(h.Save, v, caster, target),
		}
		if h.Pending != "" && (v.master || v.owns(caster)) {
			r.PendingDamageId = h.Pending
			p, err := s.queries.GetPendingDamage(ctx, playdb.GetPendingDamageParams{EncounterID: res.encounterID, ID: h.Pending})
			switch {
			case errors.Is(err, pgx.ErrNoRows): // an undo took it away meanwhile
				r.PendingDamageId = ""
			case err != nil:
				return nil, s.dbError(ctx, "read the pending damage", err)
			default:
				out.PendingDamages = append(out.PendingDamages, pendingProto(p, cs))
			}
		}
		out.Targets = append(out.Targets, r)
	}
	return out, nil
}
