package play

import (
	"math"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// promptFacts is what the prompts of morePrompt read: the window, the trigger it holds,
// who attacked and with what as the caller reads them, and the reactor's kit.
type promptFacts struct {
	w        playdb.ReactionWindow
	t        windowTrigger
	attacker string
	attack   string
	k        kit
}

// kitFor is the kit of the reactor of a window, empty when it has none or none is found.
func (wv *windowView) kitFor(reactor *playdb.Combatant) (kit, error) {
	if reactor == nil {
		return kit{}, nil
	}
	k, ok, err := wv.s.kitOf(wv.ctx, nil, wv.m.CampaignID, *reactor)
	if err != nil {
		return kit{}, wv.s.dbError(wv.ctx, "work out the reaction", err)
	}
	if !ok {
		return kit{}, nil
	}
	return k, nil
}

// morePrompt builds the prompt of the reactions that are not Shield, the master's
// check or the concentration save. Nothing in a prompt is a number the reactor cannot
// see: the damage is the reactor's own, the attacker is named only when seen, and the
// total and the armor class of an NPC are never in it (RN-20).
func (wv *windowView) morePrompt(kind reaction.Kind, w playdb.ReactionWindow, reactor *playdb.Combatant, out *playv1.ReactionWindow) error {
	t := windowTriggerOf(w)
	k, err := wv.kitFor(reactor)
	if err != nil {
		return err
	}
	attacker, attack := wv.attackerOf(t)
	if wv.v.master {
		if a, ok := wv.byID[t.Actor]; ok {
			attacker = a.Label
		}
	}
	f := promptFacts{w: w, t: t, attacker: attacker, attack: attack, k: k}
	switch kind {
	case reaction.CounterspellKind:
		return wv.counterspellPrompt(w, reactor, out)
	case reaction.UncannyDodgeKind:
		uncannyPrompt(f, out)
	case reaction.ManeuverReduceKind:
		maneuverPrompt(f, out)
	case reaction.ProtectionKind:
		wv.protectionPrompt(f, out)
	case reaction.DeflectKind:
		deflectPrompt(f, out)
	case reaction.CuttingWords:
		wv.cuttingWordsPrompt(f, out)
	case reaction.FeatherFall:
		wv.fallPrompt(w, reactor, k, out)
	case reaction.SlowFall:
		slowFallPrompt(f, out)
	case reaction.HellishRebukeKind:
		wv.hellishPrompt(f, out)
	}
	return nil
}

// uncannyPrompt is the prompt of Uncanny Dodge: the damage and the half that lands.
func uncannyPrompt(f promptFacts, out *playv1.ReactionWindow) {
	out.Prompt = &playv1.ReactionWindow_UncannyDodge{UncannyDodge: &playv1.UncannyDodgePrompt{
		AttackerLabel: f.attacker, AttackNamePt: f.attack, Damage: f.t.Damage, Halved: int32(reaction.UncannyDodge(int(f.t.Damage))), //nolint:gosec // a damage
	}}
}

// maneuverPrompt is the prompt of a damage-reducing maneuver: one option for each the
// reactor knows.
func maneuverPrompt(f promptFacts, out *playv1.ReactionWindow) {
	p := &playv1.ManeuverReducePrompt{AttackerLabel: f.attacker, AttackNamePt: f.attack, Damage: f.t.Damage}
	for _, o := range reduceOptionsOf(f.k) {
		p.Options = append(p.Options, &playv1.ManeuverReduceOption{
			Key: o.Key, NamePt: o.NamePT, DieSides: clamp32(o.Sides, 0, 100), FlatBonus: conv.Int32(o.Mod), UsesLeft: f.k.resourceLeft(o.Resource),
		})
	}
	out.Prompt = &playv1.ReactionWindow_ManeuverReduce{ManeuverReduce: p}
}

// deflectPrompt is the prompt of Deflect Missiles: the catch in the first step, the
// throw back in the second.
func deflectPrompt(f promptFacts, out *playv1.ReactionWindow) {
	k := f.k
	if f.w.Step == stepSecond {
		out.Prompt = &playv1.ReactionWindow_DeflectThrow{DeflectThrow: &playv1.DeflectMissilesThrowPrompt{
			KiLeft: k.resourceLeft(resKi), NormalRangeFt: reaction.DeflectNormalRangeFt, LongRangeFt: reaction.DeflectLongRangeFt,
			MissileNamePt: f.attack, AttackBonus: conv.Int32(k.st.Proficiency + k.st.DexMod),
		}}
		return
	}
	out.Prompt = &playv1.ReactionWindow_DeflectMissiles{DeflectMissiles: &playv1.DeflectMissilesPrompt{
		AttackerLabel: f.attacker, AttackNamePt: f.attack, Damage: f.t.Damage, DieSides: reaction.DeflectDie,
		FlatBonus: conv.Int32(k.st.DexMod + k.st.MonkLevel), DexMod: conv.Int32(k.st.DexMod), MonkLevel: clamp32(k.st.MonkLevel, 0, math.MaxInt32),
	}}
}

// rollKindOf is the kind of roll a Cutting Words trigger names; an attack when it names none.
func rollKindOf(roll string) playv1.ReactionRollKind {
	switch roll {
	case "damage":
		return playv1.ReactionRollKind_REACTION_ROLL_KIND_DAMAGE
	case "test":
		return playv1.ReactionRollKind_REACTION_ROLL_KIND_TEST
	}
	return playv1.ReactionRollKind_REACTION_ROLL_KIND_ATTACK
}

// cuttingWordsPrompt is the prompt of Cutting Words: the die, the uses left and the roll it
// can take away from, with the target named only when the caller sees it.
func (wv *windowView) cuttingWordsPrompt(f promptFacts, out *playv1.ReactionWindow) {
	k, t := f.k, f.t
	p := &playv1.CuttingWordsPrompt{
		RollerLabel: f.attacker, DieSides: clamp32(reaction.CuttingWordsDie(k.st.BardLevel), 0, math.MaxInt32), UsesLeft: k.resourceLeft(resBardic),
		RollKind: rollKindOf(t.Roll),
	}
	if tg, ok := wv.byID[t.Target]; ok && (wv.v.master || wv.v.sees(tg)) {
		p.TargetLabel = tg.Label
	}
	if t.Distance > 0 {
		p.DistanceFt = &t.Distance
	}
	out.Prompt = &playv1.ReactionWindow_CuttingWords{CuttingWords: p}
}

// hellishPrompt is the prompt of Hellish Rebuke: the aggressor's save in the second step,
// the choice of slot or legacy in the first.
func (wv *windowView) hellishPrompt(f promptFacts, out *playv1.ReactionWindow) {
	if f.w.Step == stepSecond {
		wv.hellishSavePrompt(f, out)
		return
	}
	k, t := f.k, f.t
	p := &playv1.HellishRebukePrompt{AggressorLabel: f.attacker, AttackNamePt: f.attack, SaveDc: clamp32(k.st.SaveDC, 0, math.MaxInt32)}
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

// hellishSavePrompt is the second step of Hellish Rebuke: the aggressor's save against the
// reactor's DC.
func (wv *windowView) hellishSavePrompt(f promptFacts, out *playv1.ReactionWindow) {
	k, t := f.k, f.t
	p := &playv1.HellishRebukeSavePrompt{AggressorLabel: f.attacker, DiceCount: t.Dice}
	p.SaveDc = clamp32(k.st.SaveDC, 0, math.MaxInt32)
	if t.Racial {
		p.SaveDc = clamp32(k.st.LegacyDC, 0, math.MaxInt32)
	}
	// The aggressor's modifier is the master's: the reactor's player rolls nothing here and never
	// reads an NPC's number (RN-20).
	if a, ok := wv.byID[t.Actor]; ok && wv.v.master {
		if save, err := wv.s.saveOf(wv.ctx, nil, wv.m.CampaignID, a, "dex"); err == nil {
			p.SaveBonus, p.BonusKnown = conv.Int32(save.Bonus), save.Known
		}
	}
	out.Prompt = &playv1.ReactionWindow_HellishRebukeSave{HellishRebukeSave: p}
}

// protectionPrompt is the prompt of the Protection style: who attacks and who is protected, each named
// only when the caller sees it (RN-10).
func (wv *windowView) protectionPrompt(f promptFacts, out *playv1.ReactionWindow) {
	p := &playv1.ProtectionPrompt{AttackerLabel: f.attacker}
	if ally, ok := wv.byID[f.t.Target]; ok && (wv.v.master || wv.v.sees(ally)) {
		p.AllyLabel = ally.Label
	}
	out.Prompt = &playv1.ReactionWindow_Protection{Protection: p}
}
