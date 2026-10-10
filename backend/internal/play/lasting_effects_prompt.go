package play

import (
	"math"
	"slices"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// The question of a reaction window of kind EFFECT_SAVE, for whoever answers it (RN-22): the
// ability of the saving throw, the bonus, how the d20 rolls and what a pass does. The DC is
// the master's, and the caster's own player's; the DC of an NPC's effect never reaches a
// player (RN-10, RN-20).

// effectOf finds the effect row an effect window is about, in the combat as the view reads it.
func (wv *windowView) effectOf(t windowTrigger) (playdb.CombatantState, bool) {
	i := slices.IndexFunc(wv.d.states, func(st playdb.CombatantState) bool { return st.ID == t.Effect && isEffect(st) })
	if i < 0 {
		return playdb.CombatantState{}, false
	}
	return wv.d.states[i], true
}

func (wv *windowView) effectSavePrompt(w playdb.ReactionWindow, reactor *playdb.Combatant, out *playv1.ReactionWindow) error {
	t := windowTriggerOf(w)
	st, ok := wv.effectOf(t)
	p := &playv1.EffectSavePrompt{EffectId: t.Effect, HandedToMaster: t.Handed, Skippable: wv.v.master, Phase: phaseProto(t.Phase)}
	if !ok {
		out.Prompt = &playv1.ReactionWindow_EffectSave{EffectSave: p}
		return nil
	}
	key := deref(st.SourceKey)
	ability := deref(st.EndSaveAbility)
	if t.Phase == combat.PhaseStart {
		ability = deref(st.StartSaveAbility)
	}
	p.SourceNamePt, p.Ability, p.AbilityNamePt = wv.names(key), ability, wv.names(ability)
	caster, casterKnown := wv.byID[deref(st.SourceID)]
	if wv.v.master || (casterKnown && wv.v.owns(caster)) {
		dc := derefInt32(st.SaveDc)
		if dc == 0 {
			dc = defaultEffectDC
		}
		p.Dc = &dc
	}
	content, err := wv.s.roster.RulesContent(wv.ctx, nil, wv.m.CampaignID)
	if err != nil {
		return wv.s.dbError(wv.ctx, "read the rules content", err)
	}
	def, _ := wv.s.effectDef(content, key)
	p.TextPt = effectSaveText(def, t.Phase, wv.names, st)
	if reactor != nil {
		save, err := wv.s.saveOf(wv.ctx, nil, wv.m.CampaignID, *reactor, ability)
		if err != nil {
			return wv.s.dbError(wv.ctx, "read a saving throw", err)
		}
		p.Modifier, p.BonusKnown = clamp32(save.Bonus, math.MinInt32, math.MaxInt32), save.Known
		states := statesOf(wv.d.states)
		creature := creatureFacts(*reactor, states, link.Traits{})
		sources := combat.SaveMode(combat.SaveScene{Creature: creature, Ability: ability, EffectVisible: true})
		if t.OnDamage {
			sources = append(sources, combat.Source{Kind: combat.SourceEffectSave, Effect: combat.ModeAdvantage})
		}
		p.Mode = modeKey(combat.Resolve(sources))
		for _, src := range shownSources(sources, wv.names) {
			p.ModeSourcesPt = append(p.ModeSourcesPt, src.GetTextPt())
		}
		p.AutoFail = combat.AutoFailsSave(creature, ability)
		p.ExtraDice = extraDiceProto(effectDiceFor(states, reactor.ID, rules.RollAppliesSave), true, wv.names)
	}
	out.Prompt = &playv1.ReactionWindow_EffectSave{EffectSave: p}
	return nil
}

// effectSaveText is what the sheet says of the saving throw an effect asks: no number, only the
// ability and what a pass or a failure does.
func effectSaveText(def *rules.EffectDef, phase string, names func(string) string, st playdb.CombatantState) string {
	ability := names(deref(st.EndSaveAbility))
	if phase == combat.PhaseStart {
		ability = names(deref(st.StartSaveAbility))
	}
	switch {
	case phase == combat.PhaseStart && def != nil && def.StartSave != nil && def.StartSave.OnFail != "":
		return "Você começa o turno dentro do efeito. Teste de resistência de " + ability + ". Se falhar, o efeito prende você: " + names(def.StartSave.OnFail) + "."
	case def != nil && def.EndSave != nil && def.EndSave.OnPass == "end":
		return "Teste de resistência de " + ability + ". Se passar, o efeito acaba sobre você. Se falhar, ele continua."
	}
	return "Teste de resistência de " + ability + "."
}

// effectSaveWait adds a window of kind EFFECT_SAVE to the line the caller reads: "Esperando o
// mestre" for an NPC or for an effect the caller may not read, "Esperando o teste de Brisa" for a
// player's character they see (RN-10: never which NPC, never which effect).
func (wv *windowView) effectSaveWait(w playdb.ReactionWindow, reactor *playdb.Combatant, answers bool, wait *reaction.Wait) {
	if wv.v.master {
		if reactor != nil && reactor.Kind == kindPlayer {
			wait.EffectSavers = append(wait.EffectSavers, reactor.Label)
		} else {
			wait.Self = true
		}
		return
	}
	if answers {
		return
	}
	st, ok := wv.effectOf(windowTriggerOf(w))
	readable := ok && st.PlayerVisible && st.Audience == "all"
	if reactor != nil && reactor.Kind == kindPlayer && wv.v.sees(*reactor) && readable {
		wait.EffectSavers = append(wait.EffectSavers, reactor.Label)
		return
	}
	wait.Master = true
}
