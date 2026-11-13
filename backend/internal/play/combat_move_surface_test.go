package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// The cost of swimming and climbing (#589, SRD 5.1 "Climbing, Swimming, and Crawling"): a foot of water or of a climbable surface
// costs 1 extra foot unless the mover has the matching speed. Toren stands on (6,7) with 30 ft, and the corridor of row 7 is floor.

func corridor(cols ...int) []grid.Square {
	out := make([]grid.Square, 0, len(cols))
	for _, c := range cols {
		out = append(out, grid.Square{Col: c, Row: 7})
	}
	return out
}

func withSpeed(label, column string, ft int) func(t *testing.T, c *cave) {
	return func(t *testing.T, c *cave) {
		t.Helper()
		c.execSQL(t, `UPDATE combatants SET `+column+` = $2 WHERE id = $1`, c.id(t, label), ft)
	}
}

func reachableAt(res *playv1.GetMoveOptionsResponse, col, row int32) *playv1.ReachableSquare {
	for _, r := range res.GetReachable() {
		if r.GetCol() == col && r.GetRow() == row {
			return r
		}
	}
	return nil
}

func TestSwimmingCostsAnExtraFootPerFoot(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		surface      grid.Surface
		setup        func(t *testing.T, c *cave)
		used, extra  int32
		wantSurfaceX playv1.SurfaceExtra
	}{
		{"water, no swim speed", grid.SurfaceWater, nil, 300, 150, playv1.SurfaceExtra_SURFACE_EXTRA_SWIM},
		{"water, with a swim speed", grid.SurfaceWater, withSpeed("Toren", "speed_swim_ft", 30), 150, 0, playv1.SurfaceExtra_SURFACE_EXTRA_UNSPECIFIED},
		{"water, a climb speed does not help", grid.SurfaceWater, withSpeed("Toren", "speed_climb_ft", 30), 300, 150, playv1.SurfaceExtra_SURFACE_EXTRA_SWIM},
		{"climb, no climb speed", grid.SurfaceClimb, nil, 300, 150, playv1.SurfaceExtra_SURFACE_EXTRA_CLIMB},
		{"climb, with a climb speed", grid.SurfaceClimb, withSpeed("Toren", "speed_climb_ft", 20), 150, 0, playv1.SurfaceExtra_SURFACE_EXTRA_UNSPECIFIED},
		{"climb, a swim speed does not help", grid.SurfaceClimb, withSpeed("Toren", "speed_swim_ft", 30), 300, 150, playv1.SurfaceExtra_SURFACE_EXTRA_CLIMB},
		{"a flier crosses it free", grid.SurfaceWater, flying("Toren", 30), 150, 0, playv1.SurfaceExtra_SURFACE_EXTRA_UNSPECIFIED},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := newCave(t)
			c.terrain.paintSurface(tc.surface, corridor(7, 8, 9)...)
			c.fight(t)
			if tc.setup != nil {
				tc.setup(t, c)
			}
			// GetMoveOptions says what the move will charge, and why.
			opts, err := c.options(t, c.caio, "Toren")
			if err != nil {
				t.Fatalf("GetMoveOptions error = %v", err)
			}
			r := reachableAt(opts, 9, 7)
			if r == nil || r.GetCostDft() != tc.used || r.GetExtraDft() != tc.extra || r.GetSurfaceExtra() != tc.wantSurfaceX || r.GetCrawlExtraDft() != 0 {
				t.Fatalf("the options say (9,7) = %v; want cost %d, extra %d, %v, no crawl", r, tc.used, tc.extra, tc.wantSurfaceX)
			}
			c.mustMove(t, c.caio, "Toren", 9, 7)
			who := c.who(t, c.caio, "Toren")
			if who.GetMovementUsedDft() != tc.used {
				t.Errorf("the move spent %d, want %d (what the options said)", who.GetMovementUsedDft(), tc.used)
			}
		})
	}
}

// Swimming past what is left is TOO_FAR with the real missing cost; the master's free moves stay free.
func TestSwimmingPastTheMovementIsTooFarAndTheMastersMoveIsFree(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.terrain.paintSurface(grid.SurfaceWater, corridor(7, 8, 9, 10)...)
	c.fight(t)
	_, err := c.move(t, c.caio, "Toren", 10, 7) // 20 ft of water: 40 ft against 30
	b := wantEncounterBlocked(t, err, reasonTooFar)
	if b.GetMissingDft() != 100 {
		t.Errorf("missing = %d dft, want 100 (400 against 300)", b.GetMissingDft())
	}
	if got := c.who(t, c.caio, "Toren"); got.GetMovementUsedDft() != 0 || got.GetCol() != 6 {
		t.Errorf("a refused swim left Toren on %d with %d spent", got.GetCol(), got.GetMovementUsedDft())
	}
	c.mustMove(t, c.master, "Toren", 10, 7) // the master's move is free
	if got := c.who(t, c.master, "Toren"); got.GetMovementUsedDft() != 0 || got.GetCol() != 10 {
		t.Errorf("the master's move of Toren: col %d, spent %d; want 10 and 0", got.GetCol(), got.GetMovementUsedDft())
	}
}

