package leaktest

import (
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The app rolls a creature's initiative, and a total that equals another combatant's makes them one joint turn.
// The services roll with dicetest, so the roll is the same on every run.
// A test that adds a creature to a running combat states the d20 it gets (scriptD20) and checks the total is its own (requireOwnTotal).

// ownFace is the d20 a creature added mid-combat is scripted to roll, unless a test needs another.
const ownFace = 1

// scriptD20 makes the next d20 the app rolls come out as face.
func (w *world) scriptD20(face int) {
	w.t.Helper()
	w.dice.ScriptFor(w.t, 20, face)
}

// requireOwnTotal fails when the combatant's initiative total equals another combatant's.
// The rolls are deterministic, so a tie here is a tie on every run: script the faces with ScriptFor (scriptD20).
func (w *world) requireOwnTotal(id string) {
	w.t.Helper()
	e := w.master.currentEncounter(w)
	var own *playv1.Combatant
	for _, c := range e.GetCombatants() {
		if c.GetId() == id {
			own = c
		}
	}
	if own == nil {
		w.t.Fatalf("combatant %s is not in the combat", id)
	}
	for _, c := range e.GetCombatants() {
		if c.GetId() != id && c.Initiative != nil && c.GetInitiative() == own.GetInitiative() {
			w.t.Fatalf("%s and %s tie on initiative %d and would take one joint turn: script the faces with ScriptFor (%s)",
				own.GetLabel(), c.GetLabel(), own.GetInitiative(), initiativeOrder(e))
		}
	}
}

// currentEncounter is the master's copy of the combat.
func (p *person) currentEncounter(w *world) *playv1.Encounter {
	w.t.Helper()
	return must(p.combat.GetEncounter(w.t.Context(), rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
}
