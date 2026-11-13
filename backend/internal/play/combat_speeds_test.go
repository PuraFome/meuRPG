package play

import (
	"testing"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// Using different speeds (#819, SRD 5.1 "Using Different Speeds"): each speed is the limit on its surface, and what was spent at one
// counts against the next. Toren stands on (6,7) with 30 ft of walking; the corridor of row 7 is floor unless a test paints it.

func TestASwimSpeedPastTheWalkingSpeedCarriesTheLineThroughTheWater(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.terrain.paintSurface(grid.SurfaceWater, corridor(8, 9, 10, 11, 12, 13)...)
	c.fight(t)
	withSpeed("Toren", "speed_swim_ft", 40)(t, c)
	opts, err := c.options(t, c.caio, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions error = %v", err)
	}
	r := reachableAt(opts, 13, 7)
	if r == nil || r.GetCostDft() != 350 || r.GetLeftAfterDft() != 50 {
		t.Fatalf("the options say (13,7) = %v; want cost 350 and 50 left (swim 400)", r)
	}
	if r := reachableAt(opts, 14, 7); r != nil {
		t.Errorf("(14,7) is on offer (%v); its 400 plus a floor is over the walking speed", r)
	}
	c.mustMove(t, c.caio, "Toren", 13, 7)
	who := c.who(t, c.caio, "Toren")
	if who.GetMovementUsedDft() != 350 {
		t.Errorf("the move spent %d, want 350 (what the options said)", who.GetMovementUsedDft())
	}
	// He stands in the water now: the counter is what is left at his swim speed, not at his walking one.
	if who.GetSpeedDft() != 400 || who.GetMovementLeftDft() != 50 {
		t.Errorf("standing in the water he reads speed %d, left %d; want 400 and 50", who.GetSpeedDft(), who.GetMovementLeftDft())
	}
}

func TestSteppingOutOfTheWaterIsHeldToTheWalkingSpeedLessWhatWasSpent(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.terrain.paintSurface(grid.SurfaceWater, corridor(8, 9)...)
	c.fight(t)
	withSpeed("Toren", "speed_swim_ft", 40)(t, c)
	opts, err := c.options(t, c.caio, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions error = %v", err)
	}
	if r := reachableAt(opts, 12, 7); r == nil || r.GetCostDft() != 300 {
		t.Errorf("the options say (12,7) = %v; want cost 300 (the walking speed ends the line there)", r)
	}
	if r := reachableAt(opts, 13, 7); r != nil {
		t.Errorf("(13,7) is on offer (%v); 350 on the floor is past the walking speed", r)
	}
	_, err = c.move(t, c.caio, "Toren", 13, 7)
	b := wantEncounterBlocked(t, err, reasonTooFar)
	if b.GetMissingDft() != 50 {
		t.Errorf("missing = %d dft, want 50", b.GetMissingDft())
	}
}

func TestWithoutASwimSpeedTheWaterKeepsItsExtraAndTheWalkingSpeedIsTheLimit(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.terrain.paintSurface(grid.SurfaceWater, corridor(8, 9, 10, 11)...)
	c.fight(t)
	opts, err := c.options(t, c.caio, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions error = %v", err)
	}
	if r := reachableAt(opts, 9, 7); r == nil || r.GetCostDft() != 250 || r.GetExtraDft() != 100 || r.GetLeftAfterDft() != 50 {
		t.Errorf("the options say (9,7) = %v; want 250 with 100 of extra and 50 left", r)
	}
	if r := reachableAt(opts, 10, 7); r != nil {
		t.Errorf("(10,7) is on offer (%v)", r)
	}
}

// Two moves: the first is on the floor, and the second, in the water, pays against the swim speed with what the first spent.
func TestWhatTheFirstMoveSpentCountsAgainstTheSwimSpeedOfTheSecond(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.terrain.paintSurface(grid.SurfaceWater, corridor(8, 9, 10, 11, 12, 13, 14)...)
	c.fight(t)
	withSpeed("Toren", "speed_swim_ft", 40)(t, c)
	c.mustMove(t, c.caio, "Toren", 7, 7) // 50 on the floor
	opts, err := c.options(t, c.caio, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions error = %v", err)
	}
	if r := reachableAt(opts, 14, 7); r == nil || r.GetCostDft() != 350 || r.GetLeftAfterDft() != 0 {
		t.Errorf("the options say (14,7) = %v; want 350 (all in water) and nothing left of 400", r)
	}
	if got := opts.GetMovementLeftDft(); got != 250 {
		t.Errorf("the options lead reads %d, want 250 (the walking speed on the floor he stands on)", got)
	}
	if r := reachableAt(opts, 15, 7); r != nil {
		t.Errorf("(15,7) is on offer (%v); it is over", r)
	}
}

// The master's options and the player's stream both say what is left where the combatant stands, and a swimmer on dry land never reads its swim speed.
func TestASwimmerOnDryLandReadsItsWalkingSpeed(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.fight(t)
	withSpeed("Toren", "speed_swim_ft", 40)(t, c)
	who := c.who(t, c.caio, "Toren")
	if who.GetSpeedDft() != 300 || who.GetMovementLeftDft() != 300 {
		t.Errorf("on the floor he reads speed %d, left %d; want 300 and 300", who.GetSpeedDft(), who.GetMovementLeftDft())
	}
	opts, err := c.options(t, c.master, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions error = %v", err)
	}
	if opts.GetMovementLeftDft() != 300 {
		t.Errorf("the options lead reads %d, want 300", opts.GetMovementLeftDft())
	}
}

// RN-10: the stream and the plan never reflect a surface the player has not seen. Toren is in the dark with a swim speed of 40: the
// water ahead of him is floor to him, so his counter reads the walking speed, and the master's reads the same (he stands on the floor).
// Once the master puts him in the water, his own square is his to know and the counter is the swim speed; the master's copy agrees.
func TestRN10_TheCounterNeverReflectsWaterThePlayerHasNotSeen(t *testing.T) {
	t.Parallel()
	f := newFogCave(t)
	f.torch(t, false)
	f.paint(t, mapsv1.MapLayer_MAP_LAYER_SURFACE, int32(grid.SurfaceWater), &mapsv1.MapSquare{Col: 7, Row: 7}, &mapsv1.MapSquare{Col: 8, Row: 7}, &mapsv1.MapSquare{Col: 9, Row: 7})
	f.fight(t)
	withSpeed("Toren", "speed_swim_ft", 40)(t, f.cave)

	for _, u := range []*user{f.master, f.caio} {
		if got := byLabel(t, f.get(t, u), "Toren"); got.GetSpeedDft() != 300 || got.GetMovementLeftDft() != 300 {
			t.Errorf("Toren on the floor reads speed %d, left %d; want 300 and 300", got.GetSpeedDft(), got.GetMovementLeftDft())
		}
	}
	plan, err := f.options(t, f.caio, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions error = %v", err)
	}
	if plan.GetMovementLeftDft() != 300 {
		t.Errorf("the plan's lead reads %d, want 300", plan.GetMovementLeftDft())
	}
	if r := reachableAt(plan, 8, 7); r == nil || r.GetCostDft() != 100 || r.GetExtraDft() != 0 || r.GetLeftAfterDft() != 200 {
		t.Errorf("the plan for (8,7) = %v; want 100, no extra and 200 left: the floor", r)
	}
	f.mustMove(t, f.master, "Toren", 8, 7) // the master's move is free
	for _, u := range []*user{f.master, f.caio} {
		if got := byLabel(t, f.get(t, u), "Toren"); got.GetSpeedDft() != 400 || got.GetMovementLeftDft() != 400 {
			t.Errorf("Toren in the water reads speed %d, left %d; want 400 and 400", got.GetSpeedDft(), got.GetMovementLeftDft())
		}
	}
}

// The Dash doubles each speed: a swimmer that dashed has twice its swim speed in the water and twice its walking speed on the floor.
func TestTheDashDoublesTheSwimSpeedToo(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.terrain.paintSurface(grid.SurfaceWater, corridor(7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18)...)
	c.fight(t)
	withSpeed("Toren", "speed_swim_ft", 40)(t, c)
	c.execSQL(t, `UPDATE combatants SET dashed = true WHERE id = $1`, c.id(t, "Toren"))
	opts, err := c.options(t, c.caio, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions error = %v", err)
	}
	if r := reachableAt(opts, 18, 7); r == nil || r.GetCostDft() != 600 || r.GetLeftAfterDft() != 200 {
		t.Errorf("the options say (18,7) = %v; want 600 and 200 left of 800", r)
	}
	c.mustMove(t, c.caio, "Toren", 18, 7)
	if who := c.who(t, c.caio, "Toren"); who.GetMovementUsedDft() != 600 || who.GetMovementLeftDft() != 200 {
		t.Errorf("after the move he spent %d with %d left; want 600 and 200", who.GetMovementUsedDft(), who.GetMovementLeftDft())
	}
}

// A creature with no walking speed (the shark) has the better of its swimming and climbing speeds on the floor, and its swim speed in the water.
func TestACreatureWithNoWalkingSpeedKeepsItsFallbackOnTheFloor(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.terrain.paintSurface(grid.SurfaceWater, corridor(8, 9)...)
	c.fight(t)
	withSpeed("Toren", "speed_ft", 0)(t, c)
	withSpeed("Toren", "speed_swim_ft", 40)(t, c)
	withSpeed("Toren", "speed_climb_ft", 20)(t, c)
	opts, err := c.options(t, c.caio, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions error = %v", err)
	}
	if opts.GetMovementLeftDft() != 400 {
		t.Errorf("the lead reads %d, want 400 (its best of swim and climb)", opts.GetMovementLeftDft())
	}
	if r := reachableAt(opts, 9, 7); r == nil || r.GetCostDft() != 150 || r.GetLeftAfterDft() != 250 {
		t.Errorf("the options say (9,7) = %v; want 150 and 250 left (a swim speed lifts the extra)", r)
	}
}
