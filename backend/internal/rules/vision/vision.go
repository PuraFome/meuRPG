// Package vision is the fog of war's arithmetic (MR-036, Etapa 9, D6): which
// squares of a map a viewer sees, from the light and the walls, with the
// viewer's senses. It is pure, like package rules (ADR-0008): the scene and the
// viewers come in, the states of the squares come out. The maps module stores
// the layers and the points, and the server turns what this package computes
// into what each player may receive.
//
// The rules, in short:
//
//   - Light and sight stop at walls (and at closed, locked and secret doors, which
//     Scene.Doors holds): the same straight line between two squares'
//     centers as movement (package grid), where a wall on the line, or a squeeze
//     between two walls, blocks it.
//   - A square's light is the brightest that reaches it: the base light of the
//     map or the light painted on the square, or a light source whose bright or
//     dim radius reaches it along a line no wall crosses. Distances are the
//     straight distance between centers.
//   - A viewer sees a square when the line to it is clear and it is lit (bright
//     or dim), or it is within one of the viewer's senses. Darkvision, within
//     its range, sees dim light as bright and darkness as dim light, in grey;
//     blindsight sees regardless of the light, also in grey; truesight sees
//     darkness and dim light as bright. A viewer always knows
//     its own square. A wall is seen when a seen square touches it (the eight
//     neighbors).
//   - Seen in dim light or in grey is lightly obscured, so a passive check
//     there is 5 lower (PassivePenalty).
package vision

