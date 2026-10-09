package play

import (
	"math"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// morePrompt builds the prompt of the reactions that are not Shield, the master's
// check or the concentration save. Nothing in a prompt is a number the reactor cannot
// see: the damage is the reactor's own, the attacker is named only when seen, and the
// total and the armor class of an NPC are never in it (RN-20).
func (wv *windowView) morePrompt(kind reaction.Kind, w playdb.ReactionWindow, reactor *playdb.Combatant, out *playv1.ReactionWindow) error {
	t := windowTriggerOf(w)
	var k kit
	if reactor != nil {
		var ok bool
		var err error
		if k, ok, err = wv.s.kitOf(wv.ctx, nil, wv.m.CampaignID, *reactor); err != nil {
			return wv.s.dbError(wv.ctx, "work out the reaction", err)
		} else if !ok {
			k = kit{}
		}
	}
	attacker, attack := wv.attackerOf(t)
	if wv.v.master {
		if a, ok := wv.byID[t.Actor]; ok {
			attacker = a.Label
		}
	}
	switch kind {
	case reaction.CounterspellKind:
		return wv.counterspellPrompt(w, reactor, out)
	case reaction.UncannyDodgeKind:
		out.Prompt = &playv1.ReactionWindow_UncannyDodge{UncannyDodge: &playv1.UncannyDodgePrompt{
			AttackerLabel: attacker, AttackNamePt: attack, Damage: t.Damage, Halved: int32(reaction.UncannyDodge(int(t.Damage))), //nolint:gosec // a damage
		}}
	case reaction.DeflectKind:
		if w.Step == stepSecond {
			out.Prompt = &playv1.ReactionWindow_DeflectThrow{DeflectThrow: &playv1.DeflectMissilesThrowPrompt{
				KiLeft: k.resourceLeft(resKi), NormalRangeFt: reaction.DeflectNormalRangeFt, LongRangeFt: reaction.DeflectLongRangeFt,
			}}
			break
		}
		out.Prompt = &playv1.ReactionWindow_DeflectMissiles{DeflectMissiles: &playv1.DeflectMissilesPrompt{
			AttackerLabel: attacker, AttackNamePt: attack, Damage: t.Damage, DieSides: reaction.DeflectDie,
			FlatBonus: clamp32(k.st.DexMod+k.st.MonkLevel, math.MinInt32, math.MaxInt32), DexMod: clamp32(k.st.DexMod, math.MinInt32, math.MaxInt32), MonkLevel: clamp32(k.st.MonkLevel, 0, math.MaxInt32),
		}}
	case reaction.CuttingWords:
		p := &playv1.CuttingWordsPrompt{
			RollerLabel: attacker, DieSides: clamp32(reaction.CuttingWordsDie(k.st.BardLevel), 0, math.MaxInt32), UsesLeft: k.resourceLeft(resBardic),
		}
		switch t.Roll {
		case "damage":
			p.RollKind = playv1.ReactionRollKind_REACTION_ROLL_KIND_DAMAGE
		case "test":
			p.RollKind = playv1.ReactionRollKind_REACTION_ROLL_KIND_TEST
		default:
			p.RollKind = playv1.ReactionRollKind_REACTION_ROLL_KIND_ATTACK
		}
		if tg, ok := wv.byID[t.Target]; ok && (wv.v.master || wv.v.sees(tg)) {
			p.TargetLabel = tg.Label
		}
		if t.Distance > 0 {
			p.DistanceFt = &t.Distance
		}
		out.Prompt = &playv1.ReactionWindow_CuttingWords{CuttingWords: p}
	case reaction.FeatherFall:
		wv.fallPrompt(w, reactor, k, out)
	case reaction.HellishRebukeKind:
		if w.Step == stepSecond {
			p := &playv1.HellishRebukeSavePrompt{AggressorLabel: attacker, DiceCount: t.Dice}
			p.SaveDc = clamp32(k.st.SaveDC, 0, math.MaxInt32)
			if t.Racial {
				p.SaveDc = clamp32(k.st.LegacyDC, 0, math.MaxInt32)
			}
			if a, ok := wv.byID[t.Actor]; ok {
				if save, err := wv.s.saveOf(wv.ctx, nil, wv.m.CampaignID, a, "dex"); err == nil {
					p.SaveBonus, p.BonusKnown = clamp32(save.Bonus, math.MinInt32, math.MaxInt32), save.Known
				}
			}
			out.Prompt = &playv1.ReactionWindow_HellishRebukeSave{HellishRebukeSave: p}
			break
		}
		p := &playv1.HellishRebukePrompt{AggressorLabel: attacker, AttackNamePt: attack, SaveDc: clamp32(k.st.SaveDC, 0, math.MaxInt32)}
		if t.Distance > 0 {
			p.DistanceFt = &t.Distance
		}
		if left := k.legacyLeft(); left > 0 {
			p.SaveDc = clamp32(max(k.st.LegacyDC, k.st.SaveDC), 0, math.MaxInt32)
			p.Options = append(p.Options, &playv1.HellishRebukeOption{
				Racial: true, Level: reaction.InfernalLegacyLevel, DiceCount: clamp32(reaction.HellishRebukeDice(reaction.InfernalLegacyLevel), 0, math.MaxInt32), UsesLeft: left,
			})
		}
		for _, sl := range k.slotsFor(spellHellishRebuke) {
			p.Options = append(p.Options, &playv1.HellishRebukeOption{
				Slot: sl, Level: sl.GetLevel(), DiceCount: clamp32(reaction.HellishRebukeDice(int(sl.GetLevel())), 0, math.MaxInt32),
			})
		}
		out.Prompt = &playv1.ReactionWindow_HellishRebuke{HellishRebuke: p}
	}
	return nil
}
