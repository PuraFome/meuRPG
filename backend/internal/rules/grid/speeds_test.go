package grid_test

import (
	"math"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// A 12 x 1 corridor whose squares 3 to 8 are water: two floor squares, six of water, then floor (#819).
func waterCorridor() grid.Terrain {
	g := grid.Grid{Columns: 12, Rows: 1}
	sf := grid.NewSurfaceLayer(g)
	for col := 3; col <= 8; col++ {
		sf.Set(col, 0, grid.SurfaceWater)
	}
	return grid.Terrain{Grid: g, Surface: sf}
}

// swimmer walks 30 ft and swims 40 ft; plain walks 30 ft and has no swim speed, so its water costs the extra foot at the walking speed.
func swimmer(used int) grid.Mover {
	return grid.Mover{Swims: true, Speeds: &grid.Speeds{UsedDFt: used, FloorDFt: 300, WaterDFt: 400, ClimbDFt: 300}}
}

func plain(used int) grid.Mover {
	return grid.Mover{Speeds: &grid.Speeds{UsedDFt: used, FloorDFt: 300, WaterDFt: 300, ClimbDFt: 300}}
}

func at(col int) grid.Square { return grid.Square{Col: col, Row: 0} }

// SRD 5.1, "Using Different Speeds": a swim speed above the walking one is the limit in the water, and a step back on the ground is
// held to the walking speed again, counted from what was spent.
func TestASwimSpeedPastTheWalkSpeedHoldsInTheWater(t *testing.T) {
	t.Parallel()
	tr := waterCorridor()
	mv := tr.Move(at(0), at(8), nil, swimmer(0))
	if mv.Blocked || mv.CostDFt != 400 || mv.OverDFt != 0 || mv.LeftAfterDFt != 0 {
		t.Fatalf("2 floor and 6 water squares at swim 40: cost %d, over %d, left after %d; want 400, 0 and 0", mv.CostDFt, mv.OverDFt, mv.LeftAfterDFt)
	}
	// One square more is floor again: 450 spent against the walking 300.
	if mv := tr.Move(at(0), at(9), nil, swimmer(0)); mv.OverDFt != 150 {
		t.Errorf("stepping out of the water after 40 ft is over by %d, want 150", mv.OverDFt)
	}
	// The last floor square before the water is as far as the walking speed goes on the ground.
	if mv := tr.Move(at(0), at(2), nil, swimmer(0)); mv.OverDFt != 0 || mv.LeftAfterDFt != 200 {
		t.Errorf("on the floor: over %d, left after %d; want 0 and 200", mv.OverDFt, mv.LeftAfterDFt)
	}
	// Coming out of the water with enough left: 15 ft on the floor, 5 of water would be 20, then the floor is within 30.
	if mv := tr.Move(at(2), at(3), nil, swimmer(0)); mv.LeftAfterDFt != 350 {
		t.Errorf("one square into the water: left after %d, want 350 (the swim speed less what is spent)", mv.LeftAfterDFt)
	}
}

// Without a swim speed the water costs the extra foot and the limit is the walking speed, as before #819.
func TestWithoutASwimSpeedTheWaterIsHeldToTheWalkingSpeed(t *testing.T) {
	t.Parallel()
	tr := waterCorridor()
	for _, c := range []struct{ to, cost, over int }{{3, 200, 0}, {4, 300, 0}, {5, 400, 100}} {
		mv := tr.Move(at(0), at(c.to), nil, plain(0))
		if mv.CostDFt != c.cost || mv.OverDFt != c.over {
			t.Errorf("to %d: cost %d over %d; want %d and %d", c.to, mv.CostDFt, mv.OverDFt, c.cost, c.over)
		}
	}
}

// Switching subtracts what was spent: 20 ft on the ground, then up to 20 ft more in the water, then nothing on the ground.
func TestSwitchingSubtractsWhatWasSpent(t *testing.T) {
	t.Parallel()
	tr := waterCorridor()
	in := tr.Move(at(2), at(6), nil, swimmer(200))
	if in.CostDFt != 200 || in.OverDFt != 0 || in.LeftAfterDFt != 0 {
		t.Fatalf("20 ft after 20 ft: cost %d, over %d, left after %d; want 200, 0, 0", in.CostDFt, in.OverDFt, in.LeftAfterDFt)
	}
	// 40 ft spent: the walking 30 is gone, so no square of floor is within reach, and the water has nothing left either.
	for _, to := range []int{5, 7, 2, 1} {
		if mv := tr.Move(at(6), at(to), nil, swimmer(400)); mv.Blocked || mv.OverDFt == 0 {
			t.Errorf("to %d with everything spent: over %d", to, mv.OverDFt)
		}
	}
	// Spent 350 in the water: 50 left to swim, none on the floor.
	if mv := tr.Move(at(5), at(6), nil, swimmer(350)); mv.OverDFt != 0 || mv.LeftAfterDFt != 0 {
		t.Errorf("the last 5 ft of swimming: over %d, left after %d", mv.OverDFt, mv.LeftAfterDFt)
	}
	if mv := tr.Move(at(8), at(9), nil, swimmer(350)); mv.OverDFt != 100 {
		t.Errorf("out of the water after 35 ft: over by %d, want 100 (400 less the walking 300)", mv.OverDFt)
	}
}

// Reach, Move and MoveUntil agree on every square.
func TestReachMoveAndMoveUntilAgreeOnSpeeds(t *testing.T) {
	t.Parallel()
	tr := waterCorridor()
	for _, m := range []grid.Mover{swimmer(0), swimmer(100), plain(0), plain(150)} {
		reach := map[grid.Square]grid.Reachable{}
		for _, r := range tr.Reach(at(0), 400, nil, m) {
			reach[r.Square] = r
		}
		for col := 1; col < 12; col++ {
			mv := tr.Move(at(0), at(col), nil, m)
			r, ok := reach[at(col)]
			if ok != (!mv.Blocked && mv.OverDFt == 0) {
				t.Errorf("square %d: reachable %v, move over %d (used %d, water limit %d)", col, ok, mv.OverDFt, m.Speeds.UsedDFt, m.Speeds.WaterDFt)
			}
			if ok && (r.CostDFt != mv.CostDFt || r.LeftAfterDFt != mv.LeftAfterDFt) {
				t.Errorf("square %d: reach says %d and %d, move says %d and %d", col, r.CostDFt, r.LeftAfterDFt, mv.CostDFt, mv.LeftAfterDFt)
			}
			st := tr.MoveUntil(at(0), at(col), nil, m, nil, math.MaxInt)
			if ok != (st.Reached == at(col)) {
				t.Errorf("square %d: reachable %v, MoveUntil reached %v (%v)", col, ok, st.Reached, st.Reason)
			}
			if ok && st.CostDFt != mv.CostDFt {
				t.Errorf("square %d: MoveUntil cost %d, Move cost %d", col, st.CostDFt, mv.CostDFt)
			}
		}
	}
}

// A move that goes on into the water stops where the speed gives out, and says it ran out of movement.
func TestMoveUntilStopsWhereTheSpeedOfTheSurfaceGivesOut(t *testing.T) {
	t.Parallel()
	tr := waterCorridor()
	if st := tr.MoveUntil(at(0), at(11), nil, swimmer(0), nil, math.MaxInt); st.Reached != at(8) || st.Reason != grid.StopMovement || st.CostDFt != 400 {
		t.Errorf("swimmer: reached %v, %v, cost %d; want square 8, movement, 400", st.Reached, st.Reason, st.CostDFt)
	}
	if st := tr.MoveUntil(at(0), at(11), nil, plain(0), nil, math.MaxInt); st.Reached != at(4) || st.Reason != grid.StopMovement || st.CostDFt != 300 {
		t.Errorf("no swim speed: reached %v, %v, cost %d; want square 4, movement, 300", st.Reached, st.Reason, st.CostDFt)
	}
}

// A climb speed is the limit on a climbable surface as a swim speed is in the water.
func TestAClimbSpeedIsTheLimitOnAClimbableSurface(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 12, Rows: 1}
	sf := grid.NewSurfaceLayer(g)
	for col := 3; col <= 8; col++ {
		sf.Set(col, 0, grid.SurfaceClimb)
	}
	tr := grid.Terrain{Grid: g, Surface: sf}
	m := grid.Mover{Climbs: true, Speeds: &grid.Speeds{FloorDFt: 300, WaterDFt: 300, ClimbDFt: 400}}
	if mv := tr.Move(at(0), at(8), nil, m); mv.OverDFt != 0 || mv.CostDFt != 400 {
		t.Errorf("climbing 6 squares at climb 40: cost %d, over %d", mv.CostDFt, mv.OverDFt)
	}
	if mv := tr.Move(at(0), at(9), nil, m); mv.OverDFt != 150 {
		t.Errorf("down to the floor after 45 ft: over %d, want 150", mv.OverDFt)
	}
}

// A flier's limit is its fly speed on every surface, and it pays no extra for any.
func TestAFliersLimitIsItsFlySpeedEverywhere(t *testing.T) {
	t.Parallel()
	tr := waterCorridor()
	m := grid.Mover{Flier: true, Speeds: &grid.Speeds{FloorDFt: 600, WaterDFt: 600, ClimbDFt: 600}}
	if mv := tr.Move(at(0), at(11), nil, m); mv.OverDFt != 0 || mv.CostDFt != 550 || mv.LeftAfterDFt != 50 {
		t.Errorf("a flier over the corridor: cost %d, over %d, left after %d", mv.CostDFt, mv.OverDFt, mv.LeftAfterDFt)
	}
}

// Without Speeds nothing changes: the caller's own left is the only limit.
func TestWithoutSpeedsTheCallersLeftIsTheLimit(t *testing.T) {
	t.Parallel()
	tr := waterCorridor()
	mv := tr.Move(at(0), at(9), nil, grid.Mover{Swims: true})
	if mv.Blocked || mv.OverDFt != 0 || mv.LeftAfterDFt != 0 || mv.CostDFt != 450 {
		t.Errorf("no speeds: %+v", mv)
	}
}
