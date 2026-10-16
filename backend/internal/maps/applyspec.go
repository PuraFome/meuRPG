package maps

import (
	"context"
	"fmt"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
)

// applySpecChange's kinds (MR-035, MR-036, MR-041): each one works out its own
// data for the point after the change, or clears it, and the three run in this
// order, so the first refusal is the trap's, then the treasure's, then the light's.

// applyTrapChange sets the trap of a point that is one after the change, refuses a
// trap spec for any other kind, and clears what a point that was a trap leaves.
func (s *Service) applyTrapChange(ctx context.Context, q *mapsdb.Queries, p mapsdb.MapPoint, c pointChange, params *mapsdb.UpdateMapPointParams) error {
	trap := kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_TRAP]
	switch {
	case params.Kind == trap:
		return s.setTrap(p, c, params)
	case c.trap != nil:
		return errNotThatKind("trap", "TRAP")
	}
	params.Trap, params.TrapState, params.TrapTriggeredAt = nil, nil, nil
	if p.Kind == trap {
		if err := q.DeletePointReveals(ctx, p.ID); err != nil {
			return fmt.Errorf("forget the trap's reveals: %w", err)
		}
	}
	return nil
}

// setTrap replaces the trap with the request's, or keeps the one the point has; a
// point that becomes a trap needs a spec.
func (s *Service) setTrap(p mapsdb.MapPoint, c pointChange, params *mapsdb.UpdateMapPointParams) error {
	trap := kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_TRAP]
	if c.trap == nil {
		if p.Kind != trap {
			return badSpec("trap is required to make a point a TRAP")
		}
		return nil
	}
	var current *string
	if p.Kind == trap {
		current = p.TrapState
	}
	t, err := s.cleanTrap(c.trap, current)
	if err != nil {
		return err
	}
	params.Trap, params.TrapState = t.json, &t.state
	if t.state == stateTriggered && params.TrapTriggeredAt == nil {
		params.TrapTriggeredAt = &params.Now // the first time it fires, for good
	}
	return nil
}

// applyTreasureChange sets the value of a point that is a treasure after the
// change, refuses a value for any other kind, and clears what a point that was a
// treasure leaves.
func applyTreasureChange(ctx context.Context, q *mapsdb.Queries, p mapsdb.MapPoint, c pointChange, params *mapsdb.UpdateMapPointParams) error {
	switch {
	case params.Kind == kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_TREASURE]:
		return setTreasure(p, c, params)
	case c.treasure != nil:
		return errNotThatKind("treasure_value_po", "TREASURE")
	}
	return dropTreasure(ctx, q, p, params)
}

// setTreasure sets the treasure's value; a point that becomes a treasure starts
// with 0 when the request has none. Once the treasure was turned into XP its value
// cannot change.
func setTreasure(p mapsdb.MapPoint, c pointChange, params *mapsdb.UpdateMapPointParams) error {
	switch {
	case c.treasure != nil:
		v, err := cleanTreasureValue(*c.treasure)
		if err != nil {
			return err
		}
		if p.TreasureConvertedAwardID != nil && (p.TreasureValuePo == nil || *p.TreasureValuePo != v) {
			return errTreasureConverted() // its worth became XP
		}
		params.TreasureValuePo = &v
	case p.Kind != kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_TREASURE]:
		zero := int32(0)
		params.TreasureValuePo = &zero
	}
	return nil
}

// dropTreasure clears the treasure's data of a point that is not one after the
// change. A converted or found treasure is part of the session's record, so it
// cannot stop being one.
func dropTreasure(ctx context.Context, q *mapsdb.Queries, p mapsdb.MapPoint, params *mapsdb.UpdateMapPointParams) error {
	if p.Kind == kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_TREASURE] {
		if p.TreasureConvertedAwardID != nil {
			return errTreasureConverted()
		}
		// Like a delete: a found treasure is part of the session's record, unmarked
		// before it goes, never lost silently with its finders.
		if p.TreasureFoundAt != nil {
			return errTreasureFound()
		}
		if err := q.DeleteTreasureFinders(ctx, p.ID); err != nil {
			return fmt.Errorf("forget the treasure's finders: %w", err)
		}
	}
	params.TreasureValuePo, params.TreasureFoundAt, params.TreasureSessionID = nil, nil, nil
	return nil
}

// applyLightChange sets the light of a point that is one after the change,
// refuses a light spec for any other kind, and clears the light of the rest.
func (s *Service) applyLightChange(p mapsdb.MapPoint, c pointChange, params *mapsdb.UpdateMapPointParams) error {
	switch {
	case params.Kind == kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_LIGHT]:
		return s.setLight(p, c, params)
	case c.light != nil:
		return errNotThatKind("light", "LIGHT")
	}
	params.LightPreset, params.LightBrightFt, params.LightDimFt = nil, nil, nil
	return nil
}

// setLight replaces the light with the request's, or keeps the one the point has;
// a point that becomes a light needs a spec.
func (s *Service) setLight(p mapsdb.MapPoint, c pointChange, params *mapsdb.UpdateMapPointParams) error {
	if c.light != nil {
		l, err := s.cleanLight(c.light)
		if err != nil {
			return err
		}
		params.LightPreset, params.LightBrightFt, params.LightDimFt = l.preset, &l.bright, &l.dimmed
	} else if p.Kind != kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_LIGHT] {
		return badSpec("light is required to make a point a LIGHT")
	}
	// No player ever receives a light, so it has nothing to reveal.
	if c.revealed != nil && *c.revealed {
		return badSpec("a LIGHT point is never shown to the players, so it cannot be revealed")
	}
	params.RevealedAt = nil
	return nil
}
