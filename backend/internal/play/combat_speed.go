package play

import (
	"context"
	"slices"

	playdb "github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// Using different speeds (#819, SRD 5.1 "Using Different Speeds"): each speed is the limit on its surface. A step is allowed while
// what was spent this turn plus the cost up to it is within the speed of the surface it enters; switching from one speed to
// another takes what was spent from the new one, and with nothing left the move ends.
//
// The floor's speed is the one speedDFt gives (walking, or flying for a flier; with neither, the best of swimming and
// climbing). The water's is the swim speed and the climbable surface's the climb speed, when the combatant has them; without
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

// withSpeeds is the mover with its speeds set, for a move counted over `factor` (a drag costs a factor of its length, so the
// speeds and what was spent are taken over the factor).
func withSpeeds(m grid.Mover, c playdb.Combatant, factor int) grid.Mover {
	sp := speedsOf(c)
	factor = max(factor, 1)
	sp.UsedDFt /= factor
	sp.FloorDFt /= factor
	sp.WaterDFt /= factor
	sp.ClimbDFt /= factor
	m.Speeds = &sp
	return m
}

// standingSurfaces says what each placed combatant stands on, as the viewer may know it: the counter of a swimmer reads its swim speed
// only while it is in water the viewer sees (RN-10). The master sees every square; a player with the fog sees what Sees says (the square
// of their own character always, in the light of the dark or not); without the fog a player sees the map. A square the viewer does not
// see is floor to them. It reads the surface layer once (one indexed read of one column) and only when the view carries a counter and the
// map has a surface layer to look at; nothing else of the map is read. It is nil when nobody stands on anything but the floor.
func (s *Service) standingSurfaces(ctx context.Context, d *encounterData, v combatViewer) (map[string]grid.Surface, error) {
	src, ok := s.terrain.(SurfaceSource)
	if !ok || d.enc.MapID == nil || d.enc.Status == statusEnded {
		return nil, nil
	}
	if !v.master && !slices.ContainsFunc(d.cs, func(c playdb.Combatant) bool { return v.owns(c) && placed(c) }) {
		return nil, nil // no counter of a placed combatant in this view (the joint-turn partner is a player's own, so it is in the check)
	}
	layer, err := src.Surface(ctx, nil, *d.enc.MapID, grid.Grid{Columns: int(d.enc.GridColumns), Rows: int(d.enc.GridRows)})
	if err != nil || layer == nil {
		return nil, err
	}
	out := make(map[string]grid.Surface)
	for _, c := range d.cs {
		if !placed(c) {
			continue
		}
		sq := squareOfCombatant(c)
		if sf := layer.Get(sq.Col, sq.Row); sf != grid.SurfaceFloor && (v.master || v.sight == nil || v.sight.sight.Sees(v.userID, sq)) {
			out[c.ID] = sf
		}
	}
	return out, nil
}
