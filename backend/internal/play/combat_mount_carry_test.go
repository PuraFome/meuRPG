package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// seat puts a rider on the mount's square as the table keeps it, for the tests of what carries it.
func (c *cx) seat(t *testing.T, rider, mount string) {
	t.Helper()
	if _, err := c.h.pool.Exec(t.Context(), `UPDATE combatants SET mounted_on_id = $1, grid_col = m.grid_col, grid_row = m.grid_row FROM combatants m WHERE m.id = $1 AND combatants.id = $2`,
		c.id(t, mount), c.id(t, rider)); err != nil {
		t.Fatalf("seat %s on %s: %v", rider, mount, err)
	}
}

// TestMountedCombat_AShoveAndADragCarryTheRider: a creature that is shoved or dragged takes the rider on its back along, and the pair
// stays on one square.
func TestMountedCombat_AShoveAndADragCarryTheRider(t *testing.T) {
	t.Parallel()
	t.Run("a shove", func(t *testing.T) {
		t.Parallel()
		a := newArmed(t)
		c := a.arena(t, arenaPlan{})
		c.seat(t, "Goblin", c.hob)
		started := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
			c.h.roller.queue(18)
			r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), playv1.ContestPurpose_CONTEST_PURPOSE_SHOVE
		})
		view := c.mustRespond(t, a.master, started.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, 2).GetContest()
		c.mustShove(t, a.caio, view.GetId(), playv1.ShoveOutcome_SHOVE_OUTCOME_PUSH)
		if hob, rider := c.square(t, c.hob), c.square(t, "Goblin"); hob != [2]int32{5, 3} || rider != hob {
			t.Errorf("the Hobgoblin is on %v and its rider on %v, want both on (5,3)", hob, rider)
		}
	})
	t.Run("a drag", func(t *testing.T) {
		t.Parallel()
		a := newArmed(t)
		c := dragFight(t, a, arenaPlan{})
		c.seat(t, "Goblin", c.hob)
		if _, err := c.moveTo(t, a.caio, "Toren", 3, 0); err != nil {
			t.Fatalf("a three-square drag: %v", err)
		}
		if hob, rider := c.square(t, c.hob), c.square(t, "Goblin"); hob != [2]int32{3, 1} || rider != hob {
			t.Errorf("the dragged mount is on %v and its rider on %v, want both on (3,1)", hob, rider)
		}
	})
}
