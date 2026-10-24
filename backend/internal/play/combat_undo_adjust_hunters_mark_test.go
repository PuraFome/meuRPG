package play

import "testing"

// Case (c) of the Hunter's Mark undo: the master brings an NPC that holds the mark (a
// concentration) to 0 hit points, which ends the concentration and takes the mark off its target;
// the undo of the adjustment gives the mark and the concentration back.
func TestUndoKindHitPointsAdjustedToZeroGivesAnNPCsHuntersMarkBack(t *testing.T) {
	t.Parallel()
	a := newHeroismCasters(t)
	e := a.castersFight(t, 1)
	markWithHuntersMark(t, a, e, "Capitão Goblin", "Toren")
	w := newUndoWatch(t, a)

	w.run(t, "an NPC holding Hunter's Mark brought to 0 hit points", nil, func() {
		if _, err := a.adjustHP(t, e, "Capitão Goblin", damageHP(1000)); err != nil {
			t.Fatalf("AdjustCombatantHitPoints(damage) error = %v", err)
		}
		view := a.get(t, a.master)
		if got := byLabel(t, view, "Capitão Goblin").GetConcentrationSpell(); got != "" {
			t.Fatalf("the Capitão concentrates on %q at 0 hit points, want none", got)
		}
		if huntersMarkOn(t, view, "Toren") != nil {
			t.Fatal("Toren is still marked after the NPC's concentration ended")
		}
	})
}
