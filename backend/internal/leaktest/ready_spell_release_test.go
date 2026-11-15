package leaktest

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
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

// The READIED window of a held spell, marked as being rolled while its player is on the map (#857): the held spell, its
// level and the mark are the readier's player's and the master's. Ana and the TV read the wait ("Esperando a reação de
// Toren") and nothing of the spell, before the mark and after it is taken off; her own mark is refused as the mark of a
// window that does not exist. The master's and Caio's reads are the positive controls.
func TestTheMarkedWindowOfAHeldSpellIsTheOwnersAndTheMasters(t *testing.T) {
	w := newWorld(t)
	w.onTorenTurnAsWizard()
	toren := w.combatant(w.toren)
	for _, n := range readiedSpellWords {
		w.secrets.add(&canary{needle: n, kind: "readied-spell-marked", readers: names([]*person{w.caio})})
	}
	rowID := w.combatantOf(must(w.caio.ready.ReadyAction(t.Context(), rq(&playv1.ReadyActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: toren.GetId(), IdempotencyKey: newKey(),
		TriggerText: w.secrets.marker("readied-spell-marked-trigger", w.caio), Kind: playv1.ReadiedKind_READIED_KIND_SPELL, SpellKey: "spell:hold-person", SlotLevel: 2,
	}))).GetEncounter(), w.toren.GetId()).GetReadied().GetId()
	w.endTurnOf(toren)
	must(w.master.ready.FireReadied(t.Context(), rq(&playv1.FireReadiedRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), ReadiedId: rowID, IdempotencyKey: newKey()})))
	windows := w.master.currentEncounter(w).GetReactionWindows()
	if len(windows) != 1 || windows[0].GetKind() != playv1.ReactionKind_REACTION_KIND_READIED {
		t.Fatalf("the master's windows = %v, want the READIED one", windows)
	}
	id := windows[0].GetId()
	w.secrets.add(&canary{needle: id, kind: "readied-spell-marked-window-id", readers: names([]*person{w.caio})})
	mark := func(p *person, windowID string, clear bool) reply {
		return p.call(playv1connect.CombatServiceMarkReactionRollingProcedure, &playv1.MarkReactionRollingRequest{
			CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: windowID, Clear: clear,
		})
	}
	readsOf := func(when string, rolling bool) {
		t.Helper()
		for _, p := range []*person{w.master, w.caio} {
			w.readsHoldAll(t, p, when, append([]string{id}, readiedSpellWords...)...)
			if got := windowOfKind(p, w, playv1.ReactionKind_REACTION_KIND_READIED).GetRolling(); got != rolling {
				t.Errorf("%s: %s reads rolling %v, want %v", when, p.name, got, rolling)
			}
		}
		ana := w.ana.currentEncounter(w)
		if len(ana.GetReactionWindows()) != 0 || ana.GetReactionWait().GetTitlePt() != "Esperando a reação de Toren" {
			t.Errorf("%s: Ana reads the windows %v and the wait %q", when, ana.GetReactionWindows(), ana.GetReactionWait().GetTitlePt())
		}
		w.inspectPlayers(t, w.ana, w.pending)
		w.readScreen(t, when, w.tvReader(), nil)
	}
	readsOf("before the mark", false)
	if r := mark(w.caio, id, false); !r.ok() {
		t.Fatalf("Caio's mark = %d %s", r.status, r.body)
	}
	readsOf("while Toren's player is marked as rolling", true)
	got, unknown := mark(w.ana, id, false), mark(w.ana, newKey(), false)
	if got.ok() || got.status != unknown.status || normalizeRead(string(got.body)) != normalizeRead(string(unknown.body)) {
		t.Errorf("Ana's mark of Toren's window = %d %s, of no window = %d %s", got.status, got.body, unknown.status, unknown.body)
	}
	for _, f := range w.inspect(w.ana, got, nil) {
		t.Errorf("Ana's refused mark: %s", f)
	}
	if r := mark(w.caio, id, true); !r.ok() {
		t.Fatalf("Caio's clear = %d %s", r.status, r.body)
	}
	readsOf("after the mark is taken off", false)
}