// An NPC the master drags on its own turn pays like a player's, water included.
func TestAnNPCPaysForTheWaterOnItsTurn(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.fight(t)
	c.passTo(t, c.get(t, c.master), "Goblin 1")
	from := c.who(t, c.master, "Goblin 1")
	col, row := from.GetCol(), from.GetRow()
	c.terrain.paintSurface(grid.SurfaceWater, grid.Square{Col: int(col) + 1, Row: int(row)}, grid.Square{Col: int(col) + 2, Row: int(row)})
	if _, err := c.move(t, c.master, "Goblin 1", col+2, row); err != nil {
		t.Fatalf("the goblin cannot walk two squares east: %v", err)
	}
	if got := c.who(t, c.master, "Goblin 1"); got.GetMovementUsedDft() != 200 {
		t.Errorf("the goblin swam 10 ft and spent %d, want 200", got.GetMovementUsedDft())
	}
}

// Crawling adds to the other extras (SRD 5.1): 3 a foot over rubble, 3 a foot in water, not 4.
func TestCrawlingAddsToTheOtherExtras(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.terrain.paintDifficult(corridor(7, 8)...)
	c.fight(t)
	conditioned("Toren", "condition:prone")(t, c)
	c.mustMove(t, c.caio, "Toren", 8, 7) // 10 ft: the line, the rubble and the crawl
	if got := c.who(t, c.caio, "Toren"); got.GetMovementUsedDft() != 100+100+100 {
		t.Errorf("a crawl of 10 ft over rubble spent %d, want 300 (3 a foot)", got.GetMovementUsedDft())
	}

	w := newCave(t)
	w.terrain.paintSurface(grid.SurfaceWater, corridor(7, 8)...)
	w.fight(t)
	conditioned("Toren", "condition:prone")(t, w)
	opts, err := w.options(t, w.caio, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions error = %v", err)
	}
	// The two extras come apart: the swim and the crawl are each 10 ft of the 30.
	if r := reachableAt(opts, 8, 7); r == nil || r.GetCostDft() != 300 || r.GetExtraDft() != 100 || r.GetSurfaceExtra() != playv1.SurfaceExtra_SURFACE_EXTRA_SWIM || r.GetCrawlExtraDft() != 100 {
		t.Errorf("the options for a crawl in water = %v, want cost 300 with 100 of swim and 100 of crawl", r)
	}
	w.mustMove(t, w.caio, "Toren", 8, 7)
	if got := w.who(t, w.caio, "Toren"); got.GetMovementUsedDft() != 300 {
		t.Errorf("a crawl of 10 ft in water spent %d, want 300", got.GetMovementUsedDft())
	}
}

// A jump clears the water: its cost is its length (5 ft: a standing jump goes 8 ft).
func TestAJumpOverWaterCostsItsLength(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.terrain.paintSurface(grid.SurfaceWater, corridor(7, 8)...)
	c.fight(t)
	c.mustMove(t, c.caio, "Toren", 7, 7, jumpTo(playv1.JumpKind_JUMP_KIND_LONG))
	if got := c.who(t, c.caio, "Toren"); got.GetMovementUsedDft() != 50 {
		t.Errorf("a long jump into water spent %d, want 50 (its length)", got.GetMovementUsedDft())
	}
}

// A creature with no walking and no flying speed moves on its best swimming or climbing speed; one that walks reads its
// walking speed where it stands on the floor, and its swim speed only in the water (#819).
func TestThePoolOfASwimmerAndAWalker(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.fight(t)
	withSpeed("Toren", "speed_ft", 0)(t, c)
	withSpeed("Toren", "speed_swim_ft", 40)(t, c)
	withSpeed("Toren", "speed_climb_ft", 20)(t, c)
	if got := c.who(t, c.master, "Toren"); got.GetSpeedDft() != 400 {
		t.Errorf("a swimmer with no walking speed has %d, want 400 (its best of swim and climb)", got.GetSpeedDft())
	}
	withSpeed("Toren", "speed_ft", 30)(t, c)
	if got := c.who(t, c.master, "Toren"); got.GetSpeedDft() != 300 {
		t.Errorf("a walker with a swim speed of 40 has %d, want 300 (the walking speed)", got.GetSpeedDft())
	}
}
