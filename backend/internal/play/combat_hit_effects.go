package play

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

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
	if h.attack.Spell { // a spell attack is no weapon hit
		return nil
	}
	var out []partRecord
	for _, r := range hitRowsOf(h.states, h.attacker.ID) {
		switch {
		case h.hitTriggerHolds(r):
			out = append(out, h.hitEffectPart(r))
		case r.mod.Trigger == rules.HitTriggerNextMeleeHit: // the weapon is the mismatch: listed off, the effect keeps waiting
			out = append(out, h.hitEffectOffPart(r))
		}
	}
	return out
}

// meleeHit says the hit is a melee one: a thrown weapon at range is a ranged attack.
func (h hitFacts) meleeHit() bool { return h.attack.Melee && !h.thrownAtRange() }

// hitTriggerHolds says the hit is one the row's trigger rides on.
func (h hitFacts) hitTriggerHolds(r hitRow) bool {
	switch r.mod.Trigger {
	case rules.HitTriggerNextMeleeHit:
		return h.meleeHit()
	case rules.HitTriggerMarkedTarget:
		return carriesMark(h.states[h.target.ID], r.state)
	}
	return true
}

// hitEffectPart is the damage of the row as a part. It keeps the spell's name and its dice; Hide says the effect is
// not the other players' to read, and the view of each viewer hides what he may not read (hitPartForOthers).
func (h hitFacts) hitEffectPart(r hitRow) partRecord {
	key := conv.OrZero(r.state.SourceKey)
	return partRecord{
		Key: key, Label: h.names(key), Kind: partAuto, Count: clamp32(r.mod.Dice, 0, 100), Sides: clamp32(r.mod.Die, 0, 100),
		DamageType: cmp.Or(r.mod.DamageType, h.attack.DamageType), Source: key, Available: true, Table: tableHitEffect, Effect: r.state.ID,
		Hide: !r.state.PlayerVisible || r.state.Audience != audienceAll, Note: hitEffectNote(r.mod, h.names),
	}
}

// offMeleeReason is why a next_melee_hit is listed off on a ranged weapon.
const offMeleeReason = "só num golpe corpo a corpo. Continua esperando."

// hitEffectOffPart is the part of a next_melee_hit that this weapon does not carry: listed with its dice and the
// reason, unavailable, never automatic, never rolled (an extra that is not selected does not count) and never
// marked, so the row stays unspent. Its dice are shown as they are: a critical hit does not double what is not rolled.
func (h hitFacts) hitEffectOffPart(r hitRow) partRecord {
	sp := h.hitEffectPart(r)
	sp.Kind, sp.Available, sp.Reason, sp.Note, sp.NoCrit = partExtra, false, offMeleeReason, "", true
	return sp
}

// isOffHitPart says a part is the off part of a spell's hit effect.
func isOffHitPart(sp partRecord) bool { return sp.Table == tableHitEffect && sp.Kind == partExtra }

// onlyOffRows says the parts are off rows and nothing else (and at least one).
func onlyOffRows(parts []partRecord) bool {
	return len(parts) > 0 && !slices.ContainsFunc(parts, func(sp partRecord) bool { return !isOffHitPart(sp) })
}

// plainToOthers says the players who are not the attacker's read the hit as one with no parts: its weapon part is
// flagged Plain, so the off row it was listed with cannot tell them the caster has an effect waiting.
func plainToOthers(parts []partRecord) bool {
	return slices.ContainsFunc(parts, func(sp partRecord) bool { return sp.Key == partWeapon && sp.Plain })
}

// hitEffectNote is the effect's own words on its part, built from the modifier: what the trigger means when it says
// something, the save of the target and the conditions of a failure with their length. A modifier with no save is
// the trigger's sentence alone (conditions without a save are never given, see useHitEffect).
func hitEffectNote(m rules.EffectModifier, names func(string) string) string {
	var note string
	switch m.Trigger {
	case rules.HitTriggerNextHit, rules.HitTriggerNextMeleeHit:
		note = "O efeito acaba com este golpe."
	case rules.HitTriggerEachHit:
		note = "O efeito vale em cada golpe enquanto durar."
	case rules.HitTriggerMarkedTarget:
		note = "O efeito vale nos golpes contra o alvo marcado."
	}
	ability := cmp.Or(abilityPT[m.Save], names("ability:"+m.Save))
	if m.Save == "" || ability == "" { // a save that cannot be named is left out, never printed with a hole
		return note
	}
	save := "Depois do dano, o alvo faz um teste de resistência de " + ability
	var conds []string
	for _, k := range m.OnFailConditions {
		if n := names(k); n != "" {
			conds = append(conds, n)
		}
	}
	if len(conds) == 0 {
		return strings.TrimSpace(note + " " + save + ".")
	}
	length := "até o fim do próximo turno dele"
	if m.ConditionDuration == rules.ConditionDurationSpellEnd {
		length = "enquanto a magia durar"
	}
	return strings.TrimSpace(note + " " + fmt.Sprintf("%s; se falhar, fica %s %s.", save, joinAnd(conds), length))
}

