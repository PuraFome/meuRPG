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

// fallWindows lists the windows a fall opens, in the initiative order: for each combatant that has
// the reaction, a Feather Fall one when it has the spell and a slot and is within 60 feet of a faller
// it sees (itself included), and a Slow Fall one when it is a monk of level 4 or higher whose own fall
// damage is rolled (falls is the damage of each faller, by combatant). The reactor has one reaction:
// the first answer closes the other window.
func (s *Service) fallWindows(ctx context.Context, c *combatTx, cs []playdb.Combatant, fallers []playdb.Combatant, falls map[string]int32, point string, fallFt int32) ([]windowSpec, error) {
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
		if !ok || !k.canReact() {
			continue
		}
		feather, err := s.featherFallSpec(ctx, c, cs, fallers, k, r, windowTrigger{Falling: ids, FallFt: fallFt, Trap: point})
		if err != nil {
			return nil, err
		}
		specs = append(specs, feather...)
		if spec, ok := slowFallSpec(k, &r, falls, point, fallFt); ok {
			specs = append(specs, spec)
		}
	}
	return specs, nil
}

// featherFallSpec is the Feather Fall window of a reactor that has the spell and a slot and sees a
// faller within 60 feet (none otherwise).
func (s *Service) featherFallSpec(ctx context.Context, c *combatTx, cs []playdb.Combatant, fallers []playdb.Combatant, k kit, r playdb.Combatant, t windowTrigger) ([]windowSpec, error) {
	if len(k.slotsFor(spellFeatherFall)) == 0 {
		return nil, nil
	}
	near, err := s.nearestFaller(ctx, c, cs, r, fallers)
	if err != nil || near < 0 {
		return nil, err
	}
	t.Distance = near
	return []windowSpec{{kind: reaction.FeatherFall, reactor: &r, trigger: t}}, nil
}

// nearestFaller is how far the reactor is from the nearest faller it sees, in feet: 0 when it is
// one of them, -1 when it sees none.
func (s *Service) nearestFaller(ctx context.Context, c *combatTx, cs []playdb.Combatant, r playdb.Combatant, fallers []playdb.Combatant) (int32, error) {
	var near int32 = -1
	for _, f := range fallers {
		if f.ID == r.ID {
			return 0, nil
		}
		dist, sees, err := s.reactorSees(ctx, c, cs, r, f)
		if err != nil {
			return 0, err
		}
		if sees && (near < 0 || dist < near) {
			near = dist
		}
	}
	return near, nil
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
	if err := s.checkFallers(ctx, c, reactor, t, ids); err != nil {
		return err
	}
	sl, err := s.spendFeatherFall(ctx, c, reactor, req, got)
	if err != nil {
		return err
	}
	if err := s.takeFallAway(ctx, c, t.Trap, ids); err != nil {
		return err
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

// checkFallers refuses a creature that is not one of the fall or that the reactor does not see.
func (s *Service) checkFallers(ctx context.Context, c *combatTx, reactor playdb.Combatant, t windowTrigger, ids []string) error {
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return err
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
	return nil
}

// spendFeatherFall spends the slot the answer chose and the reactor's reaction, and returns the slot.
func (s *Service) spendFeatherFall(ctx context.Context, c *combatTx, reactor playdb.Combatant, req *playv1.AnswerReactionRequest, got *answered) (slotRef, error) {
	k, err := s.reactorKit(ctx, c, reactor)
	if err != nil {
		return slotRef{}, err
	}
	sl, err := pickSlot(req.GetSlot(), k.slotsFor(spellFeatherFall))
	if err != nil {
		return sl, err
	}
	vit, err := s.spendSlotOf(ctx, c, reactor, sl)
	if err != nil {
		return sl, err
	}
	if vit != nil {
		got.vitals = append(got.vitals, vit)
	}
	return sl, s.spendReaction(ctx, c, reactor, reaction.FeatherFall)
}

// takeFallAway discards the fall damage of the creatures the answer saved.
func (s *Service) takeFallAway(ctx context.Context, c *combatTx, trap string, ids []string) error {
	pend, err := fallPendingsOf(ctx, c, trap)
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
	return nil
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
