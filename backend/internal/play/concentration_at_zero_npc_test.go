package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// concentrationEndedLines counts the lines of the master's log that say the concentration on the
// spell ended.
func concentrationEndedLines(t *testing.T, a *armed, spell string) int {
	t.Helper()
	n := 0
	for _, r := range a.log(t, a.master, a.get(t, a.master)).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_CONDITIONS_CHANGED && en.GetConcentrationEndedKey() == spell {
				n++
			}
		}
	}
	return n
}

// Finding fix/npc-zero-hp-concentration: an NPC that concentrates and whose hit points the master
// brings to 0 (AdjustCombatantHitPoints) stops concentrating, as a character does (SRD 5.1,
// "Duration": an unconscious creature stops concentrating); the log says so.
func TestAdjustingAConcentratingNPCToZeroEndsTheConcentration(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.concentrate(t, "Goblin", webSpell)

	if _, err := a.adjustHP(t, e, "Goblin", damageHP(1000)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints(damage) error = %v", err)
	}

	if got := byLabel(t, a.get(t, a.master), "Goblin").GetConcentrationSpell(); got != "" {
		t.Errorf("the Goblin concentrates on %q at 0 hit points, want none", got)
	}
	if n := concentrationEndedLines(t, a, webSpell); n != 1 {
		t.Errorf("the combat log has %d lines saying the concentration on Web ended, want 1", n)
	}
}

// Finding fix/npc-zero-hp-concentration: the undo of the adjustment gives the NPC its concentration
// back and takes the "ended" line away with it.
func TestUndoKindHitPointsAdjustedToZeroGivesTheNPCConcentrationBack(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.concentrate(t, "Goblin", webSpell)
	w := newUndoWatch(t, a)

	w.run(t, "an NPC concentrating brought to 0 hit points", nil, func() {
		if _, err := a.adjustHP(t, e, "Goblin", damageHP(1000)); err != nil {
			t.Fatalf("AdjustCombatantHitPoints(damage) error = %v", err)
		}
		if got := byLabel(t, a.get(t, a.master), "Goblin").GetConcentrationSpell(); got != "" {
			t.Fatalf("the Goblin concentrates on %q at 0 hit points", got)
		}
	})
}
