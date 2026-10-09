package play

import (
	"context"
	"fmt"
	"slices"
	"strings"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// What the effects and the conditions take away from a turn (RN-22, SRD 5.1, Conditions): the
// actions and reactions of an incapacitated creature (Incapacitated, Paralyzed, Petrified,
// Stunned, Unconscious), its movement, and the actions and the movement of the lethargy a
// Haste leaves. The turn still ends: "Encerrar turno" is always there.

// cannotActCode says why a combatant cannot take actions, UNSPECIFIED when it can.
func cannotActCode(c playdb.Combatant) rulesv1.DisabledReasonCode {
	switch {
	case c.EffectNoAction:
		return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_EFFECT_LETHARGY
	case (combat.Creature{Conditions: c.Conditions}).Incapacitated():
		return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_INCAPACITATED
	}
	return rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_UNSPECIFIED
}

// explainEffectReason writes, on every disabled line of the turn, the sentence of the reason an
// effect gives: without the cause for a player (RN-20: "Você não pode agir.", never who paralyzed
// them), with it for the master.
func (s *Service) explainEffectReason(d *encounterData, who playdb.Combatant, v combatViewer, o *rulesv1.TurnOptions, code rulesv1.DisabledReasonCode, names func(string) string) {
	var player, master string
	switch code {
	case rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_INCAPACITATED:
		player = "Você não pode agir."
		master = "Não pode agir: " + joinNames(names, incapacitatingOf(who.Conditions))
	case rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_EFFECT_LETHARGY:
		player = "Indisponível: a letargia."
		master = player
		for _, st := range effectsOn(statesOf(d.states), who.ID) {
			if st.FollowsKey != nil {
				player = "Indisponível: a letargia da " + names(*st.FollowsKey) + "."
				master = player
			}
		}
	default:
		return
	}
	for _, r := range reasonsOf(o) {
		r.TextPt = player
		if v.master {
			r.TextMasterPt = master
		}
	}
}

// reasonsOf are the reasons of the disabled lines of a turn.
func reasonsOf(o *rulesv1.TurnOptions) []*rulesv1.DisabledReason {
	var out []*rulesv1.DisabledReason
	for _, a := range o.GetAttacks() {
		out = append(out, a.GetReason())
	}
	for _, sp := range o.GetSpells() {
		out = append(out, sp.GetReason())
	}
	for _, a := range append(append([]*rulesv1.ActionOption{}, o.GetStandardActions()...), o.GetFeatureActions()...) {
		out = append(out, a.GetReason())
	}
	return out
}

func incapacitatingOf(conds []string) []string {
	var out []string
	for _, k := range conds {
		if (combat.Creature{Conditions: []string{k}}).Incapacitated() {
			out = append(out, k)
		}
	}
	return out
}

func joinNames(names func(string) string, keys []string) string {
	var out []string
	for _, k := range keys {
		out = append(out, names(k))
	}
	return strings.Join(out, ", ")
}

// effectTurnInfo fills what the effects say of the turn of a combatant: the notes of what is
// taken away and the extra action Velocidade gives. A player reads it for their own character.
func (s *Service) effectTurnInfo(d *encounterData, who playdb.Combatant, v combatViewer, res *playv1.GetTurnOptionsResponse, names func(string) string) {
	if !v.master && !v.owns(who) {
		return
	}
	if who.EffectNoAction {
		note := &playv1.EffectNote{TextPt: "O efeito acabou. Você não pode se mover nem agir até depois do seu próximo turno."}
		for _, st := range effectsOn(statesOf(d.states), who.ID) {
			if st.FollowsKey != nil {
				note.TextPt = "A " + names(*st.FollowsKey) + " acabou. Você não pode se mover nem agir até depois do seu próximo turno."
			}
		}
		res.EffectNotes = append(res.EffectNotes, note)
	}
	for _, k := range who.Conditions {
		var text string
		switch {
		case (combat.Creature{Conditions: []string{k}}).Incapacitated() && cannotMove(k):
			text = "Você está " + names(k) + ". Não age nem se move neste turno."
		case (combat.Creature{Conditions: []string{k}}).Incapacitated():
			text = "Você está " + names(k) + ". Não age neste turno."
		case k == "condition:restrained":
			text = "Você está " + names(k) + ". Deslocamento 0. Ataques contra você têm vantagem; os seus têm desvantagem."
		case k == "condition:grappled":
			text = "Você está " + names(k) + ". Deslocamento 0."
		}
		if text != "" {
			res.EffectNotes = append(res.EffectNotes, &playv1.EffectNote{TextPt: text})
		}
	}
	if opt := extraActionOf(d, who); opt != nil {
		res.ExtraAction = opt
	}
}

// extraActionOf is the extra action an effect gives a combatant (Velocidade, SRD 5.1: "an
// additional action on each of its turns", only Attack (one weapon attack), Dash, Disengage, Hide
// or Use an Object), or nil when it has none.
func extraActionOf(d *encounterData, who playdb.Combatant) *playv1.ExtraActionOption {
	for _, st := range effectsOn(statesOf(d.states), who.ID) {
		for _, m := range effectModifiers(st) {
			if m.Kind != rules.ModifierExtraAction {
				continue
			}
			out := &playv1.ExtraActionOption{
				Available: !who.ExtraActionUsed && !who.EffectNoAction && !(combat.Creature{Conditions: who.Conditions}).Incapacitated(),
				LabelPt:   "Ação extra (" + stateName(st) + ")", AllowedActions: m.Allowed,
				AllowedTextPt: "Só: Atacar (uma arma), Disparada, Desengajar, Esconder, Usar um objeto",
			}
			if who.ExtraActionUsed {
				out.ReasonPt = "A ação extra deste turno já foi usada."
			}
			return out
		}
	}
	return nil
}

// stateName is the Portuguese name of an effect's source, for what the turn calls it.
func stateName(st playdb.CombatantState) string {
	if deref(st.SourceKey) == "spell:haste" {
		return "Velocidade"
	}
	return "efeito"
}

// cannotMove says whether a condition takes the speed away (SRD 5.1, Conditions: Grappled,
// Paralyzed, Petrified, Restrained, Stunned and Unconscious: speed 0 or no movement).
func cannotMove(k string) bool {
	switch k {
	case "condition:paralyzed", "condition:petrified", "condition:stunned", "condition:unconscious",
		"condition:grappled", "condition:restrained":
		return true
	}
	return false
}

// endEffectsOfLeaver ends the effects a combatant leaving the fight carries or holds by
// concentration, before its row goes.
func (s *Service) endEffectsOfLeaver(ctx context.Context, c *combatTx, cs []playdb.Combatant, who playdb.Combatant) error {
	// A character keeps what is on it: its effects go back to game time.
	if err := s.effectsToCharacters(ctx, c, cs, []playdb.Combatant{who}); err != nil {
		return err
	}
	rows, err := c.q.ListLastingEffects(ctx, c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the effects: %w", err)
	}
	rows = slices.DeleteFunc(rows, func(st playdb.CombatantState) bool {
		return st.CombatantID != who.ID && !(st.Concentration && deref(st.SourceID) == who.ID)
	})
	return s.endEffectRows(ctx, c, cs, rows, endLeft)
}

// mustHaveExtraAction refuses an extra action the combatant does not have, has used, or may not
// spend on the action (SRD 5.1, Haste: one weapon attack, Dash, Disengage, Hide or Use an
// Object; never a spell).
func (s *Service) mustHaveExtraAction(ctx context.Context, c *combatTx, who playdb.Combatant, action string) error {
	rows, err := c.q.ListLastingEffects(ctx, c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the effects: %w", err)
	}
	for _, st := range effectsOn(statesOf(rows), who.ID) {
		for _, m := range effectModifiers(st) {
			if m.Kind != rules.ModifierExtraAction {
				continue
			}
			switch {
			case who.ExtraActionUsed:
				return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_EXTRA_ACTION_UNAVAILABLE, "the extra action of this turn is used")
			case who.EffectNoAction || cannotActCode(who) != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_UNSPECIFIED:
				return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CANNOT_ACT, "the combatant cannot act now")
			case !slices.Contains(m.Allowed, strings.TrimPrefix(action, "standard:")):
				return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_EXTRA_ACTION_UNAVAILABLE, "the extra action does not take this action")
			}
			return nil
		}
	}
	return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_EXTRA_ACTION_UNAVAILABLE, "the combatant has no extra action")
}

