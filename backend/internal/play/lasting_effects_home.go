package play

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// Where an effect lives (RN-22). An effect belongs to the character it is on: in a running
// combat its record is a row of combatant_states (the clock in rounds, read by the combatant of
// the character); out of a combat it is a row of character_effects (the clock in game time).
// The record changes home when a combat takes the character in or lets it go, whole and with
// the same id in one transaction, so there is never a copy of it. A round is 6 seconds
// (SRD 5.1, "The Order of Combat"), in a combat and out of it.

// backedByCharacter says the combatant is a character (a player's, or an NPC with a sheet):
// its effects outlive the combat. A monster and a creature are the combat's alone.
func backedByCharacter(a playdb.Combatant) bool {
	return a.CharacterID != "" && !isCreature(a) && (a.Kind == kindPlayer || a.Kind == kindNPC)
}

// castCharacterEffects puts the effect of a spell that lasts on the characters it was cast
// on, outside a combat. The duration is the spell's, in game time; the caster's concentration
// holds it. A spell that asks a saving throw has none here: the master adds it where it took hold.
func (s *Service) castCharacterEffects(ctx context.Context, c *combatTx, plan castPlan) error {
	content, err := s.contentOf(ctx, c)
	if err != nil {
		return err
	}
	def, ok := content.CombatSpellEffect(plan.spell)
	if !ok || def.Applies != "all" || len(plan.g.targets) == 0 {
		return nil
	}
	body, err := json.Marshal(append([]rules.EffectModifier{}, def.Modifiers...))
	if err != nil {
		return fmt.Errorf("encode the modifiers: %w", err)
	}
	var seconds *int32
	kind := rules.EffectDurationUntilDismissed
	switch {
	case plan.osp.DurationSeconds > 0:
		seconds, kind = new(clamp32(plan.osp.DurationSeconds, 1, 1<<30)), rules.EffectDurationRounds
	case def.Concentration:
		kind = rules.EffectDurationConcentration
	}
	caster := plan.g.caster.ID
	for _, t := range plan.g.targets {
		p := playdb.InsertCharacterEffectParams{
			CampaignID: c.session.CampaignID, CharacterID: t.ID, SourceCharacterID: &caster, GroupID: plan.row.ID, SourceKey: plan.spell,
			SourceKind: "spell", Concentration: def.Concentration, ConditionKeys: append([]string{}, def.Conditions...), Modifiers: body,
			DurationKind: kind, SecondsLeft: seconds, PlayerVisible: true, Audience: "all", CreatedAt: c.now,
		}
		if def.Visibility == rules.EffectVisibilityOwner {
			p.Audience = audienceOwner
		}
		if _, err := c.q.InsertCharacterEffect(ctx, p); err != nil {
			return fmt.Errorf("put the effect on the character: %w", err)
		}
	}
	return nil
}

// endCharacterEffectsOfCast takes away the effects a cast left on characters out of a combat
// (the cast ended, or its concentration did).
func (s *Service) endCharacterEffectsOfCast(ctx context.Context, c *combatTx, castID string) error {
	if _, err := c.q.DeleteCharacterEffectsOfGroup(ctx, playdb.DeleteCharacterEffectsOfGroupParams{CampaignID: c.session.CampaignID, GroupID: castID}); err != nil {
		return fmt.Errorf("end the effects of a cast: %w", err)
	}
	return nil
}

// advanceGameTime moves game time on by seconds (the master's: a cast that took time): what
// had less time left ends, with the cast that held it.
func (s *Service) advanceGameTime(ctx context.Context, c *combatTx, seconds int32) error {
	if seconds <= 0 {
		return nil
	}
	rows, err := c.q.AdvanceCharacterEffects(ctx, playdb.AdvanceCharacterEffectsParams{CampaignID: c.session.CampaignID, Column2: seconds})
	if err != nil {
		return fmt.Errorf("move game time on: %w", err)
	}
	for _, r := range rows {
		if r.SecondsLeft == nil || *r.SecondsLeft > 0 {
			continue
		}
		if err := c.q.DeleteCharacterEffect(ctx, r.ID); err != nil {
			return fmt.Errorf("end an effect whose time ran out: %w", err)
		}
	}
	return nil
}

