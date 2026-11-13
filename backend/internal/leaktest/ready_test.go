package leaktest

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A readied action (ReadyService, RN-10): the trigger the player wrote and the action held are the master's and the
// readier's player's. Another player reads that the combatant readied an action, never what or when; a hidden NPC's
// readied action is the master's alone, line and all. The master's and the owner's reads are the positive controls; the
// TV (the table's spectator) is inspected with the other players.

// readiedWords are the fields that only the master's and the owner's copies carry: the readied action of the combatant
// and the detail of its line.
var readiedWords = []string{`"triggerText"`, "READIED_KIND_ATTACK", "READIED_STATUS_WAITING"}

// readiedLinesOf counts the readied lines of a person's log whose actor is id, and those with the detail.
func readiedLinesOf(entries []*playv1.CombatLogEntry, id string) (lines, detailed int) {
	for _, e := range entries {
		if e.GetKind() != playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION_READIED || e.GetActorId() != id {
			continue
		}
		lines++
		if e.GetReadied() != nil {
			detailed++
		}
	}
	return lines, detailed
}

func TestAReadiedActionIsTheOwnersAndTheMasters(t *testing.T) {
	w := newWorld(t)
	w.onTorenTurn()
	toren := w.combatant(w.toren)
	trigger := w.secrets.marker("readied-trigger", w.caio)
	for _, n := range readiedWords {
		w.secrets.add(&canary{needle: n, kind: "readied-detail", readers: names([]*person{w.caio})})
	}
	must(w.caio.ready.ReadyAction(t.Context(), rq(&playv1.ReadyActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: toren.GetId(), IdempotencyKey: newKey(),
		TriggerText: trigger, Kind: playv1.ReadiedKind_READIED_KIND_ATTACK, ActionKey: "equipment:dagger",
	})))

	for _, p := range []*person{w.master, w.caio} { // the positive controls
		if lines, detailed := readiedLinesOf(w.logOfPerson(p), toren.GetId()); lines != 1 || detailed != 1 {
			t.Errorf("%s reads %d readied lines, %d with the trigger; want 1 and 1", p.name, lines, detailed)
		}
		found := false
		for _, r := range w.reads(p) {
			found = found || contains(r.body, trigger)
		}
		if !found {
			t.Errorf("%s's reads hold no trigger: the canary is unreachable", p.name)
		}
	}
	if lines, detailed := readiedLinesOf(w.logOfPerson(w.ana), toren.GetId()); lines != 1 || detailed != 0 {
		t.Errorf("Ana reads %d readied lines, %d with the trigger; want 1 and 0", lines, detailed)
	}
	for _, p := range []*person{w.ana, w.pending} {
		for _, r := range w.reads(p) {
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
	}
	w.readScreen(t, "after Toren readied", w.tvReader(), nil)
}

// The master readies an action for the hidden NPC on its turn: no player reads a word of it, not even the line.
func TestAHiddenNPCsReadiedActionIsTheMastersAlone(t *testing.T) {
	w := newWorld(t)
	hidden := w.combatant(w.hiddenNPC)
	w.onTurnOf(hidden.GetId())
	trigger := w.secrets.marker("readied-hidden-trigger")
	must(w.master.ready.ReadyAction(t.Context(), rq(&playv1.ReadyActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: hidden.GetId(), IdempotencyKey: newKey(),
		TriggerText: trigger, Kind: playv1.ReadiedKind_READIED_KIND_OTHER, ActionKey: "standard:dodge",
	})))

	if lines, detailed := readiedLinesOf(w.logOfPerson(w.master), hidden.GetId()); lines != 1 || detailed != 1 {
		t.Errorf("the master reads %d readied lines of the hidden NPC, %d with the trigger; want 1 and 1", lines, detailed)
	}
	found := false
	for _, r := range w.reads(w.master) {
		found = found || contains(r.body, trigger)
	}
	if !found {
		t.Error("the master's reads hold no trigger: the canary is unreachable")
	}
	for _, p := range []*person{w.ana, w.caio} {
		if lines, _ := readiedLinesOf(w.logOfPerson(p), hidden.GetId()); lines != 0 {
			t.Errorf("%s reads %d readied lines of the hidden NPC, want none", p.name, lines)
		}
	}
	for _, p := range []*person{w.ana, w.caio, w.pending} { // a pending member has no log at all: the reads are the check
		for _, r := range w.reads(p) {
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
	}
	w.readScreen(t, "after the hidden NPC readied", w.tvReader(), nil)
}

// onTurnOf plays the combat until the combatant's turn.
func (w *world) onTurnOf(id string) {
	w.t.Helper()
	ctx := w.t.Context()
	for range 40 {
		cur := must(w.master.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
		if cur.GetCurrentCombatantId() == id {
			w.encounter = cur
			return
		}
		must(w.master.combat.EndTurn(ctx, rq(&playv1.EndTurnRequest{CampaignId: w.campaign, EncounterId: cur.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: cur.GetCurrentCombatantId(), ExpectedRound: cur.GetRound()})))
	}
	w.t.Fatal("it never became the combatant's turn")
}
