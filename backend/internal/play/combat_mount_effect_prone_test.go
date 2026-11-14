package play

import "testing"

// A lasting effect that knocks a combatant prone (Hideous Laughter) opens the dismount window like every other source of the prone.

func TestMountedCombat_AMountLaughedProneAsksItsRider(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.execSQL(t, `UPDATE combatants SET mounted_on_id = $1, grid_col = m.grid_col, grid_row = m.grid_row FROM combatants m WHERE m.id = $1 AND combatants.id = $2`, a.id(t, "Goblin"), a.id(t, "Toren"))
	a.mustAddEffect(t, e, "spell:hideous-laughter", []string{"Goblin"}, a.rounds(t, 10, "Pensantus"))
	if a.windowOf(t, a.caio, reactionDismount) == nil {
		t.Error("Toren reads no dismount window after the Goblin was laughed prone under him")
	}
	if a.mountedOn(t, "Toren") == nil {
		t.Error("the rider was thrown before it answered")
	}
}

func TestMountedCombat_ARiderLaughedProneAsksItsSave(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.execSQL(t, `UPDATE combatants SET mounted_on_id = $1, grid_col = m.grid_col, grid_row = m.grid_row FROM combatants m WHERE m.id = $1 AND combatants.id = $2`, a.id(t, "Goblin"), a.id(t, "Toren"))
	a.mustAddEffect(t, e, "spell:hideous-laughter", []string{"Toren"}, a.rounds(t, 10, "Pensantus"))
	if a.mountSaveOf(t, a.caio) == nil {
		t.Error("Toren reads no mount save after he was laughed prone on his mount")
	}
}

func TestMountedCombat_AMountThatWasAlreadyProneOpensNoSecondWindow(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.execSQL(t, `UPDATE combatants SET mounted_on_id = $1, grid_col = m.grid_col, grid_row = m.grid_row FROM combatants m WHERE m.id = $1 AND combatants.id = $2`, a.id(t, "Goblin"), a.id(t, "Toren"))
	a.execSQL(t, `UPDATE combatants SET conditions = ARRAY['condition:prone'] WHERE id = $1`, a.id(t, "Goblin"))
	a.mustAddEffect(t, e, "spell:hideous-laughter", []string{"Goblin"}, a.rounds(t, 10, "Pensantus"))
	if a.windowOf(t, a.caio, reactionDismount) != nil {
		t.Error("a mount that was prone already opened a dismount window")
	}
}
