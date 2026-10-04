package maps

import (
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// The painted layers and the fog's settings (MR-034, MR-036; Etapa 9, D2, D6).
// The table is newScenes': a revealed map whose image is 40 x 30, so 20 columns
// make a grid of 20 x 15 squares.

var testGrid = grid.Grid{Columns: 20, Rows: 15}

func (u *user) setGrid(campaignID, mapID string, columns int32) (*mapsv1.Map, error) {
	res, err := u.maps.SetMapGrid(u.h.t.Context(), connect.NewRequest(&mapsv1.SetMapGridRequest{CampaignId: campaignID, MapId: mapID, Columns: columns}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetMap(), nil
}

func (u *user) mustSetGrid(campaignID, mapID string, columns int32) *mapsv1.Map {
	u.h.t.Helper()
	m, err := u.setGrid(campaignID, mapID, columns)
	if err != nil {
		u.h.t.Fatalf("SetMapGrid(%d) error = %v", columns, err)
	}
	return m
}

func squares(cells ...[2]int32) []*mapsv1.MapSquare {
	out := make([]*mapsv1.MapSquare, 0, len(cells))
	for _, c := range cells {
		out = append(out, &mapsv1.MapSquare{Col: c[0], Row: c[1]})
	}
	return out
}

func (u *user) paint(campaignID, mapID string, layer mapsv1.MapLayer, value int32, cells ...[2]int32) (*mapsv1.PaintMapCellsResponse, error) {
	res, err := u.maps.PaintMapCells(u.h.t.Context(), connect.NewRequest(&mapsv1.PaintMapCellsRequest{
		CampaignId: campaignID, MapId: mapID, Layer: layer, Value: value, Squares: squares(cells...),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (u *user) mustPaint(campaignID, mapID string, layer mapsv1.MapLayer, value int32, cells ...[2]int32) *mapsv1.PaintMapCellsResponse {
	u.h.t.Helper()
	res, err := u.paint(campaignID, mapID, layer, value, cells...)
	if err != nil {
		u.h.t.Fatalf("PaintMapCells(%v, %d) error = %v", layer, value, err)
	}
	return res
}

func (u *user) layers(campaignID, mapID string) (*mapsv1.GetMapLayersResponse, error) {
	res, err := u.maps.GetMapLayers(u.h.t.Context(), connect.NewRequest(&mapsv1.GetMapLayersRequest{CampaignId: campaignID, MapId: mapID}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (u *user) mustLayers(campaignID, mapID string) *mapsv1.GetMapLayersResponse {
	u.h.t.Helper()
	res, err := u.layers(campaignID, mapID)
	if err != nil {
		u.h.t.Fatalf("GetMapLayers() error = %v", err)
	}
	return res
}

// wantMapBlocked checks that err is a failed_precondition with this MapBlocked
// reason.
func wantMapBlocked(t *testing.T, call string, err error, reason mapsv1.MapBlockedReason) {
	t.Helper()
	wantCode(t, call, err, connect.CodeFailedPrecondition)
	ce, _ := errors.AsType[*connect.Error](err)
	if ce == nil {
		return
	}
	for _, d := range ce.Details() {
		if msg, derr := d.Value(); derr == nil {
			if b, ok := msg.(*mapsv1.MapBlocked); ok && b.GetReason() == reason {
				return
			}
		}
	}
	t.Errorf("%s error = %v, want a MapBlocked detail with %v", call, err, reason)
}

// decoded is the four layers of a GetMapLayers answer, read with package
// rules/grid: the web decodes the same bytes.
type decoded struct {
	terrain *grid.Layer
	walls   *grid.Layer
	cover   *grid.CoverLayer
	light   *grid.LightLayer
}

func decode(t *testing.T, res *mapsv1.GetMapLayersResponse) decoded {
	t.Helper()
	// A layer with nothing painted is empty: it reads as a layer of zeros.
	pad := func(b []byte, size int) []byte {
		if len(b) == 0 {
			return make([]byte, size)
		}
		return b
	}
	var d decoded
	var err error
	if d.terrain, err = grid.DecodeLayer(testGrid, pad(res.GetDifficultTerrain(), grid.LayerSize(testGrid))); err != nil {
		t.Fatalf("decode the terrain: %v", err)
	}
	if d.walls, err = grid.DecodeLayer(testGrid, pad(res.GetWall(), grid.LayerSize(testGrid))); err != nil {
		t.Fatalf("decode the walls: %v", err)
	}
	if d.cover, err = grid.DecodeCoverLayer(testGrid, pad(res.GetCover(), grid.CoverLayerSize(testGrid))); err != nil {
		t.Fatalf("decode the cover: %v", err)
	}
	if d.light, err = grid.DecodeLightLayer(testGrid, pad(res.GetLight(), grid.LightLayerSize(testGrid))); err != nil {
		t.Fatalf("decode the light: %v", err)
	}
	return d
}

// MR-034, D2: the master paints the four layers in batches, the last write wins,
// the bytes are package rules/grid's, and painting what is already there changes
// nothing.
func TestMR034_PaintingTheLayers(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	if got := s.master.mustGetMap(s.campaign, s.mapID).GetMap(); got.GetGridColumns() != 0 || got.GetLayersRevision() != 0 {
		t.Fatalf("a new map has grid %d, layers_revision %d; want none", got.GetGridColumns(), got.GetLayersRevision())
	}
	grid20 := m.mustSetGrid(s.campaign, s.mapID, 20)
	if grid20.GetGridColumns() != 20 || grid20.GetGridRows() != 15 {
		t.Fatalf("grid = %d x %d, want 20 x 15", grid20.GetGridColumns(), grid20.GetGridRows())
	}
	if res := m.mustLayers(s.campaign, s.mapID); len(res.GetWall())+len(res.GetCover())+len(res.GetLight())+len(res.GetDifficultTerrain()) != 0 || res.GetGridColumns() != 20 || res.GetGridRows() != 15 {
		t.Errorf("a map with nothing painted has layers %v, want all empty and the grid", res)
	}

	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{3, 2}, [2]int32{4, 2}, [2]int32{19, 14})
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_DIFFICULT_TERRAIN, 1, [2]int32{0, 0})
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_COVER, 1, [2]int32{5, 5})
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_COVER, 2, [2]int32{6, 5})
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_LIGHT, 3, [2]int32{1, 1})
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_LIGHT, 2, [2]int32{2, 1})
	last := m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_LIGHT, 1, [2]int32{3, 1})
	d := decode(t, m.mustLayers(s.campaign, s.mapID))
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"wall (3,2)", d.walls.Get(3, 2), true},
		{"wall (4,2)", d.walls.Get(4, 2), true},
		{"wall (19,14)", d.walls.Get(19, 14), true},
		{"wall (5,2)", d.walls.Get(5, 2), false},
		{"terrain (0,0)", d.terrain.Get(0, 0), true},
		{"terrain (3,2)", d.terrain.Get(3, 2), false},
		{"cover (5,5)", d.cover.Get(5, 5), grid.CoverHalf},
		{"cover (6,5)", d.cover.Get(6, 5), grid.CoverThreeQuarters},
		{"light (1,1)", d.light.Get(1, 1), grid.Bright},
		{"light (2,1)", d.light.Get(2, 1), grid.Dim},
		{"light (3,1)", d.light.Get(3, 1), grid.Dark},
		{"light (4,1)", d.light.Get(4, 1), grid.Unpainted},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if got := m.mustGetMap(s.campaign, s.mapID).GetMap().GetLayersRevision(); got != last.GetLayersRevision() || got != 7 {
		t.Errorf("layers_revision = %d (the last paint said %d), want 7: one a change", got, last.GetLayersRevision())
	}

	// The last write wins: no revision to send, a later value replaces an
	// earlier one, and 0 clears.
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_COVER, 2, [2]int32{5, 5})
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 0, [2]int32{3, 2})
	d = decode(t, m.mustLayers(s.campaign, s.mapID))
	if d.cover.Get(5, 5) != grid.CoverThreeQuarters || d.walls.Get(3, 2) || !d.walls.Get(4, 2) {
		t.Errorf("after overwriting: cover (5,5) = %v, wall (3,2) = %v, wall (4,2) = %v", d.cover.Get(5, 5), d.walls.Get(3, 2), d.walls.Get(4, 2))
	}

	// Idempotent: the same paint again changes nothing, and bumps nothing; a
	// repeated square in a batch counts once.
	before := m.mustGetMap(s.campaign, s.mapID).GetMap().GetLayersRevision()
	again := m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{4, 2}, [2]int32{4, 2})
	if again.GetChanged() != 0 || again.GetLayersRevision() != before {
		t.Errorf("painting what is there: changed %d, revision %d (was %d); want 0 and the same", again.GetChanged(), again.GetLayersRevision(), before)
	}
	twice := m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{8, 8}, [2]int32{8, 8})
	if twice.GetChanged() != 1 || twice.GetLayersRevision() != before+1 {
		t.Errorf("painting a new square twice: changed %d, revision %d; want 1 and %d", twice.GetChanged(), twice.GetLayersRevision(), before+1)
	}
	// Clearing every painted square leaves a layer with nothing painted: empty.
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_DIFFICULT_TERRAIN, 0, [2]int32{0, 0})
	if got := m.mustLayers(s.campaign, s.mapID).GetDifficultTerrain(); len(got) != 0 {
		t.Errorf("a cleared layer has %d bytes, want none", len(got))
	}
}

