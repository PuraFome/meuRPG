package play

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// Feather Fall (SRD, Feather Fall): a reaction, when a creature within 60 feet of the
// caster falls, to slow up to five falling creatures: they take no falling damage and
// land on their feet. The fall of a combat is a pit trap (the trap's `fall_ft`): its
// damage waits, as an awaiting_reaction pending damage, until the windows of the
// fall are answered or closed, and then lands (an NPC's) or waits for the master (a
// player's character's), as every trap damage does.

// fallWindows lists the windows a fall opens: one for each combatant that has the
// spell and a slot, the reaction, and is within 60 feet of a faller it sees (itself
// included), in the initiative order.
func (s *Service) fallWindows(ctx context.Context, c *combatTx, cs []playdb.Combatant, fallers []playdb.Combatant, point string, fallFt int32) ([]windowSpec, error) {
	ids := make([]string, len(fallers))
	for i, f := range fallers {
		ids[i] = f.ID
	}
	var specs []windowSpec
	for _, r := range cs {
		if r.Defeated || isCreature(r) {
			continue
		}
		k, ok, err := s.kitOf(ctx, c.tx, c.session.CampaignID, r)
		if err != nil {
			return nil, err
		}
		if !ok || !k.canReact() || len(k.slotsFor(spellFeatherFall)) == 0 {
			continue
		}
		var near int32 = -1
		for _, f := range fallers {
			if f.ID == r.ID {
				near = 0
				break
			}
			dist, sees, err := s.reactorSees(ctx, c, cs, r, f)
			if err != nil {
				return nil, err
			}
			if sees && (near < 0 || dist < near) {
				near = dist
			}
		}
		if near < 0 {
			continue
		}
		r := r
		specs = append(specs, windowSpec{
			kind: reaction.FeatherFall, reactor: &r,
			trigger: windowTrigger{Falling: ids, FallFt: fallFt, Trap: point, Distance: near},
		})
	}
	return specs, nil
}

// releaseFall lets a fall damage whose windows are all closed land: an NPC's or a
// creature's takes it at once, a player's character's waits for the master.
func (s *Service) releaseFall(ctx context.Context, c *combatTx, p playdb.PendingDamage, byID map[string]playdb.Combatant) error {
	who, ok := byID[p.TargetID]
	if !ok || p.Amount == nil {
		_, err := c.q.SetPendingDamageStatus(ctx, playdb.SetPendingDamageStatusParams{ID: p.ID, Status: pendingDiscarded, ResolvedAt: &c.now})
		if err != nil {
			return fmt.Errorf("drop a fall damage: %w", err)
		}
		return nil
	}
	if !holdsHP(who) {
		if _, err := c.q.SetPendingDamageStatus(ctx, playdb.SetPendingDamageStatusParams{ID: p.ID, Status: pendingRolled}); err != nil {
			return fmt.Errorf("let a fall damage wait for the master: %w", err)
		}
		return nil
	}
	if _, err := c.q.SetPendingDamageStatus(ctx, playdb.SetPendingDamageStatusParams{ID: p.ID, Status: pendingApplied, ResolvedAt: &c.now}); err != nil {
		return fmt.Errorf("apply a fall damage: %w", err)
	}
	hit, _, err := s.landDamage(ctx, c, p, who, *p.Amount)
	if err != nil {
		return err
	}
	if hit.Applied && hit.After != nil {
		who.HpCurrent, who.HpTemp, who.Defeated = &hit.After.HP, &hit.After.Temp, hit.After.Defeated
		byID[who.ID] = who
	}
	return nil
}

// fallPendingsOf are the fall damages that still wait for the windows of a fall.
func fallPendingsOf(ctx context.Context, c *combatTx, point string) ([]playdb.PendingDamage, error) {
	all, err := c.q.ListOpenPendingDamages(ctx, c.enc.ID)
	if err != nil {
		return nil, fmt.Errorf("list the pending damage: %w", err)
	}
	return slices.DeleteFunc(all, func(p playdb.PendingDamage) bool {
		return p.Status != pendingAwaitingReaction || p.TrapPointID == nil || *p.TrapPointID != point
	}), nil
}