import (
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// Source is a light on the map: a torch, a lantern, the light spell. BrightFt
// is the radius of its bright light and DimFt how much further its dim light
// goes, the way the SRD says it ("bright light in a 20-foot radius and dim
// light for an additional 20 feet": 20 and 20). Both in feet; a radius is the
// distance between centers.
type Source struct {
	At       grid.Square
	BrightFt int
	DimFt    int
}

// Senses are a viewer's special senses, as a range in feet (0 for none).
type Senses struct {
	DarkvisionFt int
	BlindsightFt int
	TruesightFt  int
}

// Viewer is someone who sees: a square and its senses.
type Viewer struct {
	At     grid.Square
	Senses Senses
}

// Scene is the map as light and sight need it.
type Scene struct {
	Grid grid.Grid
	// Walls block light and sight. May be nil.
	Walls *grid.Layer
	// Doors block light and sight where they are closed, locked or secret (RN-26);
	// open and barred doors let both through. May be nil.
	Doors *grid.DoorLayer
	// Base is the map's base light, for a square nothing is painted on. The
	// zero value (Unpainted) counts as Dark.
	Base grid.Light
	// Painted is the light the master painted. May be nil.
	Painted *grid.LightLayer
	// Sources are the lights on the map, and the ones the characters carry.
	Sources []Source
}

// State is how a viewer sees a square.
type State uint8

// The states, in order of how much the viewer sees (Union keeps the greatest).
const (
	// Unseen: the viewer does not see the square.
	Unseen State = iota
	// SeenWall: a wall the viewer sees, because a seen square touches it.
	SeenWall
	// SeenGrey: seen in the dark, by darkvision or blindsight, without color.
	SeenGrey
	// SeenDim: seen in dim light (penumbra).
	SeenDim
	// SeenBright: seen in bright light.
	SeenBright
)

// PassivePenalty is what the state takes from a passive check there: dim light
// and the grey of darkvision are lightly obscured (SRD), which is -5 on a
// passive Wisdom (Perception) score; bright light takes nothing.
func PassivePenalty(s State) int {
	if s == SeenDim || s == SeenGrey {
		return -5
	}
	return 0
}

// Lit is a scene with its light worked out, ready to say what viewers see.
// Compile it once for a scene and ask it about every viewer: the light, which
// does not depend on the viewer, is the part worth sharing.
type Lit struct {
	g     grid.Grid
	sight *grid.Sight
	// level[n] is the light of square n: grid.Dark, grid.Dim or grid.Bright.
	level []grid.Light
}

// MaxLightFt is the largest radius, in feet, of the bright light and of the
// dim light of a source: Daylight's 60 ft doubled. A larger one is clamped, so
// no input makes a single light cost more than a fixed share of the map.
const MaxLightFt = 120

// Compile works out the light of every square of the scene. It returns
// grid.ErrBadGrid for an invalid grid or for walls or painted light sized for
// another grid, and never panics on a scene it is given.
func Compile(s Scene) (*Lit, error) {
	if s.Painted != nil && s.Painted.Grid() != s.Grid {
		return nil, grid.ErrBadGrid
	}
	if s.Doors != nil && s.Doors.Grid() != s.Grid {
		return nil, grid.ErrBadGrid
	}
	sight, err := grid.NewSight(s.Grid, grid.SightWalls(s.Walls, s.Doors))
	if err != nil {
		return nil, err
	}
	l := &Lit{g: s.Grid, sight: sight, level: make([]grid.Light, s.Grid.Squares())}
	base := s.Base
	if base == grid.Unpainted {
		base = grid.Dark
	}
	for row := range s.Grid.Rows {
		for col := range s.Grid.Columns {
			level := s.Painted.Get(col, row)
			if level == grid.Unpainted {
				level = base
			}
			l.level[row*s.Grid.Columns+col] = level
		}
	}
	for _, src := range s.Sources {
		l.shine(src)
	}
	return l, nil
}

// shine adds a light source: every square within its radius that its line
// reaches gets at least the level of its distance. A wall square gets no light
// (a wall is decided by what touches it).
func (l *Lit) shine(src Source) {
	src.BrightFt, src.DimFt = min(src.BrightFt, MaxLightFt), min(src.DimFt, MaxLightFt)
	if !l.g.Contains(src.At) || src.BrightFt < 0 || src.DimFt < 0 || src.BrightFt+src.DimFt == 0 {
		return
	}
	outer := src.BrightFt + src.DimFt
	radius := (outer + grid.FeetPerSquare - 1) / grid.FeetPerSquare
	for row := max(src.At.Row-radius, 0); row <= min(src.At.Row+radius, l.g.Rows-1); row++ {
		for col := max(src.At.Col-radius, 0); col <= min(src.At.Col+radius, l.g.Columns-1); col++ {
			sq := grid.Square{Col: col, Row: row}
			level := grid.Dim
			switch d2 := dist2Ft(src.At, sq); {
			case d2 > outer*outer:
				continue
			case src.BrightFt > 0 && d2 <= src.BrightFt*src.BrightFt:
				level = grid.Bright
			}
			n := row*l.g.Columns + col
			if level <= l.level[n] || l.sight.Wall(sq) || !l.sight.Clear(src.At, sq) {
				continue
			}
			l.level[n] = level
		}
	}
}

// dist2Ft is the squared distance between two squares' centers in square feet,
// so that comparing it with a radius in feet is a comparison of integers.
func dist2Ft(a, b grid.Square) int {
	dc, dr := (a.Col-b.Col)*grid.FeetPerSquare, (a.Row-b.Row)*grid.FeetPerSquare
	return dc*dc + dr*dr
}

// LightAt is the light of a square after the base light, the painted light and
// the sources: grid.Dark, grid.Dim or grid.Bright. A square outside the grid is
// dark.
func (l *Lit) LightAt(sq grid.Square) grid.Light {
	if !l.g.Contains(sq) {
		return grid.Dark
	}
	return l.level[sq.Row*l.g.Columns+sq.Col]
}

// View is what one viewer, or several together, see of a scene.
type View struct {
	g      grid.Grid
	states []State
}

// At is the state of a square: Unseen for one outside the grid.
func (v *View) At(sq grid.Square) State {
	if v == nil || !v.g.Contains(sq) {
		return Unseen
	}
	return v.states[sq.Row*v.g.Columns+sq.Col]
}

// Seen says whether the viewer sees the square at all (a wall it sees counts).
func (v *View) Seen(sq grid.Square) bool { return v.At(sq) != Unseen }

// Count is how many squares are seen.
func (v *View) Count() int {
	n := 0
	if v != nil {
		for _, s := range v.states {
			if s != Unseen {
				n++
			}
		}
	}
	return n
}

// See is what a viewer sees. A viewer outside the grid sees nothing.
func (l *Lit) See(v Viewer) *View {
	out := &View{g: l.g, states: make([]State, len(l.level))}
	if !l.g.Contains(v.At) {
		return out
	}
	row0, col0 := v.At.Row, v.At.Col
	for row := range l.g.Rows {
		for col := range l.g.Columns {
			n := row*l.g.Columns + col
			if row == row0 && col == col0 {
				out.states[n] = ownState(l.level[n])
				continue
			}
			out.states[n] = l.squareState(v, grid.Square{Col: col, Row: row})
		}
	}
	l.seeWalls(out)
	return out
}

// squareState is how the viewer sees a square other than its own: the light that is
// there or its senses, with the line to it clear. A wall is decided from what touches
// it (seeWalls), so it is Unseen here.
func (l *Lit) squareState(v Viewer, sq grid.Square) State {
	if l.sight.Wall(sq) {
		return Unseen
	}
	s := v.Senses
	n := sq.Row*l.g.Columns + sq.Col
	state := Unseen
	d2 := dist2Ft(v.At, sq)
	darkvision := s.DarkvisionFt > 0 && d2 <= s.DarkvisionFt*s.DarkvisionFt
	switch light := l.level[n]; {
	case s.TruesightFt > 0 && d2 <= s.TruesightFt*s.TruesightFt:
		state = SeenBright
	case light == grid.Bright:
		state = SeenBright
	case light == grid.Dim && darkvision:
		// SRD: within its range darkvision sees dim light as if it were bright.
		state = SeenBright
	case light == grid.Dim:
		state = SeenDim
	case darkvision, s.BlindsightFt > 0 && d2 <= s.BlindsightFt*s.BlindsightFt:
		state = SeenGrey
	}
	if state != Unseen && l.sight.Clear(v.At, sq) {
		return state
	}
	return Unseen
}

// CanSee says whether the viewer sees a creature standing on the square: the same
// answer as See(v).At(sq) >= SeenGrey, worked out for that one square (a light
// lookup and one line), so asking it of a few pairs never costs a view of the whole
// map. A wall is no place for a creature, and a square outside the grid is not seen.
func (l *Lit) CanSee(v Viewer, sq grid.Square) bool {
	if !l.g.Contains(v.At) || !l.g.Contains(sq) {
		return false
	}
	if sq == v.At {
		return true
	}
	return l.squareState(v, sq) != Unseen
}

// ownState is how a viewer sees the square it stands on: it always knows it,
// in the light that is there, and in grey when there is none.
func ownState(light grid.Light) State {
	switch light {
	case grid.Bright:
		return SeenBright
	case grid.Dim:
		return SeenDim
	}
	return SeenGrey
}

// seeWalls marks the walls that touch a seen square.
func (l *Lit) seeWalls(v *View) {
	for row := range l.g.Rows {
		for col := range l.g.Columns {
			n := row*l.g.Columns + col
			if !l.sight.Wall(grid.Square{Col: col, Row: row}) {
				continue
			}
		neighbors:
			for r := max(row-1, 0); r <= min(row+1, l.g.Rows-1); r++ {
				for c := max(col-1, 0); c <= min(col+1, l.g.Columns-1); c++ {
					if s := v.states[r*l.g.Columns+c]; s != Unseen && s != SeenWall {
						v.states[n] = SeenWall
						break neighbors
					}
				}
			}
		}
	}
}

// Union is what the viewers see together, a square at the best of their
// states: the "Visão do grupo", and a player with several creatures. Views
// come from the same Lit. With none, nothing is seen.
func (l *Lit) Union(views ...*View) *View {
	out := &View{g: l.g, states: make([]State, len(l.level))}
	for _, v := range views {
		if v == nil {
			continue
		}
		for n, s := range v.states {
			out.states[n] = max(out.states[n], s)
		}
	}
	return out
}