// The refusals of PaintMapCells: a batch is all or nothing, and the values and
// the squares are checked against the layer and the grid.
func TestMR034_PaintingRefusals(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	wantMapBlocked(t, "paint on a map with no grid", func() error {
		_, err := m.paint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{0, 0})
		return err
	}(), mapsv1.MapBlockedReason_MAP_BLOCKED_REASON_NO_GRID)
	if res := m.mustLayers(s.campaign, s.mapID); res.GetGridColumns() != 0 || len(res.GetWall()) != 0 {
		t.Errorf("a map with no grid has layers %v, want none", res)
	}
	m.mustSetGrid(s.campaign, s.mapID, 20)

	var tooMany [][2]int32
	for i := range 401 {
		tooMany = append(tooMany, [2]int32{int32(i % 20), int32(i / 20 % 15)}) // repeats past 300: the grid is 20 x 15
	}
	for _, c := range []struct {
		name  string
		layer mapsv1.MapLayer
		value int32
		cells [][2]int32
	}{
		{"a square outside, after good ones", mapsv1.MapLayer_MAP_LAYER_WALL, 1, [][2]int32{{0, 0}, {1, 1}, {20, 0}}},
		{"a row outside", mapsv1.MapLayer_MAP_LAYER_WALL, 1, [][2]int32{{0, 15}}},
		{"a negative square", mapsv1.MapLayer_MAP_LAYER_WALL, 1, [][2]int32{{-1, 0}}},
		{"no squares", mapsv1.MapLayer_MAP_LAYER_WALL, 1, nil},
		{"401 squares", mapsv1.MapLayer_MAP_LAYER_WALL, 1, tooMany},
		{"no layer", mapsv1.MapLayer_MAP_LAYER_UNSPECIFIED, 1, [][2]int32{{0, 0}}},
		{"2 on a wall", mapsv1.MapLayer_MAP_LAYER_WALL, 2, [][2]int32{{0, 0}}},
		{"2 on terrain", mapsv1.MapLayer_MAP_LAYER_DIFFICULT_TERRAIN, 2, [][2]int32{{0, 0}}},
		{"3 on cover", mapsv1.MapLayer_MAP_LAYER_COVER, 3, [][2]int32{{0, 0}}},
		{"4 on light", mapsv1.MapLayer_MAP_LAYER_LIGHT, 4, [][2]int32{{0, 0}}},
		{"-1", mapsv1.MapLayer_MAP_LAYER_WALL, -1, [][2]int32{{0, 0}}},
	} {
		_, err := m.paint(s.campaign, s.mapID, c.layer, c.value, c.cells...)
		wantCode(t, "paint: "+c.name, err, connect.CodeInvalidArgument)
	}
	// 400 squares is the cap, and a whole batch fits.
	if res := m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_DIFFICULT_TERRAIN, 1, tooMany[:400]...); res.GetChanged() != 300 {
		t.Errorf("painting 400 squares changed %d, want 300 (the grid has 20 x 15)", res.GetChanged())
	}
	// A refused batch left nothing behind: its good squares are not there.
	if d := decode(t, m.mustLayers(s.campaign, s.mapID)); d.walls.Count() != 0 {
		t.Errorf("a refused batch painted %d wall squares, want none", d.walls.Count())
	}
	// A layer's refusals are the master's: a player may not paint, and an ID
	// that is not a map is not found.
	_, err := s.ana.paint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{0, 0})
	wantCode(t, "a player paints", err, connect.CodePermissionDenied)
	_, err = m.paint(s.campaign, newKey(), mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{0, 0})
	wantCode(t, "paint on a map that is not there", err, connect.CodeNotFound)
}

