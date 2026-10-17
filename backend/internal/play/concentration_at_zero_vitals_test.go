package play

import (
	"slices"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Finding fix/concentration-at-zero-hp: a character who concentrates in a combat and whose hit
// points the master sets to 0 (the sheet path, AdjustCharacterVitals) loses the concentration and
// the effects it held, as when damage takes him to 0 (SRD 5.1, "Duration": an unconscious
// creature stops concentrating).
func TestAdjustingAConcentratingCharacterToZeroEndsTheConcentration(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	e = a.passTo(t, e, "Pensantus")
	a.h.roller.queue(1)
	if _, err := a.cast(t, a.ana, e, "Pensantus", holdPerson, slotOfLevel(2), a.at(t, "Goblin"), noCastRoll); err != nil {
		t.Fatalf("CastSpell(Hold Person) error = %v", err)
	}
	if got := byLabel(t, a.get(t, a.master), "Pensantus").GetConcentrationSpell(); got != holdPerson {
		t.Fatalf("Pensantus concentrates on %q, want %q", got, holdPerson)
	}

	a.correct(t, a.pens, hpIs(0))

	view := a.get(t, a.master)
	if got := byLabel(t, view, "Pensantus").GetConcentrationSpell(); got != "" {
		t.Errorf("Pensantus concentrates on %q at 0 hit points, want none", got)
	}
	if slices.Contains(byLabel(t, view, "Goblin").GetConditions(), "condition:paralyzed") {
		t.Errorf("the Goblin is still held by the spell its caster stopped concentrating on")
	}
	var told bool
	for _, r := range a.log(t, a.master, view).GetRounds() {
		for _, en := range r.GetEntries() {
			told = told || (en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_CONDITIONS_CHANGED && en.GetConcentrationEndedKey() == holdPerson)
		}
	}
	if !told {
		t.Error("the combat log has no line saying the concentration on Hold Person ended")
	}
}