// answerFeatherFall: up to five of the falling creatures the reactor sees take no
// falling damage (their damage is discarded); one 1st-level slot and the reaction
// are spent.
func (s *Service) answerFeatherFall(ctx context.Context, c *combatTx, w playdb.ReactionWindow, reactor playdb.Combatant, use bool, req *playv1.AnswerReactionRequest, got *answered) error {
	if !use {
		return s.passWindow(ctx, c, w, reactor, got)
	}
	t := windowTriggerOf(w)
	ids := slices.Compact(slices.Sorted(slices.Values(req.GetCreatureIds())))
	if len(ids) == 0 || len(ids) > reaction.FeatherFallMaxTargets {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("creature_ids must name 1 to %d falling creatures", reaction.FeatherFallMaxTargets))
	}
	cs, err := c.q.ListCombatants(ctx, c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the combatants: %w", err)
	}
	for _, id := range ids {
		i := slices.IndexFunc(cs, func(x playdb.Combatant) bool { return x.ID == id })
		if i < 0 || !slices.Contains(t.Falling, id) {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("creature_ids must be creatures of the fall"))
		}
		if cs[i].ID == reactor.ID {
			continue
		}
		if _, sees, err := s.reactorSees(ctx, c, cs, reactor, cs[i]); err != nil {
			return err
		} else if !sees {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("creature_ids must be creatures of the fall"))
		}
	}
	k, err := s.reactorKit(ctx, c, reactor)
	if err != nil {
		return err
	}
	sl, err := pickSlot(req.GetSlot(), k.slotsFor(spellFeatherFall))
	if err != nil {
		return err
	}
	vit, err := s.spendReactionSlot(ctx, c, reactor, sl)
	if err != nil {
		return err
	}
	if vit != nil {
		got.vitals = append(got.vitals, vit)
	}
	if err := s.spendReaction(ctx, c, reactor); err != nil {
		return err
	}
	pend, err := fallPendingsOf(ctx, c, t.Trap)
	if err != nil {
		return err
	}
	for _, p := range pend {
		if slices.Contains(ids, p.TargetID) {
			if _, err := c.q.SetPendingDamageStatus(ctx, playdb.SetPendingDamageStatusParams{ID: p.ID, Status: pendingDiscarded, ResolvedAt: &c.now}); err != nil {
				return fmt.Errorf("take the fall damage away: %w", err)
			}
		}
	}
	got.ev = actionEvent{
		Round: c.enc.Round, Secret: reactor.Hidden, Actor: reactor.ID, ReactionBefore: reactor.ReactionUsed,
		Reaction: &reactionEvent{Kind: w.Kind, Window: w.ID, Used: true, ByMaster: c.master, Slot: &sl, Saved2: ids},
	}
	got.result = &playv1.ReactionResult{
		Kind: playv1.ReactionKind_REACTION_KIND_FEATHER_FALL, ByMaster: c.master, Used: true,
		Result: &playv1.ReactionResult_FeatherFall{FeatherFall: &playv1.FeatherFallResult{SavedIds: ids}},
	}
	c.lastAnswered = reaction.FeatherFall
	c.characterID = &reactor.CharacterID
	_, err = s.closeWindow(ctx, c, w, windowAnswered, reaction.ReasonNone, windowOutcome{ByMaster: c.master, Used: true, Saved: ids}, reaction.FeatherFall)
	return err
}

// fallPrompt is the prompt of a Feather Fall window: the falling creatures the
// reactor sees (the master's all), the 1st-level slots it may spend.
func (wv *windowView) fallPrompt(w playdb.ReactionWindow, reactor *playdb.Combatant, k kit, out *playv1.ReactionWindow) {
	t := windowTriggerOf(w)
	p := &playv1.FeatherFallPrompt{MaxTargets: reaction.FeatherFallMaxTargets, FallFt: t.FallFt, SlotOptions: k.slotsFor(spellFeatherFall)}
	for _, id := range t.Falling {
		f, ok := wv.byID[id]
		if !ok || (!wv.v.master && reactor != nil && f.ID != reactor.ID && !wv.v.sees(f)) {
			continue
		}
		fc := &playv1.FallingCreature{CombatantId: f.ID, Label: f.Label}
		if reactor != nil {
			fc.Ally = f.Side == reactor.Side
			if d, both := distanceFt(*reactor, f); both {
				fc.DistanceFt = &d
			}
		}
		p.Falling = append(p.Falling, fc)
	}
	out.Prompt = &playv1.ReactionWindow_FeatherFall{FeatherFall: p}
}