// D2: other columns and a new image clear all four layers (and bump their
// revision); the same columns, the same image and a rename keep them. Turning the
// grid off turns the fog off.
func TestMR034_AGridOrImageChangeClearsTheLayers(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	paintAll := func() int32 {
		m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{1, 1})
		m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_DIFFICULT_TERRAIN, 1, [2]int32{2, 2})
		m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_COVER, 1, [2]int32{3, 3})
		return m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_LIGHT, 3, [2]int32{4, 4}).GetLayersRevision()
	}
	painted := func(res *mapsv1.GetMapLayersResponse) int {
		return len(res.GetWall()) + len(res.GetDifficultTerrain()) + len(res.GetCover()) + len(res.GetLight())
	}
	m.mustSetGrid(s.campaign, s.mapID, 20)
	rev := paintAll()

	// The same columns, a new name, the same image: nothing is cleared.
	m.mustSetGrid(s.campaign, s.mapID, 20)
	current := m.mustGetMap(s.campaign, s.mapID).GetMap()
	name := "Estrada de Mirathel II"
	if _, err := m.maps.UpdateMap(t.Context(), connect.NewRequest(&mapsv1.UpdateMapRequest{
		CampaignId: s.campaign, MapId: s.mapID, Revision: current.GetRevision(), Name: &name, ImageId: proto.String(current.GetImage().GetId()),
	})); err != nil {
		t.Fatalf("UpdateMap() error = %v", err)
	}
	if res := m.mustLayers(s.campaign, s.mapID); painted(res) == 0 || res.GetLayersRevision() != rev {
		t.Fatalf("the same grid, name and image cleared the layers: %v (revision was %d)", res, rev)
	}

	// Other columns clear every layer, and the revision says so.
	m.mustSetGrid(s.campaign, s.mapID, 25)
	res := m.mustLayers(s.campaign, s.mapID)
	if painted(res) != 0 || res.GetLayersRevision() != rev+1 || res.GetGridColumns() != 25 {
		t.Errorf("after other columns: layers %v, want all empty, revision %d, 25 columns", res, rev+1)
	}

	// A new image clears them too.
	m.mustSetGrid(s.campaign, s.mapID, 20)
	rev = paintAll()
	current = m.mustGetMap(s.campaign, s.mapID).GetMap()
	other := m.newImage(s.campaign)
	if _, err := m.maps.UpdateMap(t.Context(), connect.NewRequest(&mapsv1.UpdateMapRequest{
		CampaignId: s.campaign, MapId: s.mapID, Revision: current.GetRevision(), ImageId: &other,
	})); err != nil {
		t.Fatalf("UpdateMap(image) error = %v", err)
	}
	if res := m.mustLayers(s.campaign, s.mapID); painted(res) != 0 || res.GetLayersRevision() != rev+1 {
		t.Errorf("after a new image: layers %v, want all empty and revision %d", res, rev+1)
	}

	// Removing the grid clears the layers and turns the fog off.
	m.mustSetGrid(s.campaign, s.mapID, 20)
	paintAll()
	if _, err := m.maps.SetMapFog(t.Context(), connect.NewRequest(&mapsv1.SetMapFogRequest{CampaignId: s.campaign, MapId: s.mapID, FogEnabled: proto.Bool(true)})); err != nil {
		t.Fatalf("SetMapFog(on) error = %v", err)
	}
	got := m.mustSetGrid(s.campaign, s.mapID, 0)
	if got.GetFogEnabled() || got.GetGridColumns() != 0 {
		t.Errorf("a map with the grid removed = %v, want no grid and no fog", got)
	}
	if res := m.mustLayers(s.campaign, s.mapID); painted(res) != 0 {
		t.Errorf("removing the grid left layers: %v", res)
	}
}

