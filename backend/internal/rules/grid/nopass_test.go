package grid_test

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// The "can't pass" layer: squares nobody walks into, that stop movement but not sight.

func noPassTerrain() grid.Terrain {
	g := grid.Grid{Columns: 12, Rows: 5}
	t := grid.Terrain{Grid: g, NoPass: grid.NewLayer(g)}
	t.NoPass.Set(6, 2, true)
	return t
}

func TestAMoveStopsBeforeACantPassSquare(t *testing.T) {
	t.Parallel()
	tr := noPassTerrain()
	st := tr.MoveUntil(sq(1, 2), sq(10, 2), nil, grid.Mover{}, nil, 1<<30)
	if st.Reached != sq(5, 2) || st.Reason != grid.StopNoPass {
		t.Errorf("MoveUntil = reached %v by %q, want (5,2) by %q", st.Reached, st.Reason, grid.StopNoPass)
	}
	if mv := tr.Move(sq(1, 2), sq(10, 2), nil, grid.Mover{}); !mv.Blocked || mv.By != grid.StopNoPass {
		t.Errorf("Move = %+v, want blocked by the can't-pass square", mv)
	}
	if mv := tr.Move(sq(1, 1), sq(10, 1), nil, grid.Mover{}); mv.Blocked {
		t.Errorf("a row without the square is blocked: %+v", mv)
	}
}

func TestACantPassSquareIsNotStandableAndDoesNotBlockSight(t *testing.T) {
	t.Parallel()
	tr := noPassTerrain()
	if tr.Standable(sq(6, 2)) {
		t.Error("a can't-pass square is standable")
	}
	if !tr.Standable(sq(5, 2)) {
		t.Error("the square next to it is not standable")
	}
	if tr.SightWalls().Has(sq(6, 2)) {
		t.Error("a can't-pass square blocks sight")
	}
}

func TestACantPassLayerOfAnotherGridIsRefused(t *testing.T) {
	t.Parallel()
	tr := noPassTerrain()
	tr.NoPass = grid.NewLayer(grid.Grid{Columns: 4, Rows: 4})
	if err := tr.Validate(); err == nil {
		t.Error("Validate accepted a layer of another grid")
	}
}

func TestAFormationFitsTheSquaresOfItsShape(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 12, Rows: 8}
	tr := grid.Terrain{Grid: g}
	cases := []struct {
		f    grid.Formation
		want []grid.Square
	}{
		{grid.FormationLine, []grid.Square{sq(3, 3), sq(4, 3), sq(5, 3), sq(6, 3)}},
		{grid.FormationBlock, []grid.Square{sq(3, 3), sq(4, 3), sq(3, 4), sq(4, 4)}},
	}
	for _, c := range cases {
		got := grid.Arrange(tr, sq(3, 3), 4, c.f, nil)
		if !equalSquares(got, c.want) {
			t.Errorf("Arrange(%v) = %v, want %v", c.f, got, c.want)
		}
	}
	arc := grid.Arrange(tr, sq(3, 3), 4, grid.FormationArc, nil)
	if len(arc) != 4 || distinct(arc) != 4 {
		t.Errorf("an arc of four = %v, want four different squares", arc)
	}
}

func TestAFormationMovesWhoDoesNotFitToTheNearestFreeSquare(t *testing.T) {
	t.Parallel()
	tr := noPassTerrain()
	taken := grid.Squares{sq(1, 2)}
	got := grid.Arrange(tr, sq(5, 2), 3, grid.FormationLine, taken)
	if len(got) != 3 || distinct(got) != 3 {
		t.Fatalf("Arrange = %v, want three different squares", got)
	}
	for _, s := range got {
		if !tr.Standable(s) || taken.Has(s) {
			t.Errorf("square %v is not standable or is taken", s)
		}
	}
	if got[0] != sq(5, 2) {
		t.Errorf("the first one = %v, want the anchor", got[0])
	}
}

func TestAFormationWithNoRoomGivesFewerSquares(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 2, Rows: 1}
	got := grid.Arrange(grid.Terrain{Grid: g}, sq(0, 0), 5, grid.FormationBlock, nil)
	if len(got) != 2 {
		t.Errorf("Arrange = %v, want the two squares there are", got)
	}
}

func distinct(s []grid.Square) int {
	seen := map[grid.Square]bool{}
	for _, x := range s {
		seen[x] = true
	}
	return len(seen)
}
