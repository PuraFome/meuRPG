package play

import (
	"context"
	"fmt"
	"math"
	"slices"
	"time"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// The turn clock of the effects that last (RN-22), in the order the SRD and the board
// give, in one transaction:
//
//	end of the turn of N   (1) the saving throws N's effects ask open as windows, in a
//	                           fixed order (openEndSaves); (2) they are answered; (3) what
//	                           ends at the end of N's turn ends (effectsBeforeTurn); (4) the
//	                           turn passes (releaseHeldTurn, or EndTurn itself when no window
//	                           opened).
//	start of the turn of M (1) what ends at its start ends: Esquivar, an effect of rounds
//	                           whose caster is M; (2) the damage of an effect that hits who
//	                           starts the turn under it is rolled, once for each creature when
//	                           it says so; (3) it waits for the master (PENDING_DAMAGE), or
//	                           lands on an NPC at once; (4) the saving throws asked at the
//	                           start of the turn open; (5) the turn is playable.
//
// A second tap on "Encerrar turno" finds the part ended and changes nothing, and a window
// opens once for a turn (the unique windows of openEffectSave).

// effectsBeforeTurn ends what ends at the end of the turn that is ending: it runs as the next
// turn starts, once the saving throws that held the turn were answered.
func (s *Service) effectsBeforeTurn(ctx context.Context, c *combatTx) error {
	cs, err := c.q.ListCombatants(ctx, c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the combatants: %w", err)
	}
	members := turnMembers(cs)
	if len(members) == 0 {
		return nil
	}
	rows, err := c.q.ListLastingEffects(ctx, c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the effects: %w", err)
	}
	var ending []playdb.CombatantState
	for _, st := range rows {
		for _, m := range members {
			if endsAtOf(st).ExpiresAtEnd(m.ID, int(c.enc.Round)) {
				ending = append(ending, st)
				break
			}
		}
	}
	return s.endEffectRows(ctx, c, cs, ending, endDuration)
}

// effectsAfterTurnStart does what the start of the turn of ids does to the effects.
func (s *Service) effectsAfterTurnStart( //nolint:gocyclo // the steps of the start of a turn, in the order the SRD and the board give
	ctx context.Context, c *combatTx, ids []string, round int32,
) error {
	cs, err := c.q.ListCombatants(ctx, c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the combatants: %w", err)
	}
	rows, err := c.q.ListLastingEffects(ctx, c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the effects: %w", err)
	}
	// (1) What ends now: at the start of its anchor's turn, or a round past its end with
	// an anchor that never had its turn (it left, or it is defeated).
	var ending []playdb.CombatantState
	for _, st := range rows {
		e := endsAtOf(st)
		anchorGone := !e.Timed() || e.CombatantID == "" || !slices.ContainsFunc(cs, func(o playdb.Combatant) bool { return o.ID == e.CombatantID && !o.Defeated })
		if e.ExpiresAtStart(ids, int(round)) || (e.Timed() && int(round) > e.Round && anchorGone) {
			ending = append(ending, st)
		}
	}
	if err := s.endEffectRows(ctx, c, cs, ending, endDuration); err != nil {
		return err
	}
	if err := s.refreshCombatants(ctx, c, ids...); err != nil {
		return err
	}
	if rows, err = c.q.ListLastingEffects(ctx, c.enc.ID); err != nil {
		return fmt.Errorf("list the effects: %w", err)
	}
	if cs, err = c.q.ListCombatants(ctx, c.enc.ID); err != nil {
		return fmt.Errorf("list the combatants: %w", err)
	}
	// (1b) The temporary hit points an effect gives at the start of each turn (Heroísmo).
	for _, id := range ids {
		i := slices.IndexFunc(cs, func(o playdb.Combatant) bool { return o.ID == id })
		if i < 0 || cs[i].Defeated {
			continue
		}
		var mods []rules.EffectModifier
		var from *playdb.CombatantState
		for k := range rows {
			if rows[k].CombatantID == id {
				if m := effectModifiers(rows[k]); combat.TurnTempHP(m) > 0 {
					mods, from = append(mods, m...), &rows[k]
				}
			}
		}
		if amount := combat.TurnTempHP(mods); amount > 0 {
			if err := s.turnTempHP(ctx, c, cs[i], *from, amount, round); err != nil {
				return err
			}
		}
	}
	// (2) and (3) The damage of an effect, then (4) the saving throws of the start.
	group := newGroup()
	for _, st := range rows {
		i := slices.IndexFunc(cs, func(o playdb.Combatant) bool { return o.ID == st.CombatantID })
		if i < 0 || !slices.Contains(ids, st.CombatantID) || cs[i].Defeated {
			continue
		}
		if st.TriggerDice != nil && st.TriggersFired < derefInt32(st.TriggerMaxTriggers) {
			if err := s.effectDamage(ctx, c, cs[i], st, round); err != nil {
				return err
			}
		}
	}
	if rows, err = c.q.ListLastingEffects(ctx, c.enc.ID); err != nil {
		return fmt.Errorf("list the effects: %w", err)
	}
	for _, st := range rows {
		i := slices.IndexFunc(cs, func(o playdb.Combatant) bool { return o.ID == st.CombatantID })
		if i < 0 || !slices.Contains(ids, st.CombatantID) || cs[i].Defeated || st.StartSaveAbility == nil {
			continue
		}
		if _, err := s.openEffectSave(ctx, c, group, st, cs[i], combat.PhaseStart, false); err != nil {
			return err
		}
	}
	return nil
}

// effectDamage rolls the damage of an effect that hits who starts the turn under it (the web
// that burns: 2d4 fire, SRD, Web) and lands it as every damage does: an NPC takes it at once,
// a player's character's waits for the master (RN-02). When it was the last time it hits, the
// effect and what it held end.
func (s *Service) effectDamage(ctx context.Context, c *combatTx, who playdb.Combatant, st playdb.CombatantState, round int32) error {
	f, ok := rules.ParseDice(deref(st.TriggerDice))
	if !ok || f.Count < 1 {
		return nil
	}
	roll, err := dice.Roll(s.roller, dice.Expr{Count: f.Count, Sides: f.Sides, Modifier: f.Bonus})
	if err != nil {
		return fmt.Errorf("roll an effect's damage: %w", err)
	}
	status, resolved := pendingRolled, (*time.Time)(nil)
	if holdsHP(who) {
		status, resolved = pendingApplied, &c.now
	}
	dtype := deref(st.TriggerDamageType)
	amount, err := s.afterResistance(ctx, c, who, dtype, clamp32(roll.Total, 0, math.MaxInt32))
	if err != nil {
		return err
	}
	p, err := c.q.InsertEffectPendingDamage(ctx, playdb.InsertEffectPendingDamageParams{
		EncounterID: c.enc.ID, TargetID: who.ID, Status: status, DiceCount: clamp32(f.Count, 0, 100), DiceSides: clamp32(f.Sides, 0, 100),
		DiceBonus: clamp32(f.Bonus, -1000, 1000), DamageType: dtype, Faces: faces32(roll.Faces), Amount: &amount, RollTotal: new(clamp32(roll.Total, math.MinInt32, math.MaxInt32)),
		CreatedAt: c.now, ResolvedAt: resolved, EffectSourceKey: st.SourceKey,
	})
	if err != nil {
		return fmt.Errorf("open the effect's damage: %w", err)
	}
	hit, _, err := s.landDamage(ctx, c, p, who, amount)
	if err != nil {
		return err
	}
	if err := c.q.CountLastingEffectTrigger(ctx, st.ID); err != nil {
		return fmt.Errorf("count the effect's damage: %w", err)
	}
	if err := insertEvent(ctx, c, eventLastingTriggered, &c.actorUserID, nil, actionEvent{
		Round: round, Secret: s.effectHiddenFrom(st, []playdb.Combatant{who}), Actor: who.ID, Target: who.ID, Pending: p.ID, Key: deref(st.SourceKey),
		Applied: hit.Applied, Before: hit.Before, After: hit.After,
		Lasting: &lastingEvent{Key: deref(st.SourceKey), Change: "damage", Targets: []string{who.ID}, Amount: amount, DType: dtype, Effects: []string{st.ID}},
	}); err != nil {
		return err
	}
	if hit.Applied {
		if err := s.afterDamage(ctx, c, who, damageLanded{key: deref(st.SourceKey), pending: p, hit: hit}); err != nil {
			return err
		}
	}
	if st.TriggersFired+1 >= derefInt32(st.TriggerMaxTriggers) {
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return fmt.Errorf("list the combatants: %w", err)
		}
		return s.endEffectRows(ctx, c, cs, []playdb.CombatantState{st}, endBurned)
	}
	return nil
}