// D2: both changes are refused while a combat runs on the map, with the typed
// reason, and the layers stay; the same values and a rename are fine, and once the
// combat ends the change goes through.
func TestMR034_NoGridOrImageChangeDuringACombat(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	m := s.master
	m.mustSetGrid(s.campaign, s.mapID, 20)
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{1, 1})
	if _, err := m.setCurrentMap(s.campaign, s.mapID); err != nil {
		t.Fatalf("SetCurrentMap() error = %v", err)
	}
	started, err := m.combat.StartEncounter(t.Context(), connect.NewRequest(&playv1.StartEncounterRequest{
		CampaignId: s.campaign, IdempotencyKey: newKey(), Name: "Emboscada",
		Participants: []*playv1.Participant{{CharacterId: s.pens.GetId()}},
	}))
	if err != nil {
		t.Fatalf("StartEncounter() error = %v", err)
	}

	_, err = m.setGrid(s.campaign, s.mapID, 25)
	wantMapBlocked(t, "SetMapGrid during a combat", err, mapsv1.MapBlockedReason_MAP_BLOCKED_REASON_COMBAT_RUNNING)
	_, err = m.setGrid(s.campaign, s.mapID, 0)
	wantMapBlocked(t, "SetMapGrid(0) during a combat", err, mapsv1.MapBlockedReason_MAP_BLOCKED_REASON_COMBAT_RUNNING)
	current := m.mustGetMap(s.campaign, s.mapID).GetMap()
	_, err = m.maps.UpdateMap(t.Context(), connect.NewRequest(&mapsv1.UpdateMapRequest{
		CampaignId: s.campaign, MapId: s.mapID, Revision: current.GetRevision(), ImageId: proto.String(m.newImage(s.campaign)),
	}))
	wantMapBlocked(t, "UpdateMap(image) during a combat", err, mapsv1.MapBlockedReason_MAP_BLOCKED_REASON_COMBAT_RUNNING)
	if d := decode(t, m.mustLayers(s.campaign, s.mapID)); !d.walls.Get(1, 1) {
		t.Error("a refused change cleared the layers")
	}

	// What clears nothing goes through: the same columns, a rename, painting.
	m.mustSetGrid(s.campaign, s.mapID, 20)
	name := "Estrada em guerra"
	if _, err := m.maps.UpdateMap(t.Context(), connect.NewRequest(&mapsv1.UpdateMapRequest{CampaignId: s.campaign, MapId: s.mapID, Revision: current.GetRevision(), Name: &name})); err != nil {
		t.Errorf("a rename during a combat: %v", err)
	}
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_COVER, 1, [2]int32{2, 2})
	// A map the combat is not on is free.
	free := m.createMap(s.campaign, "Outro", m.newImage(s.campaign))
	m.mustSetGrid(s.campaign, free.GetId(), 20)
	m.mustSetGrid(s.campaign, free.GetId(), 30)

	if _, err := m.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{
		CampaignId: s.campaign, EncounterId: started.Msg.GetEncounter().GetId(), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	m.mustSetGrid(s.campaign, s.mapID, 25)
	if res := m.mustLayers(s.campaign, s.mapID); len(res.GetWall())+len(res.GetCover()) != 0 {
		t.Errorf("after the combat ended and the grid changed: layers %v, want empty", res)
	}
}

