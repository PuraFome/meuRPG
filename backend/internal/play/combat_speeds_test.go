package play

import (
	"testing"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
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

// One number everywhere: a swimmer in water with walk 30, swim 40 and 20 ft spent has 20 ft left, on the stream, on the Mover page's lead and in
// the economy of GetTurnOptions. The floor's formula answered 10 on the last.
func TestTheStreamTheMoverLeadAndTheTurnOptionsAgreeForASwimmerInWater(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.terrain.paintSurface(grid.SurfaceWater, grid.Square{Col: 6, Row: 7})
	e := c.fight(t)
	withSpeed("Toren", "speed_swim_ft", 40)(t, c)
	c.execSQL(t, `UPDATE combatants SET movement_used_dft = 200, movement_used_ft = 20 WHERE id = $1`, c.id(t, "Toren"))
	who := c.who(t, c.caio, "Toren")
	opts, err := c.options(t, c.caio, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions error = %v", err)
	}
	mv := c.mustOptions(t, c.caio, e, "Toren").GetOptions().GetEconomy().GetMovement()
	if who.GetMovementLeftDft() != 200 || opts.GetMovementLeftDft() != 200 || mv.GetLeftDft() != 200 {
		t.Errorf("left on the stream %d, on the Mover lead %d, in the turn options %d; want 200 on all three", who.GetMovementLeftDft(), opts.GetMovementLeftDft(), mv.GetLeftDft())
	}
	if who.GetSpeedDft() != 400 || mv.GetSpeedDft() != 400 || mv.GetLeftFt() != 20 || mv.GetUsedDft() != 200 {
		t.Errorf("speed on the stream %d and in the turn options %d, left %d ft, used %d; want 400, 400, 20, 200", who.GetSpeedDft(), mv.GetSpeedDft(), mv.GetLeftFt(), mv.GetUsedDft())
	}
}

// Standing up costs half the speed of the surface the creature stands on, against what is left on that surface (SRD 5.1 "Being Prone").
func TestAPronSwimmerInWaterStandsUpOnHalfItsSwimSpeed(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.terrain.paintSurface(grid.SurfaceWater, grid.Square{Col: 6, Row: 7})
	e := c.fight(t)
	withSpeed("Toren", "speed_swim_ft", 40)(t, c)
	c.setConditions(t, e, "Toren", condProne)
	if got := c.who(t, c.caio, "Toren").GetStandUpCostDft(); got != 200 {
		t.Fatalf("stand_up_cost_dft = %d, want 200 (half of the 40 ft swim speed)", got)
	}
	res, err := c.standUp(t, c.caio, e, "Toren")
	if err != nil {
		t.Fatalf("StandUp() error = %v", err)
	}
	if res.GetMovementLeftDft() != 200 || c.who(t, c.caio, "Toren").GetMovementUsedDft() != 200 {
		t.Errorf("after standing up: left %d, spent %d; want 200 and 200", res.GetMovementLeftDft(), c.who(t, c.caio, "Toren").GetMovementUsedDft())
	}
}

// The cost is checked against what is left on that surface: a swimmer that spent most of its swim speed cannot stand up in the water.
func TestAPronSwimmerWithTooLittleSwimSpeedLeftCannotStandUp(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.terrain.paintSurface(grid.SurfaceWater, grid.Square{Col: 6, Row: 7})
	e := c.fight(t)
	withSpeed("Toren", "speed_swim_ft", 40)(t, c)
	c.execSQL(t, `UPDATE combatants SET movement_used_dft = 250, movement_used_ft = 25 WHERE id = $1`, c.id(t, "Toren"))
	c.setConditions(t, e, "Toren", condProne)
	_, err := c.standUp(t, c.caio, e, "Toren")
	b := wantEncounterBlocked(t, err, reasonTooFar)
	if b.GetMissingDft() != 50 {
		t.Errorf("missing = %d dft, want 50 (200 to stand against 150 left of 400)", b.GetMissingDft())
	}
}

// After a swim the jump runs on the floor speed: what was spent comes off the walking speed (a jump is on foot).
func TestAJumpAfterASwimIsHeldToTheWalkingSpeed(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.terrain.paintSurface(grid.SurfaceWater, corridor(7, 8, 9)...)
	c.fight(t)
	withSpeed("Toren", "speed_swim_ft", 40)(t, c)
	c.mustMove(t, c.caio, "Toren", 9, 7) // 150 in water
	c.execSQL(t, `UPDATE combatants SET last_move_dft = 100 WHERE id = $1`, c.id(t, "Toren"))
	opts, err := c.jumpOptions(t, c.caio, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions(jump) error = %v", err)
	}
	if opts.GetMovementLeftDft() != 150 {
		t.Errorf("the jump's lead reads %d, want 150 (the walking speed less the 150 swum)", opts.GetMovementLeftDft())
	}
	if sq := reachableAt(opts, 12, 7); sq == nil || sq.GetCostDft() != 150 {
		t.Errorf("(12,7) of the jump = %v, want it on offer at 150 (the floor's 150 left)", sq)
	}
	if sq := reachableAt(opts, 13, 7); sq != nil {
		t.Errorf("(13,7) is on offer to the jump (%v); its 200 is more than the 150 left on the floor", sq)
	}
}

// The master's stream carries the real surface: a creature in water has its swim speed there, a positive control for the RN-10 row above.
func TestTheMastersStreamShowsTheRealSurfaceOfACreatureInTheDark(t *testing.T) {
	t.Parallel()
	f := newFogCave(t)
	f.torch(t, false)
	f.paint(t, mapsv1.MapLayer_MAP_LAYER_SURFACE, int32(grid.SurfaceWater), &mapsv1.MapSquare{Col: 18, Row: 5})
	f.fight(t)
	withSpeed("Goblin 1", "speed_swim_ft", 40)(t, f.cave)
	got := byLabel(t, f.get(t, f.master), "Goblin 1")
	if got.GetSpeedDft() != 400 || got.GetMovementLeftDft() != 400 {
		t.Errorf("the master reads speed %d, left %d for Goblin 1 in water; want 400 and 400", got.GetSpeedDft(), got.GetMovementLeftDft())
	}
}

// A drag divides the speeds and what was spent by its factor and keeps the whole of what is left a whole number of the line: with 1 spent
// of 300 a drag at half speed has 149 for the line, not 150 (150 of line costs 300 against 299 left).
func TestADragWithAnOddAmountSpentNeverPaysMoreThanIsLeft(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := dragFight(t, a, arenaPlan{})
	c.execSQL(t, `UPDATE combatants SET movement_used_dft = 1 WHERE id = $1`, c.id(t, "Toren"))
	opts := c.moveOptions(t, a.caio, "Toren")
	if opts.GetMovementLeftDft() != 149 {
		t.Errorf("movement left = %d, want 149 ((300 - 1) / 2)", opts.GetMovementLeftDft())
	}
	if r := reachableAt(opts, 3, 0); r != nil {
		t.Errorf("(3,0), three squares of line at 150, is on offer with 299 left at half speed: %v", r)
	}
	_, err := c.moveTo(t, a.caio, "Toren", 3, 0)
	wantEncounterBlocked(t, err, reasonTooFar)
	if got := c.combatant(t, a.caio, "Toren").GetMovementUsedDft(); got != 1 {
		t.Errorf("a refused drag left %d spent, want 1", got)
	}
}

// A legendary move runs on a budget of its own, a share of the floor's speed: a swim speed does not change it.
func TestALegendaryMoveOfASwimmerKeepsItsFloorBudget(t *testing.T) {
	t.Parallel()
	a, e, vamp := vampireFight(t)
	a.execSQL(t, `UPDATE combatants SET speed_swim_ft = 60 WHERE id = $1`, a.id(t, vamp))
	a.put(t, "Toren", 10, 6)
	e = a.passTo(t, e, "Toren") // a legendary action is taken on another creature's turn
	if _, err := a.legendaryMove(t, a.master, e, vamp, "move", 16, 5); err != nil {
		t.Fatalf("a legendary move of 30 ft error = %v; want it made (the control)", err)
	}
	_, err := a.legendaryMove(t, a.master, e, vamp, "move", 10, 5) // back: 30 ft again is within a fresh budget
	if err != nil {
		t.Fatalf("a second legendary move of 30 ft error = %v", err)
	}
	_, err = a.legendaryMove(t, a.master, e, vamp, "move", 17, 5) // 35 ft: past the floor's 30 ft whatever its swim speed
	wantBlockedBy(t, "a legendary move past the floor's speed", err, reasonTooFar)
}

// A Wild Shape swap mid-turn keeps what was spent: the form's speeds apply to it at once.
func TestAWildShapeSwapMidTurnKeepsWhatWasSpent(t *testing.T) {
	t.Parallel()
	s := newShapers(t)
	s.fight(t)
	s.execSQL(t, `UPDATE combatants SET movement_used_dft = 100, movement_used_ft = 10 WHERE id = $1`, s.id(t, "Sálvia"))
	res := s.mustAssume(t, s.bia, s.bri, "monster:crocodile")
	if got := byLabel(t, res.GetEncounter(), "Sálvia"); got.GetSpeedDft() != 200 || got.GetMovementUsedDft() != 100 || got.GetMovementLeftDft() != 100 {
		t.Errorf("the crocodile form reads speed %d, spent %d, left %d; want 200, 100 and 100 (walk 20, 10 ft spent)", got.GetSpeedDft(), got.GetMovementUsedDft(), got.GetMovementLeftDft())
	}
	left := s.mustLeave(t, s.bia, s.bri)
	if got := byLabel(t, left.GetEncounter(), "Sálvia"); got.GetSpeedDft() != 300 || got.GetMovementLeftDft() != 200 {
		t.Errorf("back in her own shape she reads speed %d, left %d; want 300 and 200", got.GetSpeedDft(), got.GetMovementLeftDft())
	}
}

// A long jump that lands in water says what is left at the speed of the surface it lands on, as the walk does: walk 30, swim 40 and a jump
// of 15 ft into water leave 25 ft of the swim speed, not the floor's 15 (the lead of the jump's options stays the floor's, which pays for it).
func TestALongJumpIntoWaterSaysWhatIsLeftAtTheSwimSpeed(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.terrain.paintSurface(grid.SurfaceWater, corridor(9)...)
	c.fight(t)
	withSpeed("Toren", "speed_swim_ft", 40)(t, c)
	c.mustMove(t, c.master, "Toren", 6, 7)
	c.execSQL(t, `UPDATE combatants SET last_move_dft = 100 WHERE id = $1`, c.id(t, "Toren"))
	opts, err := c.jumpOptions(t, c.caio, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions(jump) error = %v", err)
	}
	if opts.GetMovementLeftDft() != 300 {
		t.Errorf("the jump's lead reads %d, want 300 (the floor pays for a jump)", opts.GetMovementLeftDft())
	}
	if r := reachableAt(opts, 9, 7); r == nil || r.GetCostDft() != 150 || r.GetLeftAfterDft() != 250 {
		t.Errorf("the jump to (9,7) in the water = %v; want cost 150 and 250 left (400 of the swim speed)", r)
	}
	if r := reachableAt(opts, 8, 7); r == nil || r.GetCostDft() != 100 || r.GetLeftAfterDft() != 200 {
		t.Errorf("the jump to (8,7) on the floor = %v; want cost 100 and 200 left (the walking speed)", r)
	}
	c.mustMove(t, c.caio, "Toren", 9, 7, jumpTo(playv1.JumpKind_JUMP_KIND_LONG))
	if got := c.who(t, c.caio, "Toren").GetMovementLeftDft(); got != 250 {
		t.Errorf("after the jump the counter reads %d, want 250 (what the options said)", got)
	}
}
