package play

import (
	"testing"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// RN-10 and the surface (#589): a player plans on the water they know of. Toren walks in the dark, so the water in the corridor is floor to
// his plan (no extra cost, no reason in the options); the real move pays it, is cut short where the movement runs out, and only says so.
// The master's options show the swim.
func TestRN10_TheWaterAPlayerDoesNotSeeCutsTheMoveShortWithoutAReason(t *testing.T) {
	t.Parallel()
	f := newFogCave(t)
	f.torch(t, false)
	f.paint(t, mapsv1.MapLayer_MAP_LAYER_SURFACE, int32(grid.SurfaceWater), &mapsv1.MapSquare{Col: 8, Row: 7}, &mapsv1.MapSquare{Col: 9, Row: 7}, &mapsv1.MapSquare{Col: 10, Row: 7})
	f.fight(t)

	plan, err := f.options(t, f.caio, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions(Toren) error = %v", err)
	}
	if r := reachableAt(plan, 10, 7); r == nil || r.GetCostDft() != 200 || r.GetExtraDft() != 0 || r.GetSurfaceExtra() != playv1.SurfaceExtra_SURFACE_EXTRA_UNSPECIFIED {
		t.Errorf("Toren's plan for (10,7) = %v; want 200, no extra and no reason: the water is floor to him", r)
	}
	actual, err := f.options(t, f.master, "Toren")
	if err != nil {
		t.Fatalf("GetMoveOptions(master) error = %v", err)
	}
	if r := reachableAt(actual, 9, 7); r == nil || r.GetExtraDft() != 100 || r.GetCostDft() != 250 || r.GetSurfaceExtra() != playv1.SurfaceExtra_SURFACE_EXTRA_SWIM {
		t.Errorf("the master's options for (9,7) = %v; want 250 with 100 of swim", r)
	}

	res, err := f.moveResponse(t, f.caio, "Toren", 10, 7)
	if err != nil {
		t.Fatalf("MoveCombatant(Toren to 10, 7) error = %v", err)
	}
	got := byLabel(t, res.GetEncounter(), "Toren")
	if !res.GetStoppedEarly() || got.GetCol() != 9 || got.GetMovementUsedDft() != 250 {
		t.Errorf("Toren ends on %d with %d spent, stopped early %v; want 9, 250 and cut short before the square he could not pay", got.GetCol(), got.GetMovementUsedDft(), res.GetStoppedEarly())
	}

	// What he walked is remembered: the water at (9,7) is his now, and the one he never reached is not.
	layers, err := f.mapsAs(f.caio).GetMapLayers(t.Context(), connect.NewRequest(&mapsv1.GetMapLayersRequest{CampaignId: f.campaignID, MapId: f.mapID}))
	if err != nil {
		t.Fatalf("GetMapLayers() error = %v", err)
	}
	known, err := grid.DecodeSurfaceLayer(caveGridSize, layers.Msg.GetSurface())
	if err != nil || known.Get(9, 7) != grid.SurfaceWater {
		t.Errorf("Toren does not remember the water he swam in: %v, %v", known, err)
	}
}