// D6: the fog's settings: off by default; the fog needs a grid; the master
// changes each one alone; turning it off keeps the layers; a player learns that
// the map has fog and what "Visão do grupo" says, but not the base light.
func TestMR036_FogSettings(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	fog := func(u *user, edit func(*mapsv1.SetMapFogRequest)) (*mapsv1.Map, error) {
		req := &mapsv1.SetMapFogRequest{CampaignId: s.campaign, MapId: s.mapID}
		edit(req)
		res, err := u.maps.SetMapFog(t.Context(), connect.NewRequest(req))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetMap(), nil
	}

	def := m.mustGetMap(s.campaign, s.mapID).GetMap()
	if def.GetFogEnabled() || def.GetBaseLight() != mapsv1.LightLevel_LIGHT_LEVEL_DARK || def.GetGroupVision() {
		t.Errorf("a new map: fog %v, base light %v, group vision %v; want off, DARK, off", def.GetFogEnabled(), def.GetBaseLight(), def.GetGroupVision())
	}
	_, err := fog(m, func(r *mapsv1.SetMapFogRequest) { r.FogEnabled = proto.Bool(true) })
	wantMapBlocked(t, "fog on a map with no grid", err, mapsv1.MapBlockedReason_MAP_BLOCKED_REASON_NO_GRID)
	// The other settings do not need a grid.
	if got, err := fog(m, func(r *mapsv1.SetMapFogRequest) { r.BaseLight = mapsv1.LightLevel_LIGHT_LEVEL_DIM.Enum() }); err != nil || got.GetBaseLight() != mapsv1.LightLevel_LIGHT_LEVEL_DIM || got.GetFogEnabled() {
		t.Errorf("base light DIM on a map with no grid: %v, %v", got, err)
	}
	m.mustSetGrid(s.campaign, s.mapID, 20)
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{1, 1})

	on, err := fog(m, func(r *mapsv1.SetMapFogRequest) { r.FogEnabled = proto.Bool(true); r.GroupVision = proto.Bool(true) })
	if err != nil || !on.GetFogEnabled() || !on.GetGroupVision() || on.GetBaseLight() != mapsv1.LightLevel_LIGHT_LEVEL_DIM {
		t.Fatalf("fog on with group vision = %v, %v; want both on and the base light kept", on, err)
	}
	// What a player gets of it.
	ana := s.ana.mustGetMap(s.campaign, s.mapID).GetMap()
	if !ana.GetFogEnabled() || !ana.GetGroupVision() || ana.GetBaseLight() != mapsv1.LightLevel_LIGHT_LEVEL_UNSPECIFIED {
		t.Errorf("Ana's map: fog %v, group vision %v, base light %v; want fog and group vision, no base light", ana.GetFogEnabled(), ana.GetGroupVision(), ana.GetBaseLight())
	}
	if got := s.ana.listMaps(s.campaign)[0]; !got.GetFogEnabled() || got.GetBaseLight() != mapsv1.LightLevel_LIGHT_LEVEL_UNSPECIFIED {
		t.Errorf("Ana's ListMaps entry: fog %v, base light %v", got.GetFogEnabled(), got.GetBaseLight())
	}

	// Each setting alone; off keeps the layers and the other settings.
	if got, err := fog(m, func(r *mapsv1.SetMapFogRequest) { r.BaseLight = mapsv1.LightLevel_LIGHT_LEVEL_BRIGHT.Enum() }); err != nil || got.GetBaseLight() != mapsv1.LightLevel_LIGHT_LEVEL_BRIGHT || !got.GetFogEnabled() {
		t.Errorf("base light BRIGHT: %v, %v", got, err)
	}
	off, err := fog(m, func(r *mapsv1.SetMapFogRequest) { r.FogEnabled = proto.Bool(false) })
	if err != nil || off.GetFogEnabled() || !off.GetGroupVision() || off.GetBaseLight() != mapsv1.LightLevel_LIGHT_LEVEL_BRIGHT {
		t.Errorf("fog off = %v, %v; want the other settings kept", off, err)
	}
	if d := decode(t, m.mustLayers(s.campaign, s.mapID)); !d.walls.Get(1, 1) {
		t.Error("turning the fog off cleared the layers")
	}

	_, err = fog(m, func(*mapsv1.SetMapFogRequest) {})
	wantCode(t, "SetMapFog with nothing", err, connect.CodeInvalidArgument)
	_, err = fog(m, func(r *mapsv1.SetMapFogRequest) { r.BaseLight = mapsv1.LightLevel_LIGHT_LEVEL_UNSPECIFIED.Enum() })
	wantCode(t, "SetMapFog with an unspecified base light", err, connect.CodeInvalidArgument)
	_, err = fog(s.ana, func(r *mapsv1.SetMapFogRequest) { r.FogEnabled = proto.Bool(true) })
	wantCode(t, "a player sets the fog", err, connect.CodePermissionDenied)
}

