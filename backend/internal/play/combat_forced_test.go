package play

import (
	"testing"
)

// Forced movement (SRD 5.1, Thunderwave): a creature that fails the save is pushed 10 feet
// away from the caster, in whole squares of the map, stopping before a wall or another
// creature, with no opportunity attack, and the master's undo of the cast puts it back.
// These tests need the database (MEURPG_TEST_DATABASE_URL); the fixture is the cave of
// combat_move_test.go: Pensantus casts Thunderwave eastwards, the cube is the three
// columns next to him.

// waveFight is areaFight with Pensantus at (col, row) and Goblin 1 at goblin.
func (c *cave) waveFight(t *testing.T, caster, goblin [2]int32) {
	t.Helper()
	c.areaFight(t)
	c.mustMove(t, c.master, "Goblin 1", goblin[0], goblin[1])
	c.mustMove(t, c.master, "Pensantus", caster[0], caster[1])
}

func (c *cave) wave(t *testing.T) {
	t.Helper()
	c.mustCastArea(t, c.ana, "Pensantus", thunderwave, slotOfLevel(1), nil, toward(1, 0))
}

func wantSquare(t *testing.T, c *cave, label string, want [2]int32) {
	t.Helper()
	if got := c.square(t, label); got != want {
		t.Errorf("%s stands at %v, want %v", label, got, want)
	}
}

// TestThunderwavePushesAFailedSaveTenFeetAway: Goblin 1 beside the caster fails the save and
// goes two squares east; a goblin outside the cube stays; the undo of the cast brings it back.
func TestThunderwavePushesAFailedSaveTenFeetAway(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.waveFight(t, [2]int32{17, 7}, [2]int32{18, 7})

	c.wave(t)
	wantSquare(t, c, "Goblin 1", [2]int32{20, 7})
	wantSquare(t, c, "Goblin 2", [2]int32{22, 8})

	c.undoLast(t)
	wantSquare(t, c, "Goblin 1", [2]int32{18, 7})
}

// TestThunderwavePushStopsBeforeAWall: the cave's east wall is at column 23, so a goblin at
// column 21 can only go one square.
func TestThunderwavePushStopsBeforeAWall(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.waveFight(t, [2]int32{20, 6}, [2]int32{21, 6})

	c.wave(t)
	wantSquare(t, c, "Goblin 1", [2]int32{22, 6})
}

// TestThunderwavePushStopsBeforeACreature: Toren stands two squares behind the goblin, outside
// the cube, so the goblin stops in the square before him.
func TestThunderwavePushStopsBeforeACreature(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.waveFight(t, [2]int32{17, 6}, [2]int32{19, 6})
	c.mustMove(t, c.master, "Toren", 21, 6)

	c.wave(t)
	wantSquare(t, c, "Goblin 1", [2]int32{20, 6})
	wantSquare(t, c, "Toren", [2]int32{21, 6})
}

// TestThunderwavePushProvokesNothing: the goblin leaves the reach of Brisa without
// an offer of an opportunity attack.
func TestThunderwavePushProvokesNothing(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.waveFight(t, [2]int32{17, 6}, [2]int32{18, 6})
	c.mustMove(t, c.master, "Brisa", 17, 5) // beside the goblin and out of the cube: the push leaves her reach
	c.wave(t)
	wantSquare(t, c, "Goblin 1", [2]int32{20, 6})
	if got := len(c.get(t, c.master).GetOpportunityOffers()); got != 0 {
		t.Errorf("opportunity offers = %d, want none for a forced move", got)
	}
}
