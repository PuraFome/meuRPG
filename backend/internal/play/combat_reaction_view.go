package play

import (
	"context"
	"fmt"
	"math"
	"slices"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// What a caller reads of the open reaction windows (PM-04, RN-10, RN-20): the master
// every window with its numbers; a player only the windows they answer, with what their
// own character sees; everyone else only the line "Esperando ...", written for them.

var reactionKindProto = map[reaction.Kind]playv1.ReactionKind{
	reaction.Shield:            playv1.ReactionKind_REACTION_KIND_SHIELD,
	reaction.UncannyDodgeKind:  playv1.ReactionKind_REACTION_KIND_UNCANNY_DODGE,
	reaction.HellishRebukeKind: playv1.ReactionKind_REACTION_KIND_HELLISH_REBUKE,
	reaction.CounterspellKind:  playv1.ReactionKind_REACTION_KIND_COUNTERSPELL,
	reaction.CuttingWords:      playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS,
	reaction.DeflectKind:       playv1.ReactionKind_REACTION_KIND_DEFLECT_MISSILES,
	reaction.FeatherFall:       playv1.ReactionKind_REACTION_KIND_FEATHER_FALL,
	reaction.Concentration:     playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE,
	reaction.MasterCheck:       playv1.ReactionKind_REACTION_KIND_MASTER_CHECK,
}

// windowView is what the builders of a caller's windows share.
type windowView struct {
	s     *Service
	ctx   context.Context
	m     authz.Membership
	v     combatViewer
	d     *encounterData
	names func(key string) string
	byID  map[string]playdb.Combatant
}

// reactionView builds Encounter.reaction_windows and Encounter.reaction_wait for the
// caller. It reads through the pool: it is a read (GetEncounter, a response after
// the commit).
func (s *Service) reactionView(ctx context.Context, m authz.Membership, d *encounterData, v combatViewer, names func(key string) string) ([]*playv1.ReactionWindow, *playv1.ReactionWait, error) {
	if d.enc.Status != statusActive {
		return nil, nil, nil
	}
	open, err := s.queries.ListOpenReactionWindows(ctx, d.enc.ID)
	if err != nil {
		return nil, nil, s.dbError(ctx, "list the reaction windows", err)
	}
	offers, err := s.queries.ListPendingOpportunityOffers(ctx, d.enc.ID)
	if err != nil {
		return nil, nil, s.dbError(ctx, "list the opportunity offers", err)
	}
	reveals, err := s.queries.ListPendingHiddenReveals(ctx, d.enc.ID)
	if err != nil {
		return nil, nil, s.dbError(ctx, "list the hidden reveals", err)
	}
	if len(open) == 0 && len(offers) == 0 && len(reveals) == 0 {
		return nil, nil, nil
	}
	wv := &windowView{s: s, ctx: ctx, m: m, v: v, d: d, names: names, byID: make(map[string]playdb.Combatant, len(d.cs))}
	for _, x := range d.cs {
		wv.byID[x.ID] = x
	}
	order := make([]reaction.Window, len(open))
	for i, w := range open {
		order[i] = reaction.Window{ID: w.ID, Group: w.GroupID, Seq: w.Seq, Open: true}
	}
	var windows []*playv1.ReactionWindow
	var wait reaction.Wait
	holds := reaction.HoldsTurn
	holdsSet := false
	for _, w := range open {
		var reactor *playdb.Combatant
		if w.ReactorID != nil {
			if r, ok := wv.byID[*w.ReactorID]; ok {
				reactor = &r
			}
		}
		answers := v.master || (reactor != nil && v.owns(*reactor))
		if answers {
			pw, err := wv.window(w, reactor, reaction.AnswerNow(order, w.ID))
			if err != nil {
				return nil, nil, err
			}
			windows = append(windows, pw)
		}
		wv.waitOf(w, reactor, answers, &wait)
		if !holdsSet && !answers {
			h, err := wv.holdsOf(w)
			if err != nil {
				return nil, nil, err
			}
			holds, holdsSet = h, true
		}
	}
	// The opportunity attacks are windows too (RN-21): the offer's own calls answer them.
	for _, o := range offers {
		mover, ok1 := wv.byID[o.MoverID]
		reactor, ok2 := wv.byID[o.ReactorID]
		if !ok1 || !ok2 || !v.seesAtOffer(mover, o) {
			continue
		}
		answers := v.master || v.owns(reactor)
		if !answers && !v.owns(mover) {
			continue
		}
		if answers {
			windows = append(windows, wv.opportunityWindow(o, mover, reactor))
		}
		wv.waitOf(playdb.ReactionWindow{Kind: "opportunity"}, &reactor, answers, &wait)
		if !holdsSet && !answers {
			holds, holdsSet = reaction.HoldsTurn, true
		}
	}
	// The master's question about a hidden creature an area hit (PM-02c) is a window too: the
	// master answers it with ResolveHiddenReveal, and every player reads the same wait.
	for _, q := range reveals {
		if v.master {
			windows = append(windows, &playv1.ReactionWindow{
				Id: q.ID, Kind: playv1.ReactionKind_REACTION_KIND_HIDDEN_REVEAL, Status: playv1.ReactionWindowStatus_REACTION_WINDOW_STATUS_OPEN,
				GroupId: q.ID, ForYou: true, AnswerNow: true,
				Trigger: &playv1.ReactionTrigger{ActorId: q.CasterID},
			})
		}
		if v.master {
			wait.Self = true
		} else {
			wait.Master = true
		}
		if !holdsSet {
			holds, holdsSet = reaction.HoldsTurn, true
		}
	}
	var out *playv1.ReactionWait
	if title := wait.Title(); title != "" {
		out = &playv1.ReactionWait{TitlePt: title}
		if !v.master {
			out.DetailPt = reaction.Detail(holds, wait, "")
		}
	}
	return windows, out, nil
}

// opportunityWindow is the window of an opportunity-attack offer for someone who answers it:
// the master also reads who moves and who may react.
func (wv *windowView) opportunityWindow(o playdb.OpportunityOffer, mover, reactor playdb.Combatant) *playv1.ReactionWindow {
	pw := &playv1.ReactionWindow{
		Id: o.ID, Kind: playv1.ReactionKind_REACTION_KIND_OPPORTUNITY, Status: playv1.ReactionWindowStatus_REACTION_WINDOW_STATUS_OPEN,
		GroupId: o.MoveID, ReactorId: reactor.ID, ReactorLabel: reactor.Label, ReactorIsPlayer: reactor.Kind == kindPlayer, ForYou: true, AnswerNow: wv.v.master,
	}
	if wv.v.master {
		pw.Trigger = &playv1.ReactionTrigger{ActorId: mover.ID, ActorLabel: mover.Label, TargetId: reactor.ID, TargetLabel: reactor.Label}
	}
	return pw
}

// waitOf adds a window to the line the caller reads. A window the caller answers is
// not waited for: they have its prompt. A reactor that is an NPC, the master's check
// or a creature the caller does not see is "o mestre"; only a player's character the
// caller sees is named (never which NPC, never why).
func (wv *windowView) waitOf(w playdb.ReactionWindow, reactor *playdb.Combatant, answers bool, wait *reaction.Wait) {
	save := reaction.Kind(w.Kind) == reaction.Concentration
	if wv.v.master {
		switch {
		case reactor != nil && reactor.Kind == kindPlayer && save:
			wait.Savers = append(wait.Savers, reactor.Label)
		case reactor != nil && reactor.Kind == kindPlayer:
			wait.Reactors = append(wait.Reactors, reactor.Label)
		default:
			wait.Self = true
		}
		return
	}
	if answers {
		return
	}
	switch {
	case reactor != nil && reactor.Kind == kindPlayer && wv.v.sees(*reactor) && save:
		wait.Savers = append(wait.Savers, reactor.Label)
	case reactor != nil && reactor.Kind == kindPlayer && wv.v.sees(*reactor):
		wait.Reactors = append(wait.Reactors, reactor.Label)
	default:
		wait.Master = true
	}
}

// holdsOf says what the window holds, for the caller: their own attack or spell, or
// only the turn.
func (wv *windowView) holdsOf(w playdb.ReactionWindow) (reaction.Holds, error) {
	t := windowTriggerOf(w)
	if w.HoldID != nil {
		h, err := wv.s.queries.GetReactionHold(wv.ctx, playdb.GetReactionHoldParams{EncounterID: wv.d.enc.ID, ID: *w.HoldID})
		if err == nil && h.ActorUserID == wv.v.userID {
			if h.Kind == "cast" {
				return reaction.HoldsYourSpell, nil
			}
			return reaction.HoldsYourAttack, nil
		}
		return reaction.HoldsTurn, nil
	}
	if w.PendingDamageID != nil {
		if a, ok := wv.byID[t.Actor]; ok && wv.v.owns(a) {
			return reaction.HoldsYourAttack, nil
		}
	}
	return reaction.HoldsTurn, nil
}

// window builds one window for whoever answers it.
func (wv *windowView) window(w playdb.ReactionWindow, reactor *playdb.Combatant, answerNow bool) (*playv1.ReactionWindow, error) {
	kind := reaction.Kind(w.Kind)
	out := &playv1.ReactionWindow{
		Id: w.ID, Kind: reactionKindProto[kind], Status: playv1.ReactionWindowStatus_REACTION_WINDOW_STATUS_OPEN, GroupId: w.GroupID,
		ForYou: true, SecondStep: w.Step == stepSecond,
	}
	if reactor != nil {
		out.ReactorId, out.ReactorLabel, out.ReactorIsPlayer = reactor.ID, reactor.Label, reactor.Kind == kindPlayer
	}
	if wv.v.master {
		out.AnswerNow = answerNow
		out.Trigger = wv.trigger(w)
	}
	var err error
	switch kind {
	case reaction.Shield:
		err = wv.shieldPrompt(w, reactor, out)
	case reaction.MasterCheck:
		err = wv.masterCheckPrompt(w, out)
	case reaction.Concentration:
		err = wv.concentrationPrompt(w, reactor, out)
	default:
		err = wv.morePrompt(kind, w, reactor, out)
	}
	return out, err
}

// trigger is what happened, with the master's numbers.
func (wv *windowView) trigger(w playdb.ReactionWindow) *playv1.ReactionTrigger {
	t := windowTriggerOf(w)
	out := &playv1.ReactionTrigger{
		ActorId: t.Actor, TargetId: t.Target, ActionKey: t.Key,
		SpellKey: t.Spell, SpellLevel: t.Level, ConcentrationSpellKey: "", DamageTaken: t.Taken,
	}
	if a, ok := wv.byID[t.Actor]; ok {
		out.ActorLabel = a.Label
	}
	if x, ok := wv.byID[t.Target]; ok {
		out.TargetLabel = x.Label
	}
	if t.Key != "" {
		out.ActionNamePt = wv.names(t.Key)
		if out.ActionNamePt == "" {
			out.ActionNamePt = wv.attackName(t)
		}
	}
	if t.Distance > 0 {
		out.DistanceFt = &t.Distance
	}
	if t.Total != 0 || t.AC != 0 {
		out.AttackTotal, out.TargetArmorClass = &t.Total, &t.AC
	}
	if t.Damage != 0 {
		out.Damage = &t.Damage
	}
	if reaction.Kind(w.Kind) == reaction.Concentration {
		out.ConcentrationSpellKey = t.Spell
	}
	switch t.Roll {
	case "attack":
		out.RollKind = playv1.ReactionRollKind_REACTION_ROLL_KIND_ATTACK
	case "test":
		out.RollKind = playv1.ReactionRollKind_REACTION_ROLL_KIND_TEST
	case "damage":
		out.RollKind = playv1.ReactionRollKind_REACTION_ROLL_KIND_DAMAGE
	}
	return out
}

// attackName is the Portuguese name of the attack of a trigger, from its actor's sheet.
func (wv *windowView) attackName(t windowTrigger) string {
	a, ok := wv.byID[t.Actor]
	if !ok {
		return ""
	}
	sheet, err := wv.s.sheetOf(wv.ctx, nil, wv.m.CampaignID, a)
	if err != nil {
		return ""
	}
	if i := slices.IndexFunc(sheet.Attacks, func(x link.Attack) bool { return x.Key == t.Key }); i >= 0 {
		return sheet.Attacks[i].Name
	}
	return ""
}

// attackerOf names who attacked and with what, only when the caller sees them (RN-20).
func (wv *windowView) attackerOf(t windowTrigger) (label, attack string) {
	a, ok := wv.byID[t.Actor]
	if !ok || !wv.v.sees(a) {
		return "", ""
	}
	return a.Label, wv.attackName(t)
}

func (wv *windowView) shieldPrompt(w playdb.ReactionWindow, reactor *playdb.Combatant, out *playv1.ReactionWindow) error {
	t := windowTriggerOf(w)
	p := &playv1.ShieldPrompt{PendingDamageId: deref(w.PendingDamageID), MagicMissile: t.Magic, SpellNamePt: wv.names(shield)}
	p.AttackerLabel, p.AttackNamePt = wv.attackerOf(t)
	if reactor != nil {
		k, ok, err := wv.s.kitOf(wv.ctx, nil, wv.m.CampaignID, *reactor)
		if err != nil {
			return wv.s.dbError(wv.ctx, "work out the reaction", err)
		}
		if ok {
			p.Slots = k.slotsFor(spellShield)
		}
		if wv.v.master {
			with := clamp32(int(t.AC)-int(reactor.AcBonus)+5, 0, math.MaxInt32)
			if out.Trigger != nil {
				out.Trigger.ArmorClassWithShield = &with
			}
		}
	}
	out.Prompt = &playv1.ReactionWindow_Shield{Shield: p}
	return nil
}

func (wv *windowView) masterCheckPrompt(w playdb.ReactionWindow, out *playv1.ReactionWindow) error {
	t := windowTriggerOf(w)
	a, tg := wv.byID[t.Actor], wv.byID[t.Target]
	summary := fmt.Sprintf("Ataque de %s contra %s", a.Label, tg.Label)
	switch {
	case t.Magic:
		summary = fmt.Sprintf("Mísseis Mágicos de %s contra %s", a.Label, tg.Label)
	case t.Roll == "cast" || t.Spell != "":
		summary = fmt.Sprintf("Conjuração de %s", a.Label)
	case t.Roll == "damage":
		summary = fmt.Sprintf("Dano de %s em %s", a.Label, tg.Label)
	case t.AC != 0 || t.Total != 0:
		summary += " · acertou"
	}
	out.Prompt = &playv1.ReactionWindow_MasterCheck{MasterCheck: &playv1.MasterCheckPrompt{SummaryPt: summary}}
	return nil
}

func (wv *windowView) concentrationPrompt(w playdb.ReactionWindow, reactor *playdb.Combatant, out *playv1.ReactionWindow) error {
	t := windowTriggerOf(w)
	p := &playv1.ConcentrationSavePrompt{
		SpellKey: t.Spell, SpellNamePt: wv.names(t.Spell), DamageTaken: t.Taken,
		Dc: clamp32(reaction.ConcentrationDC(int(t.Taken)), 0, math.MaxInt32), HandedToMaster: t.Handed,
	}
	if a, ok := wv.byID[t.Actor]; ok && wv.v.sees(a) && t.Key != "" {
		if name := wv.names(t.Key); name != "" {
			p.SourceLabel = name + " de " + a.Label
		} else if n := wv.attackName(t); n != "" {
			p.SourceLabel = n + " de " + a.Label
		}
	}
	if reactor != nil {
		save, err := wv.s.saveOf(wv.ctx, nil, wv.m.CampaignID, *reactor, "con")
		if err != nil {
			return wv.s.dbError(wv.ctx, "read a saving throw", err)
		}
		p.SaveBonus, p.BonusKnown = clamp32(save.Bonus, math.MinInt32, math.MaxInt32), save.Known
	}
	out.Prompt = &playv1.ReactionWindow_ConcentrationSave{ConcentrationSave: p}
	return nil
}