// D2, D6: what a player reads of the layers. The master reads all four. A player
// of a map they see, with no fog, reads the walls, the terrain and the cover, never
// the light; with the fog on, nothing (their reads come with the fog's slice). A
// hidden map is not found.
func TestMR034_PlayersReadOnlyWhatTheyMay(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	m.mustSetGrid(s.campaign, s.mapID, 20)
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{1, 1})
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_DIFFICULT_TERRAIN, 1, [2]int32{2, 2})
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_COVER, 2, [2]int32{3, 3})
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_LIGHT, 3, [2]int32{4, 4})

	master := decode(t, m.mustLayers(s.campaign, s.mapID))
	if master.light.Get(4, 4) != grid.Bright || !master.walls.Get(1, 1) {
		t.Fatal("the master does not read the four layers")
	}
	for _, u := range []*user{s.ana, s.caio} {
		res := u.mustLayers(s.campaign, s.mapID)
		d := decode(t, res)
		if !d.walls.Get(1, 1) || !d.terrain.Get(2, 2) || d.cover.Get(3, 3) != grid.CoverThreeQuarters {
			t.Errorf("a player does not read the walls, terrain and cover of a map without fog: %v", res)
		}
		if len(res.GetLight()) != 0 || res.GetFogWithheld() {
			t.Errorf("a player's light layer = %d bytes, withheld %v; want none, not withheld", len(res.GetLight()), res.GetFogWithheld())
		}
	}

	if _, err := m.maps.SetMapFog(t.Context(), connect.NewRequest(&mapsv1.SetMapFogRequest{CampaignId: s.campaign, MapId: s.mapID, FogEnabled: proto.Bool(true)})); err != nil {
		t.Fatal(err)
	}
	res := s.ana.mustLayers(s.campaign, s.mapID)
	if !res.GetFogWithheld() || len(res.GetWall())+len(res.GetDifficultTerrain())+len(res.GetCover())+len(res.GetLight()) != 0 || res.GetLayersRevision() != 0 {
		t.Errorf("a player's layers on a fog map = %v, want none, withheld and no revision", res)
	}
	if d := decode(t, m.mustLayers(s.campaign, s.mapID)); d.light.Get(4, 4) != grid.Bright {
		t.Error("the fog took the master's layers away")
	}
	if got := s.ana.mustGetMap(s.campaign, s.mapID).GetMap().GetLayersRevision(); got != 0 {
		t.Errorf("a player's Map.layers_revision on a fog map = %d, want 0", got)
	}

	// A hidden map is not found to a player, exactly as for GetMap.
	hidden := m.createMap(s.campaign, "Escondido", m.newImage(s.campaign))
	_, err := s.ana.layers(s.campaign, hidden.GetId())
	wantCode(t, "a player reads the layers of a hidden map", err, connect.CodeNotFound)
}

