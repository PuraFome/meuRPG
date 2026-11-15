package leaktest

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
)

// A readied move and its release (RN-10, #856): while the READIED window of the move is open and its player is on the map
// (marked as rolling), the trigger, the held move and the window are the readier's player's and the master's; Ana and the TV
// read the wait and nothing of it: the wait is the same before the mark, after it, after the clear and after the second mark, and no read of theirs holds `rolling` or `waitBeganAt`.
// The move itself is a move like any other: once made, the window is gone, the token is
// where it went, and Ana reads the fired line with the actor alone. The master's and Caio's reads are the positive controls.

// movedLinesOf counts the moved lines of a person's log whose actor is id.
func movedLinesOf(entries []*playv1.CombatLogEntry, id string) int {
	n := 0
	for _, e := range entries {
		if e.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_MOVED && e.GetActorId() == id {
			n++
		}
	}
	return n
}

func TestAReadiedMovesWindowAndReleaseAreTheOwnersAndTheMasters(t *testing.T) {
	w := newWorld(t)
	w.onTorenTurn()
	toren := w.combatant(w.toren)
	trigger := w.secrets.marker("readied-move-trigger", w.caio)
	words := []string{`"triggerText"`, "READIED_KIND_MOVE", "READIED_STATUS_WAITING", "READIED_STATUS_FIRED"}
	for _, n := range words {
		w.secrets.add(&canary{needle: n, kind: "readied-move-detail", readers: names([]*person{w.caio})})
	}
	rowID := w.combatantOf(must(w.caio.ready.ReadyAction(t.Context(), rq(&playv1.ReadyActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: toren.GetId(), IdempotencyKey: newKey(),
		TriggerText: trigger, Kind: playv1.ReadiedKind_READIED_KIND_MOVE,
	}))).GetEncounter(), w.toren.GetId()).GetReadied().GetId()
	w.endTurnOf(toren)
	must(w.master.ready.FireReadied(t.Context(), rq(&playv1.FireReadiedRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), ReadiedId: rowID, IdempotencyKey: newKey()})))
	windows := w.master.currentEncounter(w).GetReactionWindows()
	if len(windows) != 1 || windows[0].GetKind() != playv1.ReactionKind_REACTION_KIND_READIED {
		t.Fatalf("the master's windows = %v, want the READIED one", windows)
	}
	id := windows[0].GetId()
	w.secrets.add(&canary{needle: id, kind: "readied-move-window-id", readers: names([]*person{w.caio})})
	anaBefore := w.waitReadOf(w.ana)
	var began *timestamppb.Timestamp // when the wait began: the first mark's, whatever the marks and clears after it
	readsOf := func(when string, rolling bool) {
		t.Helper()
		for _, p := range []*person{w.master, w.caio} { // the positive controls
			w.readsHoldAll(t, p, when, append([]string{id, trigger}, words[:3]...)...)
			if got := windowOfKind(p, w, playv1.ReactionKind_REACTION_KIND_READIED); got == nil || got.GetRolling() != rolling {
				t.Errorf("%s: %s reads the window %v, want rolling %v", when, p.name, got, rolling)
			}
			if got := w.waitBeganOf(p); !proto.Equal(got, began) {
				t.Errorf("%s: %s reads the wait began at %v, want %v", when, p.name, got, began)
			}
		}
		w.inspectPlayers(t, w.ana, w.pending)
		tv := w.readScreen(t, when, w.tvReader(), nil)
		w.sameWaitAs(t, when, w.ana, anaBefore, "Esperando a reação de Toren", tv)
	}
	if r := w.markRolling(w.caio, id, false); !r.ok() {
		t.Fatalf("Caio's mark = %d %s", r.status, r.body)
	}
	if began = w.waitBeganOf(w.master); began == nil {
		t.Fatal("the master reads no beginning of the wait after the first mark")
	}
	readsOf("while Toren's player is on the map", true)
	for _, p := range []*person{w.master, w.caio} { // the positive controls: both fields are in their reads
		w.readsHoldAll(t, p, "while Toren's player is on the map", `"rolling"`, `"waitBeganAt"`)
	}
	if r := w.markRolling(w.caio, id, true); !r.ok() {
		t.Fatalf("Caio's clear = %d %s", r.status, r.body)
	}
	readsOf("after Toren's player went back", false)
	if r := w.markRolling(w.caio, id, false); !r.ok() {
		t.Fatalf("Caio's second mark = %d %s", r.status, r.body)
	}
	readsOf("after Toren's player went to the map again", true)

	from := w.combatant(w.toren)
	moved := w.caio.call(playv1connect.CombatServiceMoveCombatantProcedure, &playv1.MoveCombatantRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: toren.GetId(), IdempotencyKey: newKey(), Col: from.GetCol() - 2, Row: from.GetRow(),
	})
	if !moved.ok() {
		t.Fatalf("Toren's readied move: status %d %s", moved.status, moved.body)
	}
	if got := w.master.currentEncounter(w); len(got.GetReactionWindows()) != 0 || !w.combatantOf(got, w.toren.GetId()).GetReactionUsed() {
		t.Errorf("after the move: windows %v, reaction used %v", got.GetReactionWindows(), w.combatantOf(got, w.toren.GetId()).GetReactionUsed())
	}
	for _, p := range []*person{w.master, w.caio} { // the positive controls
		if n := movedLinesOf(w.logOfPerson(p), toren.GetId()); n != 1 {
			t.Errorf("%s reads %d moved lines of Toren, want 1", p.name, n)
		}
		if lines, detailed := readiedKindLinesOf(w.logOfPerson(p), toren.GetId(), playv1.CombatLogKind_COMBAT_LOG_KIND_READIED_FIRED); lines != 1 || detailed != 1 {
			t.Errorf("%s reads %d fired lines, %d with the detail; want 1 and 1", p.name, lines, detailed)
		}
	}
	if lines, detailed := readiedKindLinesOf(w.logOfPerson(w.ana), toren.GetId(), playv1.CombatLogKind_COMBAT_LOG_KIND_READIED_FIRED); lines != 1 || detailed != 0 {
		t.Errorf("Ana reads %d fired lines, %d with the detail; want 1 and 0", lines, detailed)
	}
	if n := movedLinesOf(w.logOfPerson(w.ana), toren.GetId()); n != 1 {
		t.Errorf("Ana reads %d moved lines of Toren, want 1 as for any move", n)
	}
	w.inspectPlayers(t, w.ana, w.pending)
	w.readScreen(t, "after Toren's readied move", w.tvReader(), nil)
}
