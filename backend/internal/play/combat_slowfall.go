package play

import (
	"context"
	"errors"
	"fmt"
	"math"

	"connectrpc.com/connect"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Slow Fall (SRD 5.1, Monk, level 4): the monk uses its reaction when it falls to reduce
// any falling damage it takes by five times its monk level. The fall of a combat is a
// pit trap whose damage waits for the master (or for a Feather Fall window). The
// feature's button, the reaction, takes the points off that damage: the pit fall
// pending against the monk that has not landed yet. A fall that already landed cannot
// be reduced (the master applies a player's fall damage by hand, so the monk has the
// time to ask first). Unlike Feather Fall, the monk is not asked by a window: it uses
// the button when the fall happens.

// slowFallAction is the monk's feature action.
const slowFallAction = "feature:slow-fall"

// slowFallPerLevel is the points of falling damage Slow Fall reduces for each monk
// level (SRD 5.1).
const slowFallPerLevel = 5

// fallCut is one fall damage Slow Fall took points off, with its amount before.
type fallCut struct {
	Pending string `json:"pending_id"`
	Before  int32  `json:"before"`
}

// slowFall takes five times the monk's level off the fall damage that still waits
// for the monk, and returns what it changed and how many points it took off. Several
// parts of one fall share the reduction in the order they were rolled. It refuses
// when there is no fall damage to reduce, so the reaction is not spent for nothing.
func (s *Service) slowFall(ctx context.Context, c *combatTx, who playdb.Combatant, monkLevel int) ([]fallCut, int32, error) {
	noFall := connect.NewError(connect.CodeFailedPrecondition, errors.New("there is no fall damage to reduce"))
	if s.traps == nil || c.enc.MapID == nil || monkLevel < 1 {
		return nil, 0, noFall
	}
	traps, err := s.traps.Traps(ctx, c.tx, c.session.CampaignID, *c.enc.MapID)
	if err != nil {
		return nil, 0, err
	}
	pits := map[string]bool{}
	for _, t := range traps {
		if s.fallDepth(t) > 0 {
			pits[t.PointID] = true
		}
	}
	open, err := c.q.ListOpenPendingDamages(ctx, c.enc.ID)
	if err != nil {
		return nil, 0, fmt.Errorf("list the pending damage: %w", err)
	}
	var fall []playdb.PendingDamage // the damages of the monk's last fall
	for _, p := range open {
		if p.TargetID != who.ID || p.TrapPointID == nil || !pits[*p.TrapPointID] || p.Amount == nil || *p.Amount < 1 || p.DamageType != "bludgeoning" {
			continue
		}
		if p.Status != pendingRolled && p.Status != pendingAwaitingReaction {
			continue
		}
		if len(fall) > 0 && *fall[0].TrapPointID != *p.TrapPointID {
			fall = nil // an older fall: the last one is the one that happens
		}
		fall = append(fall, p)
	}
	if len(fall) == 0 {
		return nil, 0, noFall
	}
	left := int32(min(monkLevel*slowFallPerLevel, math.MaxInt32))
	var cuts []fallCut
	var taken int32
	for _, p := range fall {
		cut := min(left, *p.Amount)
		if cut < 1 {
			break
		}
		now := *p.Amount - cut
		if err := c.q.SetPendingDamageLanding(ctx, playdb.SetPendingDamageLandingParams{ID: p.ID, Amount: &now}); err != nil {
			return nil, 0, fmt.Errorf("take the fall damage down: %w", err)
		}
		cuts = append(cuts, fallCut{Pending: p.ID, Before: *p.Amount})
		left, taken = left-cut, taken+cut
	}
	return cuts, taken, nil
}
