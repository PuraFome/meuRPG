package play

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The damage a spell adds to the caster's weapon hits (a weapon_hit_damage modifier on an effect row of the
// caster, lasting_effects_hit.go). A weapon hit that the effect's trigger holds for offers it as an automatic part
// (never choosable: it is the spell's rider) whose key is the spell's; the dice are the row's, rolled like those of
// every other part and doubled by a critical hit. When the damage is rolled the effect is used: a next_hit or a
// next_melee_hit row is marked spent (spent_at, the master's undo of the damage gives it back through
// made.EffectsSpent), and a save with conditions is made by the target against the spell's DC. A miss never reaches
// this: the parts are offered by a hit. The row, its dice, its trigger and its save are never in a payload of
// another player's: the part carries the row's id and nothing else (RN-10).

// tableHitEffect is partRecord.Table of the part a spell's weapon_hit_damage makes; defaultHitSaveDC is the DC of a row
// that kept none (a caster with no spell save DC).
const (
	tableHitEffect   = "hit_effect"
	defaultHitSaveDC = int32(10)
)

// hitRow is a weapon_hit_damage of a combatant that has not been spent.
type hitRow struct {
	state playdb.CombatantState
	mod   rules.EffectModifier
}

// hitRowsOf are the weapon_hit_damage effects on a combatant that are not spent.
func hitRowsOf(states map[string][]playdb.CombatantState, id string) []hitRow {
	var out []hitRow
	for _, st := range effectsOn(states, id) {
		if m, ok := hitModifierOf(effectModifiers(st)); ok && st.SpentAt == nil {
			out = append(out, hitRow{state: st, mod: m})
		}
	}
	return out
}

// consumesHit says the trigger is used up by the hit that rides on it.
func consumesHit(trigger string) bool {
	return trigger == rules.HitTriggerNextHit || trigger == rules.HitTriggerNextMeleeHit
}

// carriesMark says the states of a creature hold the mark of the same casting as the row of the caster.
func carriesMark(onTarget []playdb.CombatantState, r playdb.CombatantState) bool {
	return slices.ContainsFunc(onTarget, func(o playdb.CombatantState) bool {
		return isEffect(o) && o.ID != r.ID && conv.OrZero(o.GroupID) == conv.OrZero(r.GroupID) && conv.OrZero(o.SourceKey) == conv.OrZero(r.SourceKey) &&
			conv.OrZero(o.SourceID) == conv.OrZero(r.SourceID)
	})
}

// hitEffectParts are the parts the attacker's spells add to this weapon hit.
func (h hitFacts) hitEffectParts() []partRecord {
	var out []partRecord
	for _, r := range hitRowsOf(h.states, h.attacker.ID) {
		if h.hitTriggerHolds(r) {
			out = append(out, h.hitEffectPart(r))
		}
	}
	return out
}

// hitTriggerHolds says the hit is one the row's trigger rides on.
func (h hitFacts) hitTriggerHolds(r hitRow) bool {
	switch r.mod.Trigger {
	case rules.HitTriggerNextMeleeHit:
		return h.attack.Melee && !h.thrownAtRange()
	case rules.HitTriggerMarkedTarget:
		return carriesMark(h.states[h.target.ID], r.state)
	}
	return true
}

// hitEffectPart is the damage of the row as a part. An effect the master hides is "Outra fonte" in the part.
func (h hitFacts) hitEffectPart(r hitRow) partRecord {
	key := conv.OrZero(r.state.SourceKey)
	label, source := h.names(key), key
	if !r.state.PlayerVisible {
		label, source = otherSourceTx, ""
	}
	return partRecord{
		Key: key, Label: label, Kind: partAuto, Count: clamp32(r.mod.Dice, 0, 100), Sides: clamp32(r.mod.Die, 0, 100),
		DamageType: cmp.Or(r.mod.DamageType, h.attack.DamageType), Source: source, Available: true, Table: tableHitEffect, Effect: r.state.ID,
	}
}

// liveHitParts takes out of the parts the damage of an effect that was used or ended since the hit offered it (two
// hits of one turn wait for their damage: the first one's roll uses the next hit).
func (s *Service) liveHitParts(ctx context.Context, c *combatTx, parts []partRecord) ([]partRecord, error) {
	for i, sp := range parts {
		if sp.Table != tableHitEffect || sp.Removed {
			continue
		}
		st, err := c.q.GetLastingEffect(ctx, playdb.GetLastingEffectParams{ID: sp.Effect, EncounterID: c.enc.ID})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && st.SpentAt != nil) {
			parts[i].Removed = true
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("find the effect of the part: %w", err)
		}
	}
	return parts, nil
}

// hitUse is what a damage roll needs to use the effects its parts came from.
type hitUse struct {
	m                authz.Membership
	cs               []playdb.Combatant
	attacker, target playdb.Combatant
}

