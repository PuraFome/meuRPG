package play

import (
	"context"
	"fmt"
	"math"
	"slices"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// The question of Slow Fall (SRD 5.1, Monk, level 4): a monk that falls may use its reaction to take
// five times its monk level off the falling damage. The fall of a combat is a pit trap; while a window
// of the fall is open its damage waits (awaiting_reaction), as Feather Fall's does, and it lands, or
// waits for the master, only when every window of the fall is closed. Only a player's character is
// asked: an NPC's or a creature's fall is the master's to run, and the monk's button (slowFall) stays
// for a fall that already landed or that was not a pit.

// slowFallMinLevel is the monk level that has Slow Fall.
const slowFallMinLevel = 4

// fallDamagesOf is the falling damage each caught combatant rolled, by combatant, for the combatants
// that took some: the damages of the outcome that are of the fall's type. Only a player's character is
// asked, and a player's character takes the damage as rolled (the resistances of a character are the
// master's, applied by hand: afterResistance changes nothing for it), so this is the amount stored.
func fallDamagesOf(caught []playdb.Combatant, outcomes []trapOutcome) map[string]int32 {
	out := map[string]int32{}
	for i, o := range outcomes {
		if i >= len(caught) {
			break
		}
		for _, d := range o.damages {
			if d.damageType == fallDamageType && d.amount > 0 {
				out[caught[i].ID] += clamp32(d.amount, 0, math.MaxInt32)
			}
		}
	}
	return out
}

// slowFallSpec is the Slow Fall window of a reactor that falls: a monk of level 4 or higher, with its
// reaction (the caller checked it), whose own fall damage is rolled. Only a player's character is asked:
// an NPC's fall is the master's to run.
func slowFallSpec(k kit, r *playdb.Combatant, falls map[string]int32, point string, fallFt int32) (windowSpec, bool) {
	if r.Kind != kindPlayer || k.st.MonkLevel < slowFallMinLevel || falls[r.ID] <= 0 {
		return windowSpec{}, false
	}
	return windowSpec{
		kind: reaction.SlowFall, reactor: r,
		trigger: windowTrigger{Falling: []string{r.ID}, FallFt: fallFt, Trap: point},
	}, true
}

// answerSlowFall: "Usar" takes five times the monk level off the monk's own fall damage that waits
// (the points are kept in the event for the undo) and spends the reaction; "Deixar passar" lets the
// fall be.
func (s *Service) answerSlowFall(ctx context.Context, c *combatTx, w playdb.ReactionWindow, reactor playdb.Combatant, use bool, got *answered) error {
	if !use {
		return s.passWindow(ctx, c, w, reactor, got)
	}
	k, err := s.reactorKit(ctx, c, reactor)
	if err != nil {
		return err
	}
	fall, err := ownFallOf(ctx, c, windowTriggerOf(w).Trap, reactor)
	if err != nil {
		return err
	}
	if len(fall) == 0 {
		return errNoFall()
	}
	var before int32
	for _, p := range fall {
		before += *p.Amount
	}
	cuts, taken, err := reduceFall(ctx, c, fall, k.st.MonkLevel)
	if err != nil {
		return err
	}
	if taken < 1 {
		return errNoFall()
	}
	if err := s.spendReaction(ctx, c, reactor, reaction.SlowFall); err != nil {
		return err
	}
	after := before - taken
	got.ev = actionEvent{
		Round: c.enc.Round, Secret: reactor.Hidden, Actor: reactor.ID, ReactionBefore: reactor.ReactionUsed, ACBonusBefore: reactor.AcBonus,
		FallCuts: cuts, Amount: taken,
		Reaction: &reactionEvent{Kind: w.Kind, Window: w.ID, Used: true, ByMaster: c.master, Before: before, After: after, Reduction: taken},
	}
	res := &playv1.SlowFallResult{Reduction: taken}
	if c.master {
		res.DamageBefore, res.DamageAfter = &before, &after
	}
	got.kind = eventReactionUsed
	got.result = &playv1.ReactionResult{
		Kind: playv1.ReactionKind_REACTION_KIND_SLOW_FALL, ByMaster: c.master, Used: true,
		Result: &playv1.ReactionResult_SlowFall{SlowFall: res},
	}
	c.lastAnswered = reaction.SlowFall
	c.characterID = &reactor.CharacterID
	_, err = s.closeWindow(ctx, c, w, windowAnswered, reaction.ReasonNone, windowOutcome{ByMaster: c.master, Used: true, Reduction: taken, Before: before, After: after}, reaction.SlowFall)
	return err
}

// ownFallOf is the fall damage of the combatant that still waits for the windows of the fall.
func ownFallOf(ctx context.Context, c *combatTx, point string, who playdb.Combatant) ([]playdb.PendingDamage, error) {
	pend, err := fallPendingsOf(ctx, c, point)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(pend, func(p playdb.PendingDamage) bool {
		return p.TargetID != who.ID || p.DamageType != fallDamageType || p.Amount == nil || *p.Amount < 1
	}), nil
}

// slowFallPrompt is the prompt of a Slow Fall window: how deep the fall is and the points the feature
// takes off. The damage itself is not in it.
func slowFallPrompt(f promptFacts, out *playv1.ReactionWindow) {
	out.Prompt = &playv1.ReactionWindow_SlowFall{SlowFall: &playv1.SlowFallPrompt{
		FallFt: f.t.FallFt, Reduction: clamp32(f.k.st.MonkLevel*slowFallPerLevel, 0, math.MaxInt32),
	}}
}

// slowFall is the line of a Slow Fall: the reactor's player and the master read the points taken off,
// the master also the damage before and after; the others read that the monk used it.
func (f *reactionFacts) slowFall() (string, bool) {
	re := f.re
	if !re.Used || !f.seesReactor {
		return "", false
	}
	switch {
	case f.v.master:
		return fmt.Sprintf("%s usou Queda Lenta: -%d no dano da queda (de %d para %d).%s", f.who, re.Reduction, re.Before, re.After, f.by), true
	case f.numbers:
		return fmt.Sprintf("%s usou Queda Lenta: -%d no dano da queda.%s", f.who, re.Reduction, f.by), true
	}
	return fmt.Sprintf("%s usou Queda Lenta.%s", f.who, f.by), true
}