// joinAnd lists the names in Portuguese: "Cego", "Cego e Caído", "Cego, Caído e Surdo".
func joinAnd(items []string) string {
	n := len(items)
	if n == 0 {
		return ""
	}
	if n == 1 {
		return items[0]
	}
	return strings.Join(items[:n-1], ", ") + " e " + items[n-1]
}

// hitPartForOthers is the key, the label and the source of a spell's hit part as a player who is neither the master
// nor the attacker's owner reads it (RN-10): an effect the master hides, or that is the owner's, is "Outra fonte" with
// the row's id for a key, and nothing of the spell.
func hitPartForOthers(sp partRecord, label, source string) (string, string, string) {
	if sp.Hide {
		return sp.Effect, otherSourceTx, ""
	}
	return sp.Key, label, source
}

// liveHitParts takes out of the parts the damage of an effect that was used or ended since the hit offered it, and, for a
// next_hit or a next_melee_hit, the damage an earlier attack of the same attacker still waits to roll: two hits of one
// turn wait for their damage, and the effect goes to the earlier attack whichever damage is rolled first.
func (s *Service) liveHitParts(ctx context.Context, c *combatTx, p playdb.PendingDamage, parts []partRecord) ([]partRecord, error) {
	var open []playdb.PendingDamage
	for i, sp := range parts {
		if sp.Table != tableHitEffect || sp.Removed {
			continue
		}
		gone, err := s.hitPartGone(ctx, c, p, sp, &open)
		if err != nil {
			return nil, err
		}
		parts[i].Removed = gone
	}
	return parts, nil
}

