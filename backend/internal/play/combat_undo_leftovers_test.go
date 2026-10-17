package play

import "testing"

// Finding fix/undo-leftovers: a rage that begins while the barbarian concentrates ends the
// concentration (SRD 5.1, Rage), and the undo of the rage gives it back.
func TestUndoKindActionTakenRageEndsAConcentrationAndTheUndoGivesItBack(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	a.concentrate(t, "Brisa", webSpell)
	w := newUndoWatch(t, a)

	w.run(t, "a rage while concentrating", nil, func() {
		if _, err := a.action(t, a.bia, e, "Brisa", "feature:rage"); err != nil {
			t.Fatalf("TakeAction(Rage) error = %v", err)
		}
		if got := byLabel(t, a.get(t, a.master), "Brisa").GetConcentrationSpell(); got != "" {
			t.Fatalf("Brisa concentrates on %q while raging, want none", got)
		}
	})
}
