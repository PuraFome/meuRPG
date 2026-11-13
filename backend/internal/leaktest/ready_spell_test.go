package leaktest

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A readied spell (RN-10): the spell held, its level, that it is a spell, and the concentration that holds it are the
// master's and the readier's player's. Another player reads that the combatant readied an action and, when it is lost,
// that it was lost; never which spell. The master's and Caio's reads are the positive controls.

// readiedSpellWords are what only the master and Caio read of Toren's readied Hold Person: its key and its name (they
// hold letters outside a-f, so no id matches them), that it is a spell, and Toren's concentration.
var readiedSpellWords = []string{"spell:hold-person", "Imobilizar Pessoa", "READIED_KIND_SPELL", `"concentrationSpell"`}

// onTorenTurnAsWizard makes Toren a 3rd-level wizard who prepared Hold Person and plays the combat until his turn.
func (w *world) onTorenTurnAsWizard() {
	w.t.Helper()
	sheet := w.fullSheet("class:wizard", "race:human", 3)
	sheet.GetFull().KnownSpellKeys = []string{"spell:hold-person"}
	sheet.GetFull().PreparedSpellKeys = []string{"spell:hold-person"}
	must(w.master.characters.UpdateCharacter(w.t.Context(), rq(&charactersv1.UpdateCharacterRequest{
		CampaignId: w.campaign, CharacterId: w.toren.GetId(), Revision: w.toren.GetRevision(), Name: w.toren.GetName(), Sheet: sheet,
	})))
	w.onTurnOf(w.combatant(w.toren).GetId())
}

// readsHoldAll fails unless the person's reads hold every needle: the positive control.
func (w *world) readsHoldAll(t *testing.T, p *person, when string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !w.readsHold(p, n) {
			t.Errorf("%s: %s's reads hold no %q: the canary is unreachable", when, p.name, n)
		}
	}
}

func TestAReadiedSpellIsTheOwnersAndTheMasters(t *testing.T) {
	w := newWorld(t)
	w.onTorenTurnAsWizard()
	toren := w.combatant(w.toren)
	trigger := w.secrets.marker("readied-spell-trigger", w.caio)
	for _, n := range readiedSpellWords {
		w.secrets.add(&canary{needle: n, kind: "readied-spell", readers: names([]*person{w.caio})})
	}
	readied := must(w.caio.ready.ReadyAction(t.Context(), rq(&playv1.ReadyActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: toren.GetId(), IdempotencyKey: newKey(),
		TriggerText: trigger, Kind: playv1.ReadiedKind_READIED_KIND_SPELL, SpellKey: "spell:hold-person", SlotLevel: 2,
	}))).GetEncounter()
	for _, p := range []*person{w.master, w.caio} {
		w.readsHoldAll(t, p, "after the Ready", append([]string{trigger}, readiedSpellWords...)...)
		if lines, detailed := readiedLinesOf(w.logOfPerson(p), toren.GetId()); lines != 1 || detailed != 1 {
			t.Errorf("%s reads %d readied lines, %d with the spell; want 1 and 1", p.name, lines, detailed)
		}
	}
	if lines, detailed := readiedLinesOf(w.logOfPerson(w.ana), toren.GetId()); lines != 1 || detailed != 0 {
		t.Errorf("Ana reads %d readied lines, %d with the spell; want 1 and 0", lines, detailed)
	}
	w.inspectPlayers(t, w.ana, w.pending)
	w.readScreen(t, "after Toren readied a spell", w.tvReader(), nil)

	// Given up, the spell is lost with its line: the line names it for the master and Caio alone.
	must(w.caio.ready.CancelReadied(t.Context(), rq(&playv1.CancelReadiedRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), ReadiedId: w.combatantOf(readied, w.toren.GetId()).GetReadied().GetId(), IdempotencyKey: newKey(),
	})))
	lost := playv1.CombatLogKind_COMBAT_LOG_KIND_READIED_LOST
	for _, p := range []*person{w.master, w.caio} {
		w.readsHoldAll(t, p, "after the cancel", "spell:hold-person", "READIED_KIND_SPELL")
		if lines, detailed := readiedKindLinesOf(w.logOfPerson(p), toren.GetId(), lost); lines != 1 || detailed != 1 {
			t.Errorf("%s reads %d lost lines, %d with the spell; want 1 and 1", p.name, lines, detailed)
		}
	}
	if lines, detailed := readiedKindLinesOf(w.logOfPerson(w.ana), toren.GetId(), lost); lines != 1 || detailed != 0 {
		t.Errorf("Ana reads %d lost lines, %d with the spell; want 1 and 0", lines, detailed)
	}
	w.inspectPlayers(t, w.ana, w.pending)
}
