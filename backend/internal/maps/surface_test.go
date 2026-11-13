package maps

import (
	"testing"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// #589: the surface layer holds water and climbable squares, two bits a square. The master paints it, a player of a map
// without fog reads it as painted, a combat's Terrain carries it, and 3 is no value.
func TestSurfaceLayerPaintReadAndTerrain(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	m.mustSetGrid(s.campaign, s.mapID, 20)
	surface := mapsv1.MapLayer_MAP_LAYER_SURFACE

	if res := m.mustLayers(s.campaign, s.mapID); len(res.GetSurface()) != 0 {
		t.Fatalf("a map with nothing painted sends %d bytes of surface, want none", len(res.GetSurface()))
	}
	m.mustPaint(s.campaign, s.mapID, surface, int32(grid.SurfaceWater), [2]int32{2, 2}, [2]int32{3, 2})
	m.mustPaint(s.campaign, s.mapID, surface, int32(grid.SurfaceClimb), [2]int32{4, 2})
	for _, who := range []*user{m, s.ana} {
		got, err := grid.DecodeSurfaceLayer(testGrid, who.mustLayers(s.campaign, s.mapID).GetSurface())
		if err != nil || got.Get(2, 2) != grid.SurfaceWater || got.Get(3, 2) != grid.SurfaceWater || got.Get(4, 2) != grid.SurfaceClimb || got.Get(5, 2) != grid.SurfaceFloor {
			t.Fatalf("the surface read back = %v, %v", got, err)
		}
	}
	terrain, err := s.h.svc.Terrain(t.Context(), nil, s.campaign, s.mapID)
	if err != nil || terrain.Surface.Get(2, 2) != grid.SurfaceWater || terrain.Surface.Get(4, 2) != grid.SurfaceClimb || terrain.Validate() != nil {
		t.Fatalf("Terrain surface = %v, %v", terrain.Surface, err)
	}

	// Painting 0 clears; the layer with nothing left is empty again; 3 and -1 are refused whole.
	_, err = m.paint(s.campaign, s.mapID, surface, 3, [2]int32{0, 0})
	wantCode(t, "paint 3 on the surface", err, connect.CodeInvalidArgument)
	m.mustPaint(s.campaign, s.mapID, surface, 0, [2]int32{2, 2}, [2]int32{3, 2}, [2]int32{4, 2})
	if got := m.mustLayers(s.campaign, s.mapID).GetSurface(); len(got) != 0 {
		t.Errorf("a cleared surface has %d bytes, want none", len(got))
	}
	_, err = s.ana.paint(s.campaign, s.mapID, surface, 1, [2]int32{0, 0})
	wantCode(t, "a player paints the surface", err, connect.CodePermissionDenied)
}

// RN-10: on a fog map a player reads the surface of the squares they see or remember, and nothing else.
func TestSurfaceLayerIsFilteredByFog(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	m.mustSetGrid(s.campaign, s.mapID, 20)
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_SURFACE, 1, [2]int32{6, 6})
	if _, err := m.maps.SetMapFog(t.Context(), connect.NewRequest(&mapsv1.SetMapFogRequest{CampaignId: s.campaign, MapId: s.mapID, FogEnabled: new(true)})); err != nil {
		t.Fatal(err)
	}
	res := s.ana.mustLayers(s.campaign, s.mapID) // Ana has no character on the map: she sees nothing
	if !res.GetFogWithheld() || len(res.GetSurface()) != 0 {
		t.Errorf("a player who sees nothing reads %d bytes of surface (withheld %v), want none", len(res.GetSurface()), res.GetFogWithheld())
	}
	if got := m.mustLayers(s.campaign, s.mapID).GetSurface(); len(got) == 0 {
		t.Error("the fog took the master's surface away")
	}
}

// A calibration that is a multiple scales the surface with the other layers.
func TestSurfaceLayerIsScaledByCalibration(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	m.mustSetCalibration(s.campaign, s.mapID, 8, 1)
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_SURFACE, 2, [2]int32{1, 1})
	m.mustSetCalibration(s.campaign, s.mapID, 8, 2)
	got, err := grid.DecodeSurfaceLayer(grid.Grid{Columns: 16, Rows: 12}, m.mustLayers(s.campaign, s.mapID).GetSurface())
	if err != nil {
		t.Fatal(err)
	}
	for _, sq := range [][2]int{{2, 2}, {3, 2}, {2, 3}, {3, 3}} {
		if got.Get(sq[0], sq[1]) != grid.SurfaceClimb {
			t.Errorf("square %v is not climbable after the calibration", sq)
		}
	}
	if got.Get(0, 0) != grid.SurfaceFloor || got.Get(4, 2) != grid.SurfaceFloor {
		t.Error("the calibration painted squares nobody painted")
	}
}