// openEndSaves opens the saving throws the effects on a combatant ask at the end of its
// turn, in a fixed order (the effect's id): the part of the turn that ended is not over until
// they are answered.
func (s *Service) openEndSaves(ctx context.Context, c *combatTx, current playdb.Combatant) error {
	rows, err := c.q.ListLastingEffects(ctx, c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the effects: %w", err)
	}
	rows = slices.DeleteFunc(rows, func(st playdb.CombatantState) bool { return st.CombatantID != current.ID || st.EndSaveAbility == nil })
	slices.SortFunc(rows, func(a, b playdb.CombatantState) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
	group := newGroup()
	for _, st := range rows {
		if _, err := s.openEffectSave(ctx, c, group, st, current, combat.PhaseEnd, false); err != nil {
			return err
		}
	}
	return nil
}

// endSaveOpen says a saving throw of an effect is open: the turn that ended waits for it.
func (s *Service) endSaveOpen(ctx context.Context, c *combatTx) (bool, error) {
	open, err := openWindowsOf(ctx, c)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(open, func(w playdb.ReactionWindow) bool { return reaction.Kind(w.Kind) == reaction.EffectSave }), nil
}

// releaseHeldTurn passes the turn that waited for a saving throw once every window is
// answered or closed: it runs after every change of a combat. The turn is held when the combat
// runs, nobody in the turn still acts, some member's part ended and no reaction window is open.
func (s *Service) releaseHeldTurn(ctx context.Context, c *combatTx) error {
	if c.enc.ID == "" || c.enc.Status != statusActive {
		return nil
	}
	cs, err := c.q.ListCombatants(ctx, c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the combatants: %w", err)
	}
	if slices.ContainsFunc(cs, func(o playdb.Combatant) bool { return o.TurnState == turnActing }) {
		return nil
	}
	members := turnMembers(cs)
	if len(members) == 0 {
		return nil
	}
	open, err := openWindowsOf(ctx, c)
	if err != nil {
		return err
	}
	if len(open) > 0 {
		return nil
	}
	next, newRound, ok := nextTurnGroup(cs, members[0].ID, "")
	if !ok {
		return nil
	}
	round := c.enc.Round
	if newRound {
		round++
	}
	if err := startTurn(ctx, c, next, round); err != nil {
		return err
	}
	return insertEvent(ctx, c, eventTurnEnded, &c.actorUserID, nil, map[string]any{"from": members[0].ID, "to": next[0], "round": round, "after_save": true})
}

// turnTempHP gives the temporary hit points an effect gives at the start of a turn: they do not
// stack, so the creature keeps the larger of what it had and what the effect gives (SRD 5.1,
// "Temporary Hit Points").
func (s *Service) turnTempHP(ctx context.Context, c *combatTx, who playdb.Combatant, st playdb.CombatantState, amount int, round int32) error {
	t, err := s.readFxTarget(ctx, c, who)
	if err != nil {
		return err
	}
	if t.temp >= amount {
		return nil
	}
	var hit castHit
	v, err := s.giveTempHP(ctx, c, t, amount, &hit)
	if err != nil {
		return err
	}
	if v != nil {
		c.told = append(c.told, v)
	}
	return insertEvent(ctx, c, eventLastingTriggered, &c.actorUserID, nil, actionEvent{
		Round: round, Secret: s.effectHiddenFrom(st, []playdb.Combatant{who}), Actor: who.ID, Target: who.ID, Key: deref(st.SourceKey),
		Lasting: &lastingEvent{Key: deref(st.SourceKey), Change: "temp_hp", Targets: []string{who.ID}, Amount: clamp32(amount, 0, 100), Effects: []string{st.ID}},
	})
}

// clearTempHP takes the temporary hit points off a combatant: what an effect gave goes when it ends.
func (s *Service) clearTempHP(ctx context.Context, c *combatTx, who playdb.Combatant) error {
	t, err := s.readFxTarget(ctx, c, who)
	if err != nil {
		return err
	}
	if t.temp == 0 {
		return nil
	}
	none := int32(0)
	if holdsHP(who) {
		before := hpOf(who)
		if err := c.q.SetCombatantHitPoints(ctx, playdb.SetCombatantHitPointsParams{ID: who.ID, HpCurrent: &before.HP, HpTemp: &none, Defeated: before.Defeated}); err != nil {
			return fmt.Errorf("take the temporary hit points off: %w", err)
		}
		return nil
	}
	_, after, err := s.vitalsOf(ctx, c, who.CharacterID, &playv1.AdjustCharacterVitalsRequest{HitPointsTemporary: &none})
	if err != nil {
		return err
	}
	if after != nil {
		c.told = append(c.told, after)
	}
	return nil
}
