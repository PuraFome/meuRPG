package leaktest

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

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
// Toren") and nothing of the spell; Ana's wait line is the same after the mark, the clear and the second mark as before it, and no read of hers or of
// the TV's holds `rolling` or `waitBeganAt` (the wait began at the first mark and the master and Caio read the same moment
// after a clear and a second mark); her own mark is refused as the mark of a window that does not exist. The master's and Caio's
// reads are the positive controls.
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
	anaBefore := w.waitReadOf(w.ana)
	var began *timestamppb.Timestamp // when the wait began: the first mark's, whatever the marks and clears after it
	readsOf := func(when string, rolling bool) {
		t.Helper()
		for _, p := range []*person{w.master, w.caio} {
			w.readsHoldAll(t, p, when, append([]string{id}, readiedSpellWords...)...)
			if got := windowOfKind(p, w, playv1.ReactionKind_REACTION_KIND_READIED).GetRolling(); got != rolling {
				t.Errorf("%s: %s reads rolling %v, want %v", when, p.name, got, rolling)
			}
			if got := w.waitBeganOf(p); !proto.Equal(got, began) {
				t.Errorf("%s: %s reads the wait began at %v, want %v", when, p.name, got, began)
			}
		}
		w.inspectPlayers(t, w.ana, w.pending)
		tv := w.readScreen(t, when, w.tvReader(), nil)
		w.sameWaitAs(t, when, w.ana, anaBefore, "Esperando a reação de Toren", tv)
	}
	readsOf("before the mark", false)
	if r := w.markRolling(w.caio, id, false); !r.ok() {
		t.Fatalf("Caio's mark = %d %s", r.status, r.body)
	}
	if began = w.waitBeganOf(w.master); began == nil {
		t.Fatal("the master reads no beginning of the wait after the first mark")
	}
	readsOf("while Toren's player is marked as rolling", true)
	for _, p := range []*person{w.master, w.caio} { // the positive controls: both fields are in their reads
		w.readsHoldAll(t, p, "while Toren's player is marked as rolling", `"rolling"`, `"waitBeganAt"`)
	}
	got, unknown := w.markRolling(w.ana, id, false), w.markRolling(w.ana, newKey(), false)
	if got.ok() || got.status != unknown.status || normalizeRead(string(got.body)) != normalizeRead(string(unknown.body)) {
		t.Errorf("Ana's mark of Toren's window = %d %s, of no window = %d %s", got.status, got.body, unknown.status, unknown.body)
	}
	for _, f := range w.inspect(w.ana, got, nil) {
		t.Errorf("Ana's refused mark: %s", f)
	}
	if r := w.markRolling(w.ana, id, true); r.ok() {
		t.Errorf("Ana's clear of Toren's window = %d %s, want it refused", r.status, r.body)
	}
	readsOf("after Ana's refused clear", true)
	if r := w.markRolling(w.caio, id, true); !r.ok() {
		t.Fatalf("Caio's clear = %d %s", r.status, r.body)
	}
	readsOf("after the mark is taken off", false)
	if r := w.markRolling(w.caio, id, false); !r.ok() {
		t.Fatalf("Caio's second mark = %d %s", r.status, r.body)
	}
	readsOf("after the mark is set again", true)
}