// hitPartGone says the part of a hit effect is out of this damage: its row is used or ended, or an earlier attack waits
// for it. open is the open pending damages, read once the first time they are needed.
func (s *Service) hitPartGone(ctx context.Context, c *combatTx, p playdb.PendingDamage, sp partRecord, open *[]playdb.PendingDamage) (bool, error) {
	st, err := c.q.GetLastingEffect(ctx, playdb.GetLastingEffectParams{ID: sp.Effect, EncounterID: c.enc.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("find the effect of the part: %w", err)
	}
	if st.SpentAt != nil {
		return true, nil
	}
	if mod, ok := hitModifierOf(effectModifiers(st)); !ok || !consumesHit(mod.Trigger) {
		return false, nil
	}
	if *open == nil {
		if *open, err = c.q.ListOpenPendingDamages(ctx, c.enc.ID); err != nil {
			return false, fmt.Errorf("list the pending damages: %w", err)
		}
	}
	return waitsForEarlierAttack(*open, p, sp.Effect), nil
}

// waitsForEarlierAttack says an attack of the same attacker, made before p and still waiting for its damage, offers the
// effect row too: the earlier attack uses it.
func waitsForEarlierAttack(open []playdb.PendingDamage, p playdb.PendingDamage, effect string) bool {
	return slices.ContainsFunc(open, func(o playdb.PendingDamage) bool {
		before := o.CreatedAt.Before(p.CreatedAt) || o.CreatedAt.Equal(p.CreatedAt) && o.ID < p.ID
		waiting := o.Status == pendingAwaitingRoll || o.Status == pendingAwaitingReaction
		return o.ID != p.ID && before && waiting && conv.OrZero(o.AttackerID) == conv.OrZero(p.AttackerID) &&
			slices.ContainsFunc(readParts(o.Parts), func(sp partRecord) bool {
				return sp.Table == tableHitEffect && sp.Effect == effect && !sp.Removed && !isOffHitPart(sp)
			})
	})
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

// useHitEffect uses one effect. A next_hit that is already spent (the replay of a roll a window held, after another roll
// used it) is not used again, and its save is not rolled again.
func (s *Service) useHitEffect(ctx context.Context, c *combatTx, u hitUse, st playdb.CombatantState, ev *actionEvent) error {
	mod, ok := hitModifierOf(effectModifiers(st))
	if !ok {
		return nil
	}
	if consumesHit(mod.Trigger) {
		if st.SpentAt != nil {
			return nil
		}
		if err := c.q.SetLastingEffectSpent(ctx, playdb.SetLastingEffectSpentParams{ID: st.ID, SpentAt: &c.now}); err != nil {
			return fmt.Errorf("use the effect: %w", err)
		}
		ev.EffectsSpent = append(ev.EffectsSpent, st.ID)
	}
	if mod.Save == "" {
		return nil
	}
	return s.hitSave(ctx, c, u, st, mod, ev)
}

// hitSave has the target make the save of the spell against the DC kept on the row, as a rider's save is made
// (rollRiderSave: Magic Resistance, Legendary Resistance, the held rolls), writes the line of the roll like a
// monk's rider (the master, the attacker's player and the target's own player read it, no other player: RN-10) and
// on a failure gives it the conditions: until the end of its next turn, or as long as the spell lasts. The rows and
// the lines are noted in the event, so that the undo of the damage takes them away.
func (s *Service) hitSave(ctx context.Context, c *combatTx, u hitUse, st playdb.CombatantState, mod rules.EffectModifier, ev *actionEvent) error {
	dc := defaultHitSaveDC
	if st.SaveDc != nil {
		dc = *st.SaveDc
	}
	re := riderEvent{Choice: choiceApply, Result: riderSaved, DC: dc, OwnersOnly: true}
	saved, err := s.rollRiderSave(ctx, c, u.m, u.target, mod.Save, mod.OnFailConditions, dc, &re)
	if err != nil {
		return err
	}
	var conds []string
	if !saved {
		if conds, err = s.hitConditions(ctx, c, u.target, mod.OnFailConditions); err != nil {
			return err
		}
		if len(conds) > 0 {
			re.Result = riderFailed
		}
	}
	if err := s.noteHitSave(ctx, c, u, st, mod, conds, &re); err != nil {
		return err
	}
	if err := s.hitSaveLine(ctx, c, u, st, &re, ev); err != nil || len(conds) == 0 {
		return err
	}
	spec, err := s.hitConditionsSpec(ctx, c, u, st, mod, conds)
	if err != nil {
		return err
	}
	if err := s.endHitConditions(ctx, c, u.cs, u.target, st); err != nil {
		return err
	}
	rows, line, err := s.putRiderEffect(ctx, c, u.cs, spec)
	if err != nil {
		return err
	}
	for _, r := range rows {
		ev.EffectsAdded = append(ev.EffectsAdded, r.ID)
	}
	if line != "" {
		ev.EffectLines = append(ev.EffectLines, line)
	}
	return nil
}

// noteHitSave puts on the line of the save what the viewers read besides the roll: the conditions a failed save gave and
// how long they last, and whether the spell is hidden from the other players.
func (s *Service) noteHitSave(ctx context.Context, c *combatTx, u hitUse, st playdb.CombatantState, mod rules.EffectModifier, conds []string, re *riderEvent) error {
	re.HiddenSource = !st.PlayerVisible || st.Audience != audienceAll
	if len(conds) == 0 {
		return nil
	}
	names, err := s.namerFor(ctx, c.tx, u.m.CampaignID)
	if err != nil {
		return err
	}
	re.ConditionName, re.Duration = hitConditionText(names, conds), hitConditionDuration(mod)
	return nil
}

// hitConditionText is the names of the conditions a failed save gave, for the line of the save.
func hitConditionText(names func(string) string, conds []string) string {
	out := make([]string, len(conds))
	for i, k := range conds {
		out[i] = names(k)
	}
	return strings.Join(out, ", ")
}

// The words of how long the conditions of a failed save last when they hold as long as the spell; the neutral words are
// the ones a reader reads when the effect's source is hidden from them (RN-10).
const (
	durationSpellEndTx  = "até o fim da magia"
	durationEffectEndTx = "até o fim do efeito"
)

// hitConditionDuration is how long the conditions of a failed save last, in the words of the line of the save.
func hitConditionDuration(mod rules.EffectModifier) string {
	if mod.ConditionDuration == rules.ConditionDurationSpellEnd {
		return durationSpellEndTx
	}
	return "até o fim do próximo turno dele"
}

// hitSaveLine writes the line of the save: the target's roll, the DC and what came of it, as the line of a rider.
func (s *Service) hitSaveLine(ctx context.Context, c *combatTx, u hitUse, st playdb.CombatantState, re *riderEvent, ev *actionEvent) error {
	re.EffectHidden = s.effectHiddenFrom(st, []playdb.Combatant{u.target})
	line, err := insertEventID(ctx, c, eventHitRider, &c.actorUserID, nil, actionEvent{
		Round: c.enc.Round, Secret: secretOf(u.attacker, u.target), Actor: u.attacker.ID, Target: u.target.ID, Key: conv.OrZero(st.SourceKey), Rider: re,
	})
	if err != nil {
		return err
	}
	ev.EffectLines = append(ev.EffectLines, line)
	return nil
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

// hitConditionsSpec is the effect a failed save puts on the target: the conditions, of the spell's casting. They last
// until the end of the target's next turn, or, when the modifier says spell_end, as the spell does: the end of the
// caster's concentration ends them.
func (s *Service) hitConditionsSpec(ctx context.Context, c *combatTx, u hitUse, st playdb.CombatantState, mod rules.EffectModifier, conds []string) (effectSpec, error) {
	key := conv.OrZero(st.SourceKey)
	spec := effectSpec{
		key: key, sourceKind: "spell", caster: &u.attacker, group: conv.OrZero(st.GroupID), targets: []playdb.Combatant{u.target},
		dur: durationSpec{Kind: rules.EffectDurationUntilEndOfTurnOf}, dc: st.SaveDc, visible: &st.PlayerVisible, audience: st.Audience,
		conditions: conds, modifiers: []rules.EffectModifier{},
	}
	if mod.ConditionDuration != rules.ConditionDurationSpellEnd {
		return spec, nil
	}
	content, err := s.contentOf(ctx, c)
	if err != nil {
		return effectSpec{}, err
	}
	spec.concentration, spec.dur = st.Concentration, durationSpec{Kind: rules.EffectDurationUntilDismissed}
	if def, ok := content.CombatSpellEffect(key); ok {
		spec.dur = spellEffectDuration(content, def, key, u.attacker)
	}
	return spec, nil
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