// D2: painting tells the watchers with `map_changed`: the master always, the
// players only for a layer they read, on a map with no fog. (The once-a-second
// limit is TestHintGateMergesTheHintsOfAnInterval.)
func TestMR034_LayerChangesReachTheRightStreams(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	m := s.master
	m.mustSetGrid(s.campaign, s.mapID, 20)
	probeMap := m.createMap(s.campaign, "Sonda", m.newImage(s.campaign)).GetId()
	anaWatch, masterWatch := s.ana.watch(s.campaign), m.watch(s.campaign)

	// A wall: everyone who reads it.
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{0, 0})
	masterWatch.mapChanged(s.mapID)
	anaWatch.mapChanged(s.mapID)
	// The light: the master only. Painting what is already there says nothing.
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_LIGHT, 3, [2]int32{9, 9})
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{0, 0})
	masterWatch.mapChanged(s.mapID)
	// With the fog on, the layers are the master's: the switch itself reaches the
	// players (the map has fog now), painting does not.
	if _, err := m.maps.SetMapFog(t.Context(), connect.NewRequest(&mapsv1.SetMapFogRequest{CampaignId: s.campaign, MapId: s.mapID, FogEnabled: proto.Bool(true)})); err != nil {
		t.Fatal(err)
	}
	masterWatch.mapChanged(s.mapID)
	anaWatch.mapChanged(s.mapID)
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{1, 1})
	masterWatch.mapChanged(s.mapID)
	s.probe(probeMap)
	if got := anaWatch.drain(probeMap); len(got) != 0 {
		t.Errorf("Ana's stream got %v for a light, a repeated wall and a wall on a fog map, want nothing", got)
	}
	if got := masterWatch.drain(probeMap); len(got) != 0 {
		t.Errorf("the master's stream got %v beyond the changes, want nothing", got)
	}
}

