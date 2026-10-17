package play

import (
	"context"
	"math"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Aid's bonus to the hit point maximum. The combat (raiseMaxHP, endAid) and a cast outside
// one (maxHPTargets, takeBackMaxHP) share this arithmetic and the two functions that move a
// character's bonus (raiseCharacterMaxHP, lowerCharacterAid), so one place owns "raise the
// maximum" and "take it back".

// aidRaise is what a new Aid does to a target that already has bonusBefore from an
// earlier one: casts of the same spell do not combine, the stronger bonus applies, and
// the maximum (and the current hit points) rise by the difference.
func aidRaise(bonusBefore, amount int32) (stronger, rose int32) {
	stronger = max(bonusBefore, amount)
	return stronger, stronger - bonusBefore
}

// aidEnd is the hit points and the maximum of a target whose bonus ends: the maximum
// falls back (never below 1) and the current hit points lose only what is above it,
// never the last one of a target that still has some.
func aidEnd(hp, maxBefore, bonus int32) (hpAfter, maxAfter int32) {
	maxAfter = max(maxBefore-bonus, 1)
	hpAfter = hp
	if hp > 0 {
		hpAfter = max(min(hp, maxAfter), 1)
	}
	return hpAfter, maxAfter
}

// aidLeft is the bonus a target keeps when one Aid ends: the strongest bonus of the
// other Aids still on it (0 for none), never more than the bonus it has.
func aidLeft(bonus int32, others []int32) int32 {
	left := int32(0)
	for _, o := range others {
		left = max(left, o)
	}
	return min(bonus, left)
}

// aidTargetsOf lists the targets a cast raised the maximum of.
func aidTargetsOf(r playdb.SpellCast) []castTarget {
	var out []castTarget
	for _, t := range castTargetsOf(r) {
		if t.Effect == castMaxHP {
			out = append(out, t)
		}
	}
	return out
}

// maxHPTargets is Aid cast outside a combat: it raises the maximum and the current hit
// points of each player character among the targets, as it does in a combat
// (raiseCharacterMaxHP). An NPC has no hit points outside a combat: the master narrates.
// The bonus stays until the cast ends (takeBackMaxHP).
func (s *Service) maxHPTargets(ctx context.Context, c *combatTx, plan castPlan, out *castOutcome) ([]castTarget, error) {
	amount := int32(0)
	if hp := plan.osp.Spell.HP; hp != nil {
		amount = clamp32(hp.Amount, 0, math.MaxInt32)
	}
	var targets []castTarget
	for _, t := range s.effectTargetsAll(plan) {
		if !t.Player {
			targets = append(targets, castTarget{ID: t.ID, Effect: castNarrated})
			continue
		}
		ft, err := s.readFxTarget(ctx, c, stub(t))
		if err != nil {
			return nil, err
		}
		var hit castHit
		v, err := s.raiseCharacterMaxHP(ctx, c, ft, amount, &hit)
		if err != nil {
			return nil, err
		}
		out.vitals = append(out.vitals, v)
		ct := castTarget{ID: t.ID, Effect: castMaxHP, Amount: amount, Before: clamp32(ft.hp, 0, math.MaxInt32)}
		if hit.HPAfter != nil {
			ct.After = *hit.HPAfter
		}
		targets = append(targets, ct)
	}
	return targets, nil
}

// takeBackMaxHP takes Aid's hit point bonus off the targets of a cast that ended, down to the
// strongest bonus of the other Aids still on the target (two casts do not combine, so the
// weaker one shows again when the stronger ends). A character that is gone has nothing to take
// back. It returns the vitals it changed.
func (s *Service) takeBackMaxHP(ctx context.Context, c *combatTx, r playdb.SpellCast) ([]*playv1.CharacterVitals, error) {
	var vitals []*playv1.CharacterVitals
	for _, t := range aidTargetsOf(r) {
		others, err := s.otherAidOn(ctx, c, r, t.ID)
		if err != nil {
			return nil, err
		}
		now, err := s.vitals.GetVitalsTx(ctx, c.tx, c.session.CampaignID, t.ID)
		if connect.CodeOf(err) == connect.CodeNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		_, after, err := s.lowerCharacterAid(ctx, c, t.ID, aidLeft(now.GetHitPointsMaxBonus(), others))
		if err != nil {
			return nil, err
		}
		if after.GetHitPointsMaxBonus() != now.GetHitPointsMaxBonus() {
			vitals = append(vitals, after)
		}
	}
	return vitals, nil
}

// otherAidOn is the bonus each other Aid that still lasts gives the character.
func (s *Service) otherAidOn(ctx context.Context, c *combatTx, r playdb.SpellCast, characterID string) ([]int32, error) {
	casts, err := s.otherCastsOn(ctx, c, r, characterID, castMaxHP)
	if err != nil {
		return nil, err
	}
	var out []int32
	for _, o := range casts {
		for _, t := range aidTargetsOf(o) {
			if t.ID == characterID {
				out = append(out, t.Amount)
			}
		}
	}
	return out, nil
}

// lowerCharacterAid puts a character's Aid bonus at left (0 ends it): the maximum falls
// back and the current hit points lose only what is above it, never the last one
// (aidEnd's rule, kept by the characters module). It returns the vitals before and after.
func (s *Service) lowerCharacterAid(ctx context.Context, c *combatTx, characterID string, left int32) (before, after *playv1.CharacterVitals, err error) {
	return s.vitals.SetHitPointsMaxBonus(ctx, c.tx, c.session.CampaignID, characterID, left)
}