// effectsToCombat runs when characters join a combat: their effects come into it with the
// time they have left, in whole rounds (a partial round does not count: 59 seconds are 9
// rounds), counted from the start of the first round. The record is the same: it only moves.
func (s *Service) effectsToCombat(ctx context.Context, c *combatTx, all, added []playdb.Combatant) error {
	var ids []string
	for _, a := range added {
		if backedByCharacter(a) {
			ids = append(ids, a.CharacterID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := c.q.ListCharacterEffects(ctx, ids)
	if err != nil {
		return fmt.Errorf("list the characters' effects: %w", err)
	}
	var touched []string
	for _, r := range rows {
		i := slices.IndexFunc(added, func(a playdb.Combatant) bool { return a.CharacterID == r.CharacterID && backedByCharacter(a) })
		if i < 0 {
			continue
		}
		target := added[i]
		var casterCombatant *playdb.Combatant
		if r.SourceCharacterID != nil {
			if j := slices.IndexFunc(all, func(a playdb.Combatant) bool { return a.CharacterID == *r.SourceCharacterID && backedByCharacter(a) }); j >= 0 {
				casterCombatant = &all[j]
			}
		}
		dur := durationSpec{Kind: r.DurationKind}
		if r.SecondsLeft != nil {
			dur.Rounds = roundsOfSeconds(*r.SecondsLeft)
		}
		round, who, phase := endsOf(c.enc, all, dur, casterCombatant, target)
		if r.DurationKind == rules.EffectDurationRounds && r.SecondsLeft == nil {
			round, who, phase = nil, nil, nil
		}
		p := playdb.InsertLastingEffectParams{
			ID: &r.ID, EncounterID: c.enc.ID, CombatantID: target.ID, StartedRound: max(c.enc.Round, 1), CreatedAt: r.CreatedAt,
			GroupID: &r.GroupID, SourceKey: &r.SourceKey, SourceKind: &r.SourceKind, Concentration: r.Concentration,
			ConditionKeys: r.ConditionKeys, Modifiers: r.Modifiers, DurationKind: &r.DurationKind,
			PlayerVisible: r.PlayerVisible, Audience: r.Audience, EndsRound: round, EndsCombatantID: who, EndsPhase: phase,
			EndSaveAbility: r.EndSaveAbility, StartSaveAbility: r.StartSaveAbility, SaveDc: r.SaveDc, OnFailEffect: r.OnFailEffect,
			FollowsKey: r.FollowsKey, TriggerDice: r.TriggerDice, TriggerDamageType: r.TriggerDamageType, TriggerMaxTriggers: r.TriggerMaxTriggers,
			PlayerLabel: r.PlayerLabel,
		}
		if casterCombatant != nil {
			p.SourceID = &casterCombatant.ID
		}
		if _, err := c.q.InsertLastingEffect(ctx, p); err != nil {
			return fmt.Errorf("bring the effect into the combat: %w", err)
		}
		if err := c.q.DeleteCharacterEffect(ctx, r.ID); err != nil {
			return fmt.Errorf("take the effect off the character: %w", err)
		}
		touched = append(touched, target.ID)
	}
	// Every character that joined is worked out again: its exhaustion comes from its vitals.
	for _, a := range added {
		if backedByCharacter(a) {
			touched = append(touched, a.ID)
		}
	}
	slices.Sort(touched)
	return s.refreshCombatants(ctx, c, slices.Compact(touched)...)
}

// effectsToCharacters runs when a combat ends, or when a character leaves it: the effects on
// the characters go back to game time with the rounds they had left, 6 seconds each, and keep
// running. One that was to end at a turn that never comes (until the end of a turn, until the
// start of a turn) ends with the combat. only limits it to those combatants (a leaver).
func (s *Service) effectsToCharacters(ctx context.Context, c *combatTx, cs, only []playdb.Combatant) error {
	rows, err := c.q.ListLastingEffects(ctx, c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the effects: %w", err)
	}
	for _, st := range rows {
		i := slices.IndexFunc(cs, func(a playdb.Combatant) bool { return a.ID == st.CombatantID })
		if i < 0 || !backedByCharacter(cs[i]) || (only != nil && !slices.ContainsFunc(only, func(a playdb.Combatant) bool { return a.ID == st.CombatantID })) {
			continue
		}
		kind := deref(st.DurationKind)
		if kind == rules.EffectDurationUntilStartOfTurnOf || kind == rules.EffectDurationUntilEndOfTurnOf {
			continue // its turn never comes: it ends with the combat
		}
		var seconds *int32
		if st.EndsRound != nil {
			left := max(*st.EndsRound-max(c.enc.Round, 1), 0)
			if left == 0 {
				continue
			}
			seconds = new(left * rules.SecondsPerRound)
		}
		p := playdb.InsertCharacterEffectParams{
			ID: &st.ID, CampaignID: c.session.CampaignID, CharacterID: cs[i].CharacterID, GroupID: deref(st.GroupID), SourceKey: deref(st.SourceKey),
			SourceKind: deref(st.SourceKind), Concentration: st.Concentration, ConditionKeys: st.ConditionKeys, Modifiers: st.Modifiers,
			DurationKind: kind, SecondsLeft: seconds, EndSaveAbility: st.EndSaveAbility, StartSaveAbility: st.StartSaveAbility, SaveDc: st.SaveDc,
			OnFailEffect: st.OnFailEffect, FollowsKey: st.FollowsKey, TriggerDice: st.TriggerDice, TriggerDamageType: st.TriggerDamageType,
			TriggerMaxTriggers: st.TriggerMaxTriggers, TriggersFired: st.TriggersFired, PlayerVisible: st.PlayerVisible, Audience: st.Audience,
			PlayerLabel: st.PlayerLabel, CreatedAt: st.CreatedAt,
		}
		if st.SourceID != nil {
			if j := slices.IndexFunc(cs, func(a playdb.Combatant) bool { return a.ID == *st.SourceID }); j >= 0 && backedByCharacter(cs[j]) {
				p.SourceCharacterID = &cs[j].CharacterID
			}
		}
		if _, err := c.q.InsertCharacterEffect(ctx, p); err != nil {
			return fmt.Errorf("give the effect back to the character: %w", err)
		}
		if err := c.q.DeleteLastingEffect(ctx, st.ID); err != nil {
			return fmt.Errorf("take the effect off the combatant: %w", err)
		}
	}
	return nil
}