// reconcileConditions settles the conditions the master set against the effects: one an effect
// leaves, taken off by hand, ends the effects that left it; exhaustion is never set as a
// condition (it has levels: SetExhaustion). It returns the set to write and the combatant as
// the ended effects left it.
func (s *Service) reconcileConditions(ctx context.Context, c *combatTx, target playdb.Combatant, conditions []string) ([]string, playdb.Combatant, error) {
	conditions = slices.DeleteFunc(slices.Clone(conditions), func(k string) bool { return k == conditionExhaustion })
	if slices.Contains(target.Conditions, conditionExhaustion) {
		conditions = append(conditions, conditionExhaustion)
	}
	var gone []string
	for _, k := range target.EffectConditions {
		if !slices.Contains(conditions, k) {
			gone = append(gone, k)
		}
	}
	if len(gone) == 0 {
		return conditions, target, nil
	}
	rows, err := c.q.ListLastingEffects(ctx, c.enc.ID)
	if err != nil {
		return nil, target, fmt.Errorf("list the effects: %w", err)
	}
	rows = slices.DeleteFunc(rows, func(st playdb.CombatantState) bool {
		return st.CombatantID != target.ID || !slices.ContainsFunc(st.ConditionKeys, func(k string) bool { return slices.Contains(gone, k) })
	})
	cs, err := c.q.ListCombatants(ctx, c.enc.ID)
	if err != nil {
		return nil, target, fmt.Errorf("list the combatants: %w", err)
	}
	if err := s.endEffectRows(ctx, c, cs, rows, endByMaster); err != nil {
		return nil, target, err
	}
	cs, err = c.q.ListCombatants(ctx, c.enc.ID)
	if err != nil {
		return nil, target, fmt.Errorf("list the combatants: %w", err)
	}
	if i := slices.IndexFunc(cs, func(o playdb.Combatant) bool { return o.ID == target.ID }); i >= 0 {
		target = cs[i]
	}
	return conditions, target, nil
}

