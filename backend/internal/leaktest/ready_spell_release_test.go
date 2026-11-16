package leaktest

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The targets of a held spell (#857) are listed in its readier's options, which only the readier's player and the master
// read (RN-10). Ana is refused the options of Toren, and nothing of the held spell is in what she or the TV reads.
func TestTheTargetsOfAHeldSpellAreTheOwnersAndTheMasters(t *testing.T) {
	w := newWorld(t)
	w.onTorenTurnAsWizard()
	toren := w.combatant(w.toren)
	for _, n := range readiedSpellWords {
		w.secrets.add(&canary{needle: n, kind: "readied-spell-targets", readers: names([]*person{w.caio})})
	}
	must(w.caio.ready.ReadyAction(t.Context(), rq(&playv1.ReadyActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: toren.GetId(), IdempotencyKey: newKey(),
		TriggerText: w.secrets.marker("readied-spell-targets-trigger", w.caio), Kind: playv1.ReadiedKind_READIED_KIND_SPELL, SpellKey: "spell:hold-person", SlotLevel: 2,
	})))
	w.endTurnOf(toren) // the options list the held spell off the readier's turn
	options := func(p *person) (*playv1.GetTurnOptionsResponse, error) {
		res, err := p.combat.GetTurnOptions(t.Context(), rq(&playv1.GetTurnOptionsRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: toren.GetId()}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	for _, p := range []*person{w.master, w.caio} {
		res, err := options(p)
		if err != nil {
			t.Fatalf("%s reads Toren's options: %v", p.name, err)
		}
		held := false
		for _, st := range res.GetSpellTargets() {
			held = held || (st.GetSpellKey() == "spell:hold-person" && len(st.GetTargets()) > 0)
		}
		if !held {
			t.Errorf("%s's options list no targets of the held Hold Person: the canary is unreachable", p.name)
		}
	}
	if _, err := options(w.ana); err == nil {
		t.Error("Ana read the options of Toren while he holds a spell")
	}
	w.inspectPlayers(t, w.ana, w.pending)
	w.readScreen(t, "after Toren readied a spell and his options were read", w.tvReader(), nil)
}

// heldTargetIDs are the combatants the person's options of the combatant list as targets of the spell.
func (w *world) heldTargetIDs(p *person, c *playv1.Combatant, spell string) []string {
	w.t.Helper()
	res := must(p.combat.GetTurnOptions(w.t.Context(), rq(&playv1.GetTurnOptionsRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: c.GetId()})))
	var ids []string
	for _, st := range res.GetSpellTargets() {
		if st.GetSpellKey() != spell {
			continue
		}
		for _, tg := range st.GetTargets() {
			ids = append(ids, tg.GetCombatantId())
		}
	}
	return ids
}

// The targets of a held spell and the answer that names them leave out an NPC the players do not see (RN-10): the readier's
// options list the hidden NPC for the master alone; a pick of it is refused exactly as a pick of an id that is nowhere, so
// the refusal does not tell it from one; and a player who is not the readier's is denied the answer with or without targets.
func TestAHeldSpellsTargetsAndItsAnswerKeepTheHiddenNPCApart(t *testing.T) {
	w := newWorld(t)
	w.onTorenTurnAsWizard()
	toren, hidden := w.combatant(w.toren), w.combatant(w.hiddenNPC)
	readied := must(w.caio.ready.ReadyAction(t.Context(), rq(&playv1.ReadyActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: toren.GetId(), IdempotencyKey: newKey(),
		TriggerText: w.secrets.marker("readied-hidden-pick-trigger", w.caio), Kind: playv1.ReadiedKind_READIED_KIND_SPELL, SpellKey: "spell:hold-person", SlotLevel: 2,
	}))).GetEncounter()
	w.endTurnOf(toren)
	if !slices.Contains(w.heldTargetIDs(w.master, toren, "spell:hold-person"), hidden.GetId()) { // the positive control
		t.Fatal("the master's list of the held Hold Person's targets has no hidden NPC: the canary is unreachable")
	}
	if got := w.heldTargetIDs(w.caio, toren, "spell:hold-person"); len(got) == 0 || slices.Contains(got, hidden.GetId()) {
		t.Errorf("Caio's list of the held Hold Person's targets = %v, want some and not the hidden NPC", got)
	}

	ctx := t.Context()
	must(w.master.ready.FireReadied(ctx, rq(&playv1.FireReadiedRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), ReadiedId: w.combatantOf(readied, w.toren.GetId()).GetReadied().GetId(), IdempotencyKey: newKey()})))
	window := w.master.currentEncounter(w).GetReactionWindows()[0].GetId()
	answer := func(p *person, window string, targets ...string) error {
		req := &playv1.AnswerReactionRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: window, Answer: playv1.ReactionChoice_REACTION_CHOICE_USE, IdempotencyKey: newKey()}
		for _, id := range targets {
			req.Targets = append(req.Targets, &playv1.SpellTarget{CombatantId: id})
		}
		_, err := p.combat.AnswerReaction(ctx, rq(req))
		return err
	}
	picked, nowhere := answer(w.caio, window, hidden.GetId()), answer(w.caio, window, newKey()) // newKey is a UUID, as a combatant's id is
	if picked == nil || nowhere == nil || picked.Error() != nowhere.Error() || connect.CodeOf(picked) != connect.CodeNotFound || connect.CodeOf(nowhere) != connect.CodeNotFound {
		t.Errorf("a pick of the hidden NPC = %v, a pick of an id that is nowhere = %v; want the same refusal, not_found", picked, nowhere)
	}
	none, with, without := answer(w.ana, newKey(), w.combatant(w.pens).GetId()), answer(w.ana, window, w.combatant(w.pens).GetId()), answer(w.ana, window)
	if none == nil || with == nil || without == nil || with.Error() != none.Error() || without.Error() != none.Error() || connect.CodeOf(with) != connect.CodeOf(none) {
		t.Errorf("Ana's answer to a window that is nowhere = %v, to Toren's with targets = %v and without = %v; want the same denial", none, with, without)
	}
	w.inspectPlayers(t, w.ana, w.pending)
	w.readScreen(t, "after the answers to Toren's held spell were refused", w.tvReader(), nil)
}
