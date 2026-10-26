package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// multiattackFight is a theatre combat with one creature of the bestiary (named name) on turn, with
// Toren and Pensantus as its targets.
func multiattackFight(t *testing.T, creature, name string) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newArmed(t)
	e := a.monsterSetup(t, true)
	a.h.roller.queue(20)
	a.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) { r.CreatureKey, r.Count, r.Name = creature, 1, name })
	e = a.begin(t, a.get(t, a.master))
	return a, e
}

func (a *armed) attacksLeft(t *testing.T, e *playv1.Encounter, label string) int {
	t.Helper()
	return int(a.mustOptions(t, a.master, e, label).GetOptions().GetEconomy().GetAttacksLeft())
}

// MR-042: a creature with a Multiattack (the animated armor's two slams) makes the attacks of the
// routine one after another, each with its own target; the server counts them and refuses one more
// than the routine has, to the master too.
func TestMR042_MultiattackAllowsTheRoutineAndRefusesOneMore(t *testing.T) {
	t.Parallel()
	a, e := multiattackFight(t, "monster:animated-armor", "Armadura")
	if left := a.attacksLeft(t, e, "Armadura"); left != 2 {
		t.Fatalf("attacks left at the start = %d, want 2", left)
	}
	a.mustAttack(t, a.master, e, "Armadura", sword, "Toren", d20(1))
	if left := a.attacksLeft(t, e, "Armadura"); left != 1 {
		t.Fatalf("attacks left after the first slam = %d, want 1", left)
	}
	a.mustAttack(t, a.master, e, "Armadura", sword, "Pensantus", d20(1)) // another target
	if left := a.attacksLeft(t, e, "Armadura"); left != 0 {
		t.Fatalf("attacks left after the second slam = %d, want 0", left)
	}
	_, err := a.attack(t, a.master, e, "Armadura", sword, "Toren", d20(1))
	wantBlockedBy(t, "a third slam", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ATTACKS_USED)
}

// The log says each attack of the routine, and the undo takes back the last one only: the second
// slam is offered again, the first stays.
func TestMR042_MultiattackLogsEachAttackAndUndoTakesBackTheLast(t *testing.T) {
	t.Parallel()
	a, e := multiattackFight(t, "monster:animated-armor", "Armadura")
	a.mustAttack(t, a.master, e, "Armadura", sword, "Toren", d20(1))
	a.mustAttack(t, a.master, e, "Armadura", sword, "Pensantus", d20(1))
	attackLines := func() int {
		n := 0
		for _, r := range a.log(t, a.master, e).GetRounds() {
			for _, en := range r.GetEntries() {
				if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK {
					n++
				}
			}
		}
		return n
	}
	if n := attackLines(); n != 2 {
		t.Fatalf("the log has %d attack lines, want 2", n)
	}
	if err := a.undo(t, a.master, e, a.log(t, a.master, e).GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	if left := a.attacksLeft(t, e, "Armadura"); left != 1 {
		t.Errorf("attacks left after the undo = %d, want 1 (only the last attack is taken back)", left)
	}
	if n := attackLines(); n != 1 {
		t.Errorf("the log has %d attack lines after the undo, want 1", n)
	}
	a.mustAttack(t, a.master, e, "Armadura", sword, "Toren", d20(1)) // the second slam again
}

// A creature without a Multiattack works as before: the master is not held to a count.
func TestMR042_NoMultiattackLeavesTheMasterFree(t *testing.T) {
	t.Parallel()
	a, e := multiattackFight(t, "monster:skeleton", "Esqueleto")
	for i := 1; i <= 3; i++ {
		if _, err := a.attack(t, a.master, e, "Esqueleto", sword, "Toren", d20(1)); err != nil {
			t.Fatalf("attack %d of a creature with no Multiattack error = %v, want none", i, err)
		}
	}
}
