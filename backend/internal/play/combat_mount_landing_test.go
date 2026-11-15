package play

import "testing"

// A failed save tells the fog and the streams of the fall once, and a mount with no square has no landing square to find.

func TestMountedCombat_AFailedSaveTellsTheFogOnce(t *testing.T) {
	t.Parallel()
	c := riddenCave(t)
	fog := &dragFogRecorder{}
	c.h.svc.SetFog(fog)
	c.askSave(t, "Toren", causePushed)
	fog.reset()
	c.mustRollSave(t, c.caio, "Toren", saveFace(1))
	fog.mu.Lock()
	n := len(fog.moved)
	fog.mu.Unlock()
	if n != 1 {
		t.Errorf("the fog was told %d moves for one fall, want 1", n)
	}
}

func TestMountedCombat_AMountWithNoSquareLandsItsRiderWithoutAPanic(t *testing.T) {
	t.Parallel()
	t.Run("a failed save", func(t *testing.T) {
		t.Parallel()
		c := riddenCave(t)
		c.askSave(t, "Toren", causePushed)
		c.execSQL(t, `UPDATE combatants SET grid_col = NULL, grid_row = NULL WHERE id = $1`, c.id(t, "Ogro"))
		c.mustRollSave(t, c.caio, "Toren", saveFace(1))
		if c.mountedOn(t, "Toren") != nil {
			t.Error("the rider is still on a mount it failed the save for")
		}
	})
	t.Run("a rider that falls at once", func(t *testing.T) {
		t.Parallel()
		c := riddenCave(t)
		c.execSQL(t, `UPDATE combatants SET reaction_used = true WHERE id = $1`, c.id(t, "Toren"))
		c.execSQL(t, `UPDATE combatants SET grid_col = NULL, grid_row = NULL WHERE id = $1`, c.id(t, "Ogro"))
		if _, err := c.adjustHP(t, c.get(t, c.master), "Ogro", damageHP(500)); err != nil {
			t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
		}
		if c.mountedOn(t, "Toren") != nil {
			t.Error("the rider is still on a defeated mount")
		}
	})
}