// conditionExhaustion is the condition the levels of exhaustion leave on a combatant.
const conditionExhaustion = "condition:exhaustion"

// restoreEffects puts back the effects an undone action ended, with the id and the clock they had.
func (s *Service) restoreEffects(ctx context.Context, c *combatTx, rows []playdb.CombatantState) error {
	var touched []string
	for _, st := range rows {
		p := playdb.InsertLastingEffectParams{
			ID: &st.ID, EncounterID: st.EncounterID, CombatantID: st.CombatantID, StartedRound: st.StartedRound, CreatedAt: st.CreatedAt,
			GroupID: st.GroupID, SourceKey: st.SourceKey, SourceKind: st.SourceKind, Concentration: st.Concentration, ConditionKeys: st.ConditionKeys,
			Modifiers: st.Modifiers, DurationKind: st.DurationKind, PlayerVisible: st.PlayerVisible, Audience: st.Audience,
			SourceID: st.SourceID, EndsCombatantID: st.EndsCombatantID, EndsPhase: st.EndsPhase, EndsRound: st.EndsRound,
			EndSaveAbility: st.EndSaveAbility, StartSaveAbility: st.StartSaveAbility, SaveDc: st.SaveDc, OnFailEffect: st.OnFailEffect,
			FollowsKey: st.FollowsKey, TriggerDice: st.TriggerDice, TriggerDamageType: st.TriggerDamageType, TriggerMaxTriggers: st.TriggerMaxTriggers,
			PlayerLabel: st.PlayerLabel,
		}
		if _, err := c.q.InsertLastingEffect(ctx, p); err != nil {
			return fmt.Errorf("put an effect back: %w", err)
		}
		touched = append(touched, st.CombatantID)
	}
	if len(touched) == 0 {
		return nil
	}
	slices.Sort(touched)
	return s.refreshCombatants(ctx, c, slices.Compact(touched)...)
}