// useHitEffects uses the effects whose damage the roll counted: the one that is used up is marked spent, noted in the
// event for the undo, and a save of the target is made.
func (s *Service) useHitEffects(ctx context.Context, c *combatTx, u hitUse, pr *partsRoll, ev *actionEvent) error {
	if pr == nil {
		return nil
	}
	for i, r := range pr.rolls {
		if !r.Counted || pr.parts[i].Table != tableHitEffect {
			continue
		}
		st, err := c.q.GetLastingEffect(ctx, playdb.GetLastingEffectParams{ID: pr.parts[i].Effect, EncounterID: c.enc.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("find the effect of the part: %w", err)
		}
		if err := s.useHitEffect(ctx, c, u, st, ev); err != nil {
			return err
		}
	}
	return nil
}

// useHitEffect uses one effect.
func (s *Service) useHitEffect(ctx context.Context, c *combatTx, u hitUse, st playdb.CombatantState, ev *actionEvent) error {
	mod, ok := hitModifierOf(effectModifiers(st))
	if !ok {
		return nil
	}
	if consumesHit(mod.Trigger) && st.SpentAt == nil {
		if err := c.q.SetLastingEffectSpent(ctx, playdb.SetLastingEffectSpentParams{ID: st.ID, SpentAt: &c.now}); err != nil {
			return fmt.Errorf("use the effect: %w", err)
		}
		ev.EffectsSpent = append(ev.EffectsSpent, st.ID)
	}
	if mod.Save == "" {
		return nil
	}
	return s.hitSave(ctx, c, u, st, mod)
}

// hitSave has the target make the save of the spell against the DC kept on the row, as a rider's save is made
// (rollRiderSave: Magic Resistance, Legendary Resistance, the held rolls), and on a failure gives it the
// conditions for as long as the spell lasts.
func (s *Service) hitSave(ctx context.Context, c *combatTx, u hitUse, st playdb.CombatantState, mod rules.EffectModifier) error {
	dc := defaultHitSaveDC
	if st.SaveDc != nil {
		dc = *st.SaveDc
	}
	var re riderEvent
	saved, err := s.rollRiderSave(ctx, c, u.m, u.target, mod.Save, mod.OnFailConditions, dc, &re)
	if err != nil || saved {
		return err
	}
	conds, err := s.hitConditions(ctx, c, u.target, mod.OnFailConditions)
	if err != nil || len(conds) == 0 {
		return err
	}
	spec, err := s.hitConditionsSpec(ctx, c, u, st, conds)
	if err != nil {
		return err
	}
	if err := s.endHitConditions(ctx, c, u.cs, u.target, st); err != nil {
		return err
	}
	return s.putRiderEffect(ctx, c, u.cs, spec)
}

// hitConditions are the conditions of the list the target is not immune to.
func (s *Service) hitConditions(ctx context.Context, c *combatTx, target playdb.Combatant, conds []string) ([]string, error) {
	states, err := s.readStates(ctx, c.tx, c.enc.ID)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, k := range conds {
		immune, err := c.immuneToCondition(ctx, target, k)
		if err != nil {
			return nil, err
		}
		if !immune && !combat.ImmuneTo(effectModsOf(states, target.ID), k) {
			out = append(out, k)
		}
	}
	return out, nil
}

// hitConditionsSpec is the effect a failed save puts on the target: the conditions, of the spell's casting, so that
// the end of the caster's concentration ends it.
func (s *Service) hitConditionsSpec(ctx context.Context, c *combatTx, u hitUse, st playdb.CombatantState, conds []string) (effectSpec, error) {
	key := conv.OrZero(st.SourceKey)
	content, err := s.contentOf(ctx, c)
	if err != nil {
		return effectSpec{}, err
	}
	dur := durationSpec{Kind: rules.EffectDurationUntilDismissed}
	if def, ok := content.CombatSpellEffect(key); ok {
		dur = spellEffectDuration(content, def, key, u.attacker)
	}
	return effectSpec{
		key: key, sourceKind: "spell", caster: &u.attacker, group: conv.OrZero(st.GroupID), targets: []playdb.Combatant{u.target}, dur: dur,
		concentration: st.Concentration, dc: st.SaveDc, visible: &st.PlayerVisible, audience: st.Audience,
		conditions: conds, modifiers: []rules.EffectModifier{},
	}, nil
}

// endHitConditions ends the conditions an earlier hit of the same casting left on the target: the new ones replace them.
func (s *Service) endHitConditions(ctx context.Context, c *combatTx, cs []playdb.Combatant, target playdb.Combatant, st playdb.CombatantState) error {
	rows, err := c.q.ListLastingEffects(ctx, c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the effects: %w", err)
	}
	old := slices.DeleteFunc(rows, func(o playdb.CombatantState) bool {
		return o.CombatantID != target.ID || len(o.ConditionKeys) == 0 || conv.OrZero(o.GroupID) != conv.OrZero(st.GroupID) || conv.OrZero(o.SourceKey) != conv.OrZero(st.SourceKey)
	})
	return s.endEffectRows(ctx, c, cs, old, endReplaced)
}
