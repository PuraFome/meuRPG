package maps

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// The plan of a move outside combat: where each token ends, how far it got and why it stopped.

func planTerrain() grid.Terrain {
	g := grid.Grid{Columns: 16, Rows: 6}
	t := grid.Terrain{Grid: g, Walls: grid.NewLayer(g), NoPass: grid.NewLayer(g), Doors: grid.NewDoorLayer(g)}
	t.NoPass.Set(8, 2, true)
	t.Walls.Set(12, 4, true)
	return t
}

func sqr(c, r int) grid.Square { return grid.Square{Col: c, Row: r} }

func TestALoneTokenGoesAllTheWayWhenNothingIsInTheWay(t *testing.T) {
	t.Parallel()
	p := planMove(planTerrain(), []grid.Square{sqr(1, 1)}, sqr(5, 1))
	if !p.moved || p.blocked != -1 || len(p.steps) != 1 {
		t.Fatalf("plan = %+v, want one token that moved", p)
	}
	if s := p.steps[0]; s.reached != sqr(5, 1) || s.asked != 4 || s.moved != 4 || s.stop != grid.StopNone {
		t.Errorf("step = %+v, want 4 of 4 squares, no stop", s)
	}
}

func TestALoneTokenStopsBeforeACantPassSquareAndSaysSo(t *testing.T) {
	t.Parallel()
	p := planMove(planTerrain(), []grid.Square{sqr(3, 2)}, sqr(13, 2))
	s := p.steps[0]
	if s.reached != sqr(7, 2) || s.asked != 10 || s.moved != 4 || s.stop != grid.StopNoPass {
		t.Errorf("step = %+v, want it stopped at (7,2) after 4 of 10 squares by the can't-pass square", s)
	}
	if !p.moved {
		t.Error("a move that got four squares is not moved")
	}
}

func TestALoneTokenThatCannotTakeAStepDoesNotMove(t *testing.T) {
	t.Parallel()
	p := planMove(planTerrain(), []grid.Square{sqr(7, 2)}, sqr(10, 2))
	if p.moved || p.steps[0].moved != 0 || p.steps[0].stop != grid.StopNoPass || p.steps[0].reached != sqr(7, 2) {
		t.Errorf("plan = %+v, want nobody moved, stopped by the can't-pass square", p)
	}
}

func TestAGroupKeepsItsOffsetAndMovesTogether(t *testing.T) {
	t.Parallel()
	starts := []grid.Square{sqr(1, 1), sqr(2, 1), sqr(1, 2)}
	p := planMove(planTerrain(), starts, sqr(4, 1))
	if !p.moved || p.blocked != -1 {
		t.Fatalf("plan = %+v, want the group moved", p)
	}
	want := []grid.Square{sqr(4, 1), sqr(5, 1), sqr(4, 2)}
	for i, w := range want {
		if p.steps[i].reached != w || p.steps[i].moved != grid.RangeSquares(starts[i], w) {
			t.Errorf("token %d = %+v, want it at %v", i, p.steps[i], w)
		}
	}
}

func TestAGroupWhoseArrivalSquareDoesNotFitDoesNotMoveAndNamesWho(t *testing.T) {
	t.Parallel()
	// The anchor's arrival (7,2) is free, the second one's (8,2) is a can't-pass square.
	starts := []grid.Square{sqr(3, 2), sqr(4, 2), sqr(3, 3)}
	p := planMove(planTerrain(), starts, sqr(7, 2))
	if p.moved || p.blocked != 1 {
		t.Fatalf("plan = %+v, want nobody moved and the second one named", p)
	}
	for i, s := range p.steps {
		if s.reached != starts[i] || s.moved != 0 {
			t.Errorf("token %d moved in a group that was refused: %+v", i, s)
		}
	}
	if p.steps[1].stop != grid.StopNoPass {
		t.Errorf("the named token's stop = %q, want no_pass", p.steps[1].stop)
	}
}

func TestAGroupGoingOffTheMapIsRefused(t *testing.T) {
	t.Parallel()
	p := planMove(planTerrain(), []grid.Square{sqr(14, 1), sqr(15, 1)}, sqr(15, 1))
	if p.moved || p.blocked != 1 || p.steps[1].stop != grid.StopWall {
		t.Errorf("plan = %+v, want the second one refused by the edge of the map", p)
	}
}

func TestAMoveThatOpensClosedDoorsListsThem(t *testing.T) {
	t.Parallel()
	tr := planTerrain()
	tr.Doors.Set(4, 1, grid.DoorClosed)
	p := planMove(tr, []grid.Square{sqr(1, 1)}, sqr(6, 1))
	if got := p.steps[0].opens; len(got) != 1 || got[0] != sqr(4, 1) {
		t.Errorf("opens = %v, want the door at (4,1)", got)
	}
}
