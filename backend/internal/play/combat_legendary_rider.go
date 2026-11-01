package play

import (
	"context"

	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// Legendary Resistance on the save of a hit rider (SRD 5.1, "Legendary Resistance"; Stunning Strike, the Open Hand
// technique, a creature's rider). The answer of a rider that rolls the save of a creature with the trait and a use left
// waits in a window of the master's alone, exactly as a cast does (combat_legendary_hold.go): the answer runs under a
// savepoint, keeps the target's roll, rolls its writes back and holds the request (kind hit_rider).
// It waits whether the save passed or failed, so the wait tells a player nothing; the window offers only a failed save.
// The replay lands the very same d20, turns the failed save into a success when the master chose to (spending the use in
// its own transaction) and only then closes the offer. A rider that rolls no save, a decline, a target that is not an
// NPC with the trait and a use left are never held.
// A hit rider is not undoable (it is not in undoableKinds): the use a replay spent is not given back, as for any
// answered rider.

// guardRider opens the savepoint of a rider answer that may be held: the choice rolls the target's save and the target
// is an NPC with Legendary Resistance and a use left, or the answer is the replay of a held one (which has the savepoint
// too, so a replay that is dropped leaves nothing). Any other answer has none.
func (s *Service) guardRider(ctx context.Context, c *combatTx, row playdb.HitRider, target playdb.Combatant, choice string) (*resistScope, error) {
	rolls := riderSaveAbility(choice) != "" || (row.Kind == riderCreature && choice == choiceApply)
	if !rolls || target.Kind != kindNPC {
		return nil, nil
	}
	capable := map[string]bool{}
	if c.replay.keptSave(target.ID) == nil { // not decided yet: the first run of the answer
		resist, err := s.resistancesOf(ctx, c, []playdb.Combatant{target})
		if err != nil {
			return nil, err
		}
		if resist[target.ID].left == 0 {
			return nil, nil
		}
		capable[target.ID] = true
		if err := s.reactionGate(ctx, c); err != nil { // a rider that may wait for the master waits for no other window either
			return nil, err
		}
	}
	sp, err := c.tx.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &resistScope{sp: sp, before: *c, capable: capable}, nil
}

// abort ends the guard of an answer that failed: what it wrote since the savepoint is rolled back.
func (g *resistScope) abort(ctx context.Context, c *combatTx) {
	if g != nil {
		g.rollback(ctx, c)
	}
}

// release ends the guard of an answer that went through, which keeps its writes.
func (g *resistScope) release(ctx context.Context) error {
	if g == nil {
		return nil
	}
	return g.sp.Commit(ctx)
}

// holdRider holds the answer of a rider whose target has the trait and a use left, whether its save passed or failed.
// It rolls the answer's writes back, keeps the target's roll with the request, and opens the window of the master's
// alone with the failed save as its only offer. It answers the event that stands for the held request, and false when
// the answer is not held.
func (s *Service) holdRider(ctx context.Context, c *combatTx, m authz.Membership, g *resistScope, msg proto.Message, row playdb.HitRider, attacker, target playdb.Combatant, re *riderEvent, ev actionEvent) (actionEvent, bool, error) {
	if g == nil || !g.capable[target.ID] || re.save == nil {
		return actionEvent{}, false, nil
	}
	var offers []resistOffer
	if !re.save.Saved {
		offers = append(offers, resistOffer{Who: target.ID, D20: re.save.D20, Total: re.save.Total, DC: re.save.DC})
	}
	kept := []keptSave{{Target: target.ID, Roll: *re.save}}
	g.rollback(ctx, c)
	spec := windowSpec{kind: reaction.LegendaryResistance, trigger: windowTrigger{
		Actor: attacker.ID, Key: row.Kind, Roll: "rider", Resist: offers, Held: []string{target.ID},
	}}
	ev, err := s.holdAction(ctx, c, m, "hit_rider", attacker, msg, holdData{Saves: kept}, []windowSpec{spec}, ev)
	if err != nil {
		return actionEvent{}, false, err
	}
	if err := touch(ctx, c); err != nil {
		return actionEvent{}, false, err
	}
	c.characterID = &attacker.CharacterID
	return ev, true, nil
}

// resistRiderSave turns the failed save of the target of a rider into a success when the master chose to in the window
// of the held answer, spending the use in the replay's transaction. A save that passed keeps its result.
func (s *Service) resistRiderSave(ctx context.Context, c *combatTx, target playdb.Combatant, re *riderEvent, saved bool) (bool, error) {
	if saved || !c.replay.resists(target.ID) {
		return saved, nil
	}
	resisted, left, err := s.spendResistance(ctx, c, target)
	if err != nil || !resisted {
		return false, err
	}
	re.Resisted, re.ResistLeft = true, left
	if re.save != nil {
		re.save.Saved, re.save.Resisted, re.save.ResistedLeft = true, true, left
	}
	return true, nil
}

// replayHeldRider runs the held answer of a rider again, as the one who gave it.
func (s *Service) replayHeldRider(ctx context.Context, m authz.Membership, h playdb.ReactionHold) error {
	var req playv1.UseHitRiderRequest
	if err := proto.Unmarshal(h.Request, &req); err != nil {
		return err
	}
	_, err := s.useHitRider(ctx, m, &req)
	return err
}
