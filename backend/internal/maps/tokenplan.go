package maps

import (
	"math"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// tokenStep is what a move does to one token: where it ends, how many squares it was asked
// to go and really went, why it stopped early, and the closed doors it walked through.
type tokenStep struct {
	reached      grid.Square
	asked, moved int
	stop         grid.StopReason
	opens        []grid.Square
	// inTheWay is the square that stopped it: the next one past reached on the way to the destination.
	inTheWay grid.Square
	// freeAt is where the token ends on a map with no grid, in base points (nil on a grid).
	freeAt *[2]int32
}

// movePlan is the plan of a move outside combat.
type movePlan struct {
	steps []tokenStep
	// moved says some token changed square.
	moved bool
	// blocked is the index of the first token of a group whose arrival square did not fit,
	// -1 when nothing was refused.
	blocked int
}

// planMove plans the move of the tokens standing on starts, the first being the anchor, whose
// destination is anchorTo. A lone token walks the straight line and stops before the first wall,
// locked door or can't-pass square, with no limit of movement (it is not combat). Several tokens
// keep their offset from the anchor and all arrive or none does: the group moves only when every
// arrival square is on the map and standable (the line between is not walked, so a group may cross
// what a lone token could not; the arrival squares are what the master sees on the screen).
func planMove(t grid.Terrain, starts []grid.Square, anchorTo grid.Square) movePlan {
	if len(starts) == 1 {
		return planAlone(t, starts[0], anchorTo)
	}
	plan := movePlan{steps: make([]tokenStep, len(starts)), blocked: -1}
	dc, dr := anchorTo.Col-starts[0].Col, anchorTo.Row-starts[0].Row
	dests := make([]grid.Square, len(starts))
	for i, from := range starts {
		dests[i] = grid.Square{Col: from.Col + dc, Row: from.Row + dr}
		plan.steps[i] = tokenStep{reached: from, asked: grid.RangeSquares(from, dests[i])}
		if plan.blocked < 0 && from != dests[i] && !t.Standable(dests[i]) {
			plan.blocked = i
			plan.steps[i].stop, plan.steps[i].inTheWay = groupStop(t, dests[i]), dests[i]
		}
	}
	if plan.blocked >= 0 {
		return plan
	}
	for i, dest := range dests {
		plan.steps[i].reached, plan.steps[i].moved = dest, plan.steps[i].asked
		plan.moved = plan.moved || dest != starts[i]
	}
	return plan
}

// groupStop is why a group's arrival square does not fit.
func groupStop(t grid.Terrain, dest grid.Square) grid.StopReason {
	if t.Grid.Contains(dest) && t.NoPass.Has(dest) && !t.Walls.Has(dest) {
		return grid.StopNoPass
	}
	return grid.StopWall
}

// planAlone plans the move of one token.
func planAlone(t grid.Terrain, from, to grid.Square) movePlan {
	st := t.MoveUntil(from, to, nil, grid.Mover{}, nil, math.MaxInt)
	step := tokenStep{reached: st.Reached, asked: grid.RangeSquares(from, to), moved: grid.RangeSquares(from, st.Reached), stop: st.Reason, opens: st.Opens}
	if st.Reason != grid.StopNone {
		step.inTheWay = squareAfter(st.Reached, to)
	}
	return movePlan{steps: []tokenStep{step}, moved: st.Reached != from, blocked: -1}
}

// squareAfter is the square the line from one square to another enters right after the first.
func squareAfter(from, to grid.Square) grid.Square {
	if line := grid.Line(from, to); len(line) > 0 {
		return line[0].Square
	}
	return from
}