// hintGate lets the first hint of a key go at once, and merges the ones that follow
// within the interval into one sent when it ends, with the players flag of any.
func TestHintGateMergesTheHintsOfAnInterval(t *testing.T) {
	t.Parallel()
	const every = 300 * time.Millisecond
	var gate hintGate
	sent := make(chan bool, 10)
	send := func(players bool) { sent <- players }
	fire := func(key string, players bool) { gate.fire(key, every, players, send) }

	fire("a", false) // at once
	if got := <-sent; got {
		t.Error("the first hint went to the players, want the master only")
	}
	fire("a", false) // these four merge into one
	fire("a", true)
	fire("a", false)
	fire("a", false)
	fire("b", true) // another key has its own interval
	if got := <-sent; !got {
		t.Error("the other key's hint did not go at once to the players")
	}
	select {
	case got := <-sent:
		t.Fatalf("a hint %v went within the interval, want it held", got)
	case <-time.After(every / 3):
	}
	select {
	case got := <-sent:
		if !got {
			t.Error("the merged hint lost the players flag of one of them")
		}
	case <-time.After(5 * every):
		t.Fatal("the merged hint never went: the last change would stay unannounced")
	}
	select {
	case got := <-sent:
		t.Fatalf("a second merged hint %v went, want one", got)
	case <-time.After(every):
	}
	// After a quiet interval the next goes at once again.
	fire("a", true)
	select {
	case <-sent:
	case <-time.After(every / 3):
		t.Error("a hint after a quiet interval was held")
	}
}

// MR-034, RN-21: Terrain is what a combat walks over: the painted walls,
// difficult terrain and cover as grid.Terrain, open floor with no layers, and
// not_found for a map that is not the campaign's.
func TestMR034_TerrainForACombat(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	ctx := t.Context()
	m.mustSetGrid(s.campaign, s.mapID, 20)

	open, err := s.h.svc.Terrain(ctx, s.campaign, s.mapID)
	if err != nil || open.Grid != testGrid || open.Walls.Count() != 0 || open.Difficult.Count() != 0 || open.Validate() != nil {
		t.Fatalf("Terrain of an unpainted map = %+v, %v; want open floor on the 20 x 15 grid", open, err)
	}
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{3, 2})
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_DIFFICULT_TERRAIN, 1, [2]int32{0, 0})
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_COVER, 2, [2]int32{6, 5})
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_LIGHT, 3, [2]int32{1, 1})
	got, err := s.h.svc.Terrain(ctx, s.campaign, s.mapID)
	if err != nil {
		t.Fatalf("Terrain() error = %v", err)
	}
	if !got.Walls.Get(3, 2) || got.Walls.Count() != 1 || !got.Difficult.Get(0, 0) || got.Cover.Get(6, 5) != grid.CoverThreeQuarters {
		t.Errorf("Terrain = walls %d, difficult (0,0) %v, cover %v; want the painted squares", got.Walls.Count(), got.Difficult.Get(0, 0), got.Cover.Get(6, 5))
	}

	other := s.h.newUser("Outra")
	otherCampaign := s.h.newCampaign(other)
	if _, err := s.h.svc.Terrain(ctx, otherCampaign, s.mapID); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("Terrain of another campaign's map = %v, want not_found", err)
	}
	if _, err := s.h.svc.Terrain(ctx, s.campaign, "not-a-uuid"); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("Terrain of a bad id = %v, want not_found", err)
	}
}
