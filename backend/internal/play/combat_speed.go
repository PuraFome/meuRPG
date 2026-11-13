package play

import (
	"context"
	"slices"

	"github.com/jackc/pgx/v5"

	playdb "github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// Using different speeds (#819, SRD 5.1 "Using Different Speeds"): each speed is the limit on its surface. A step is allowed while
// what was spent this turn plus the cost up to it is within the speed of the surface it enters; switching from one speed to
// another takes what was spent from the new one, and with nothing left the move ends.
//
// The floor's speed is the walking speed, or the flying one for a flier, and with neither the best of swimming and
// climbing. The water's is the swim speed and the climbable surface's the climb speed, when the combatant has them; without
// one the surface is held to the floor's speed and keeps its extra cost. A flier flies on every surface (fly is a mode of the
// whole turn, see docs/architecture/play/movement.md). Dash, the effects and the conditions that take the speed away apply to each
// speed alike.

// speedAtDFt is a speed of the combatant in feet, as the turn's speed in tenths of a foot: what the Dash, the effects and the
// conditions leave of it.
func speedAtDFt(c playdb.Combatant, ft int32) int {
	if slices.ContainsFunc(c.Conditions, func(k string) bool { return slices.Contains(noSpeed, k) }) || c.EffectNoMove {
		return 0 // grappled, restrained and the like; the lethargy of a Velocidade that ended (RN-22)
	}
	speed := (int(ft) + int(c.EffectSpeedAddFt)) * 10
	if c.Dashed {
		speed *= 2
	}
	if c.EffectSpeedPct > 0 {
		speed = speed * int(c.EffectSpeedPct) / 100
	}
	return speed
}

// speedsOf is the combatant's speed on each surface and what it has spent this turn.
func speedsOf(c playdb.Combatant) grid.Speeds {
	floor := speedAtDFt(c, speedFtOf(c))
	sp := grid.Speeds{UsedDFt: int(c.MovementUsedDft), FloorDFt: floor, WaterDFt: floor, ClimbDFt: floor}
	if !flies(c) {
		if c.SpeedSwimFt > 0 {
			sp.WaterDFt = speedAtDFt(c, c.SpeedSwimFt)
		}
		if c.SpeedClimbFt > 0 {
			sp.ClimbDFt = speedAtDFt(c, c.SpeedClimbFt)
		}
	}
	return sp
}

// movementLeftAtDFt is the movement the combatant has left on a surface, in tenths of a foot.
func movementLeftAtDFt(c playdb.Combatant, sf grid.Surface) int { return speedsOf(c).LeftAt(sf) }

// speedOnDFt is the combatant's speed on a surface, in tenths of a foot.
func speedOnDFt(c playdb.Combatant, sf grid.Surface) int { return speedsOf(c).Limit(sf) }

// mostLeftDFt is the most the combatant has left on any surface: the circle the options look at, and where a move can
// possibly end.
func mostLeftDFt(sp grid.Speeds) int {
	return max(sp.LeftAt(grid.SurfaceFloor), sp.LeftAt(grid.SurfaceWater), sp.LeftAt(grid.SurfaceClimb))
}

// withSpeeds is the mover with its speeds set, for a move whose cost is `factor` times the length of its line (a drag at half speed
// is 2). The line's cost is checked against what is left on each surface divided by the factor, rounded down, so that the cost it
// charges (the line times the factor) never goes past what is left, whatever the amount spent.
func withSpeeds(m grid.Mover, c playdb.Combatant, factor int) grid.Mover {
	sp := speedsOf(c)
	factor = max(factor, 1)
	used := sp.UsedDFt / factor
	left := func(limit int) int { return used + max(limit-sp.UsedDFt, 0)/factor }
	sp.FloorDFt, sp.WaterDFt, sp.ClimbDFt = left(sp.FloorDFt), left(sp.WaterDFt), left(sp.ClimbDFt)
	sp.UsedDFt = used
	m.Speeds = &sp
	return m
}

// surfaceLayerOf reads the surface layer of the combat's map: one indexed read of one column. It is nil for a combat without a map
// (the theatre of the mind) or one that ended, where everybody stands on the floor.
func (s *Service) surfaceLayerOf(ctx context.Context, tx pgx.Tx, enc playdb.Encounter) (*grid.SurfaceLayer, error) {
	if s.terrain == nil || enc.MapID == nil || enc.Status == statusEnded {
		return nil, nil
	}
	return s.terrain.Surface(ctx, tx, *enc.MapID, grid.Grid{Columns: int(enc.GridColumns), Rows: int(enc.GridRows)})
}

// surfaceUnder is the surface the viewer may know a combatant stands on (RN-10): the real one for the master, for a player when the fog
// lets them see the square (their own character's always) and for a map without the fog; a square the viewer does not see is floor to them.
func surfaceUnder(layer *grid.SurfaceLayer, v combatViewer, c playdb.Combatant) grid.Surface {
	if layer == nil || !placed(c) {
		return grid.SurfaceFloor
	}
	sq := squareOfCombatant(c)
	if v.master || v.sight == nil || v.sight.sight.Sees(v.userID, sq) {
		return layer.Get(sq.Col, sq.Row)
	}
	return grid.SurfaceFloor
}

// standingSurface is the surface one combatant stands on, as the viewer may know it.
func (s *Service) standingSurface(ctx context.Context, tx pgx.Tx, enc playdb.Encounter, v combatViewer, c playdb.Combatant) (grid.Surface, error) {
	layer, err := s.surfaceLayerOf(ctx, tx, enc)
	if err != nil {
		return grid.SurfaceFloor, err
	}
	return surfaceUnder(layer, v, c), nil
}

// standingSurfaces says what each combatant a view carries a counter of stands on, as the viewer may know it: the counter of a swimmer
// reads its swim speed only while it is in water the viewer sees (RN-10). It reads the surface layer once and only when the view carries
// a counter (the master's, or a player with a placed character); nothing else of the map is read. A combatant it does not name stands on
// the floor.
func (s *Service) standingSurfaces(ctx context.Context, d *encounterData, v combatViewer) (map[string]grid.Surface, error) {
	if !v.master && !slices.ContainsFunc(d.cs, func(c playdb.Combatant) bool { return v.owns(c) && placed(c) }) {
		return nil, nil
	}
	layer, err := s.surfaceLayerOf(ctx, nil, d.enc)
	if err != nil || layer == nil {
		return nil, err
	}
	out := make(map[string]grid.Surface)
	for _, c := range d.cs {
		if sf := surfaceUnder(layer, v, c); sf != grid.SurfaceFloor {
			out[c.ID] = sf
		}
	}
	return out, nil
}
