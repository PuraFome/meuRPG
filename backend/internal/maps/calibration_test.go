package maps

import (
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// The calibration of a map's grid (MR-025, RN-25, Etapa 10): "each square of this
// drawing is 3 m". The table is newScenes': a revealed map whose image is 40 x 30,
// so 8 drawn columns make 8 x 6 drawn squares (6 rows: round(8 * 30 / 40)), and a
// factor f makes the rules' grid 8f x 6f.

func (u *user) setCalibration(campaignID, mapID string, columns, factor int32) (*mapsv1.Map, error) {
	res, err := u.maps.SetMapGrid(u.h.t.Context(), connect.NewRequest(&mapsv1.SetMapGridRequest{
		CampaignId: campaignID, MapId: mapID, Columns: columns, SquareFactor: factor,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetMap(), nil
}

func (u *user) mustSetCalibration(campaignID, mapID string, columns, factor int32) *mapsv1.Map {
	u.h.t.Helper()
	m, err := u.setCalibration(campaignID, mapID, columns, factor)
	if err != nil {
		u.h.t.Fatalf("SetMapGrid(%d, factor %d) error = %v", columns, factor, err)
	}
	return m
}

// layersOn decodes every layer of an answer for the grid; an empty layer is a
// layer with nothing painted.
type layersOn struct {
	terrain, walls *grid.Layer
	cover          *grid.CoverLayer
	light          *grid.LightLayer
	doors          *grid.DoorLayer
}

func decodeOn(t *testing.T, g grid.Grid, res *mapsv1.GetMapLayersResponse) layersOn {
	t.Helper()
	pad := func(b []byte, size int) []byte {
		if len(b) == 0 {
			return make([]byte, size)
		}
		return b
	}
	var l layersOn
	var err error
	if l.terrain, err = grid.DecodeLayer(g, pad(res.GetDifficultTerrain(), grid.LayerSize(g))); err != nil {
		t.Fatalf("decode the terrain for %v: %v", g, err)
	}
	if l.walls, err = grid.DecodeLayer(g, pad(res.GetWall(), grid.LayerSize(g))); err != nil {
		t.Fatalf("decode the walls for %v: %v", g, err)
	}
	if l.cover, err = grid.DecodeCoverLayer(g, pad(res.GetCover(), grid.CoverLayerSize(g))); err != nil {
		t.Fatalf("decode the cover for %v: %v", g, err)
	}
	if l.light, err = grid.DecodeLightLayer(g, pad(res.GetLight(), grid.LightLayerSize(g))); err != nil {
		t.Fatalf("decode the light for %v: %v", g, err)
	}
	l.doors = doorsOf(t, g, res)
	return l
}

// Every factor is stored, read back by the master and by a player, and the rules'
// grid is the drawing's times the factor; a caller that never sends the factor
// gets 1.
func TestMR025_EveryFactorRoundTrips(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	m := s.master
	for _, f := range []int32{1, 2, 3, 4, 1} {
		got := m.mustSetCalibration(s.campaign, s.mapID, 8, f)
		if got.GetDrawnColumns() != 8 || got.GetDrawnRows() != 6 || got.GetSquareFactor() != f || got.GetGridColumns() != 8*f || got.GetGridRows() != 6*f {
			t.Errorf("factor %d: drawn %d x %d, factor %d, rules' grid %d x %d; want 8 x 6, %d, %d x %d",
				f, got.GetDrawnColumns(), got.GetDrawnRows(), got.GetSquareFactor(), got.GetGridColumns(), got.GetGridRows(), f, 8*f, 6*f)
		}
		seen := s.ana.mustGetMap(s.campaign, s.mapID).GetMap()
		if seen.GetDrawnColumns() != 8 || seen.GetSquareFactor() != f || seen.GetGridColumns() != 8*f || seen.GetGridRows() != 6*f {
			t.Errorf("factor %d: a player reads %d drawn, factor %d, grid %d x %d", f, seen.GetDrawnColumns(), seen.GetSquareFactor(), seen.GetGridColumns(), seen.GetGridRows())
		}
		res := m.mustLayers(s.campaign, s.mapID)
		if res.GetGridColumns() != 8*f || res.GetGridRows() != 6*f {
			t.Errorf("factor %d: the layers' grid is %d x %d", f, res.GetGridColumns(), res.GetGridRows())
		}
	}
	// Unset reads as 1, and the existing callers (a grid, no factor) are unchanged.
	m.mustSetCalibration(s.campaign, s.mapID, 8, 3)
	if got := m.mustSetGrid(s.campaign, s.mapID, 20); got.GetSquareFactor() != 1 || got.GetGridColumns() != 20 || got.GetDrawnColumns() != 20 || got.GetGridRows() != 15 {
		t.Errorf("SetMapGrid(20) = factor %d, grid %d x %d, drawn %d; want 1, 20 x 15, 20", got.GetSquareFactor(), got.GetGridColumns(), got.GetGridRows(), got.GetDrawnColumns())
	}
	// No grid: no drawn squares, and the factor reads 1.
	if got := m.mustSetGrid(s.campaign, s.mapID, 0); got.GetDrawnColumns() != 0 || got.GetDrawnRows() != 0 || got.GetSquareFactor() != 1 || got.GetGridColumns() != 0 {
		t.Errorf("no grid: drawn %d x %d, factor %d, grid %d", got.GetDrawnColumns(), got.GetDrawnRows(), got.GetSquareFactor(), got.GetGridColumns())
	}
	// Only the master calibrates.
	_, err := s.ana.setCalibration(s.campaign, s.mapID, 8, 2)
	wantCode(t, "a player calibrates", err, connect.CodePermissionDenied)
}

// The rules' grid stays within 200 x 400, and the refusal names the limit; the
// factor itself is 1 to 20.
func TestMR025_TheEngineGridLimits(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	// A tall image: 10 columns make 150 drawn rows, so a factor of 3 makes 450.
	tall := m.mustUpload(s.campaign, "alto.png", pngImage(t, 10, 150)).GetId()
	tallMap := m.createMap(s.campaign, "A torre", tall).GetId()

	for name, c := range map[string]struct {
		mapID           string
		columns, factor int32
		want            string
	}{
		"201 columns across":    {s.mapID, 101, 2, "200 x 400"},
		"401 rows down":         {tallMap, 10, 3, "200 x 400"},
		"a factor of 21":        {s.mapID, 8, 21, "square_factor"},
		"a negative factor":     {s.mapID, 8, -1, "square_factor"},
		"a factor without grid": {s.mapID, 0, 2, "square_factor"},
		"3 drawn columns":       {s.mapID, 3, 1, "columns"},
		"201 drawn columns":     {s.mapID, 201, 1, "columns"},
	} {
		_, err := m.setCalibration(s.campaign, c.mapID, c.columns, c.factor)
		wantCode(t, name, err, connect.CodeInvalidArgument)
		if err != nil && !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want it to name %q", name, err, c.want)
		}
	}
	// The limits themselves are fine: 100 x 2 is 200 columns, 10 x 2 on the tall map is 300 rows.
	if got := m.mustSetCalibration(s.campaign, s.mapID, 100, 2); got.GetGridColumns() != 200 {
		t.Errorf("100 columns of factor 2 make %d, want 200", got.GetGridColumns())
	}
	if got := m.mustSetCalibration(s.campaign, tallMap, 10, 2); got.GetGridRows() != 300 {
		t.Errorf("10 columns of factor 2 on the tall map make %d rows, want 300", got.GetGridRows())
	}
}

// painted is what the lossless tests put on an 8 x 6 map: every layer, doors of
// three states among them (one on a wall, as a secret door is).
func paintEverything(m *user, campaign, mapID string) {
	m.mustPaint(campaign, mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{0, 0}, [2]int32{7, 5}, [2]int32{3, 2})
	m.mustPaint(campaign, mapID, mapsv1.MapLayer_MAP_LAYER_DIFFICULT_TERRAIN, 1, [2]int32{1, 1}, [2]int32{2, 4})
	m.mustPaint(campaign, mapID, mapsv1.MapLayer_MAP_LAYER_COVER, 1, [2]int32{4, 0})
	m.mustPaint(campaign, mapID, mapsv1.MapLayer_MAP_LAYER_COVER, 2, [2]int32{6, 3})
	m.mustPaint(campaign, mapID, mapsv1.MapLayer_MAP_LAYER_LIGHT, 3, [2]int32{5, 1})
	m.mustPaint(campaign, mapID, mapsv1.MapLayer_MAP_LAYER_LIGHT, 1, [2]int32{0, 5})
	m.mustPaint(campaign, mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_LOCKED), [2]int32{3, 1})
	m.mustPaint(campaign, mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_BARRED), [2]int32{2, 3})
	m.mustPaint(campaign, mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_SECRET), [2]int32{3, 2}) // on the wall
}

// sameAsScaled checks, square by square, that every layer on the new grid is the
// original's scaled by k.
func sameAsScaled(t *testing.T, name string, old, now layersOn, k int) {
	t.Helper()
	g := now.walls.Grid()
	for row := range g.Rows {
		for col := range g.Columns {
			oc, or := col/k, row/k
			if now.walls.Get(col, row) != old.walls.Get(oc, or) || now.terrain.Get(col, row) != old.terrain.Get(oc, or) ||
				now.cover.Get(col, row) != old.cover.Get(oc, or) || now.light.Get(col, row) != old.light.Get(oc, or) ||
				now.doors.Get(col, row) != old.doors.Get(oc, or) {
				t.Fatalf("%s: square (%d, %d) is not its origin (%d, %d) scaled by %d", name, col, row, oc, or, k)
			}
		}
	}
}

// MR-025: a calibration that is a multiple of the old keeps everything painted,
// scaled, whichever layer (the doors too), the revision goes up once, and the same
// calibration again changes nothing.
func TestMR025_AMultipleScalesEveryLayer(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	m.mustSetCalibration(s.campaign, s.mapID, 8, 1)
	paintEverything(m, s.campaign, s.mapID)
	g1 := grid.Grid{Columns: 8, Rows: 6}
	original := decodeOn(t, g1, m.mustLayers(s.campaign, s.mapID))
	if original.walls.Count() != 3 || original.doors.Count() != 3 || original.light.Get(5, 1) != grid.Bright {
		t.Fatalf("the test's own paint is wrong: %d walls, %d doors", original.walls.Count(), original.doors.Count())
	}
	rev := m.mustGetMap(s.campaign, s.mapID).GetMap().GetLayersRevision()

	// 1 to 2.
	m.mustSetCalibration(s.campaign, s.mapID, 8, 2)
	g2 := grid.Grid{Columns: 16, Rows: 12}
	res := m.mustLayers(s.campaign, s.mapID)
	now := decodeOn(t, g2, res)
	sameAsScaled(t, "1 to 2", original, now, 2)
	if now.walls.Count() != 3*4 || now.doors.Count() != 3*4 {
		t.Errorf("1 to 2: %d wall squares and %d door squares, want 12 each", now.walls.Count(), now.doors.Count())
	}
	if res.GetLayersRevision() != rev+1 {
		t.Errorf("1 to 2: layers revision %d, want %d", res.GetLayersRevision(), rev+1)
	}
	// The same calibration again: nothing changes, the revision stays.
	m.mustSetCalibration(s.campaign, s.mapID, 8, 2)
	if again := m.mustLayers(s.campaign, s.mapID); again.GetLayersRevision() != rev+1 {
		t.Errorf("the same calibration again moved the revision to %d", again.GetLayersRevision())
	}
	// 2 to 6 (a multiple, by 3): from the first drawing, the squares are 6 x 6 blocks.
	m.mustSetCalibration(s.campaign, s.mapID, 8, 6)
	g6 := grid.Grid{Columns: 48, Rows: 36}
	sameAsScaled(t, "2 to 6", original, decodeOn(t, g6, m.mustLayers(s.campaign, s.mapID)), 6)
	if got := m.mustGetMap(s.campaign, s.mapID).GetMap(); got.GetLayersRevision() != rev+2 {
		t.Errorf("2 to 6: layers revision %d, want %d", got.GetLayersRevision(), rev+2)
	}
}

// A change that loses something clears, as a grid change always did: other
// drawn columns, a smaller factor, a factor that is not a multiple, and a grid
// set on a map that had none.
func TestMR025_AnythingElseClears(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	for name, next := range map[string][2]int32{
		"other columns, same factor": {10, 2}, "a smaller factor": {8, 1}, "not a multiple": {8, 3},
		"other columns and no factor": {9, 1},
	} {
		m.mustSetCalibration(s.campaign, s.mapID, 8, 2)
		m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{1, 1})
		m.mustPaint(s.campaign, s.mapID, doors, 2, [2]int32{2, 2})
		rev := m.mustGetMap(s.campaign, s.mapID).GetMap().GetLayersRevision()
		got := m.mustSetCalibration(s.campaign, s.mapID, next[0], next[1])
		res := m.mustLayers(s.campaign, s.mapID)
		if len(res.GetWall()) != 0 || len(res.GetDoors()) != 0 || got.GetLayersRevision() != rev+1 {
			t.Errorf("%s: %d wall bytes, %d door bytes left, revision %d (was %d); want none and +1", name, len(res.GetWall()), len(res.GetDoors()), got.GetLayersRevision(), rev)
		}
	}
	// 2 to 4 after 4 to 2: a smaller one is lossy even when the other way was not.
	m.mustSetCalibration(s.campaign, s.mapID, 8, 4)
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{9, 9})
	m.mustSetCalibration(s.campaign, s.mapID, 8, 2)
	if res := m.mustLayers(s.campaign, s.mapID); len(res.GetWall()) != 0 {
		t.Error("going from 4 to 2 kept the walls")
	}
}

// The scale is refused while a combat that is not ended runs on the map (the
// combatants stand on squares of the old grid), the layers stay as they were, and
// once the combat ends it goes through.
func TestMR025_NotDuringACombat(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	m := s.master
	m.mustSetCalibration(s.campaign, s.mapID, 20, 1)
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
	_, err = m.setCalibration(s.campaign, s.mapID, 20, 2)
	wantMapBlocked(t, "a calibration during a combat", err, mapsv1.MapBlockedReason_MAP_BLOCKED_REASON_COMBAT_RUNNING)
	if got := m.mustGetMap(s.campaign, s.mapID).GetMap(); got.GetSquareFactor() != 1 || got.GetGridColumns() != 20 {
		t.Errorf("a refused calibration left factor %d, %d columns", got.GetSquareFactor(), got.GetGridColumns())
	}
	if d := decode(t, m.mustLayers(s.campaign, s.mapID)); !d.walls.Get(1, 1) {
		t.Error("a refused calibration changed the layers")
	}
	// The same calibration is not a change.
	m.mustSetCalibration(s.campaign, s.mapID, 20, 1)

	if _, err := m.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{
		CampaignId: s.campaign, EncounterId: started.Msg.GetEncounter().GetId(), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	m.mustSetCalibration(s.campaign, s.mapID, 20, 2)
}

// map_changed goes to the master and to a player who sees the map, for a scale as
// for any other change of the grid.
func TestMR025_ACalibrationReachesTheStreams(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	m := s.master
	m.mustSetCalibration(s.campaign, s.mapID, 8, 1)
	paintEverything(m, s.campaign, s.mapID)
	anaWatch, masterWatch := s.ana.watch(s.campaign), m.watch(s.campaign)
	m.mustSetCalibration(s.campaign, s.mapID, 8, 2)
	masterWatch.mapChanged(s.mapID)
	anaWatch.mapChanged(s.mapID)
}

// MR-025 with the fog (RN-10): the cave of the fog tests, calibrated from 1 to 2
// (24 x 16 squares of 3 m would be a drawing of 12 x 8; here the drawing stays at
// 24 columns and each square becomes 2 x 2 of 1,5 m). What every player saw stays
// seen, in whole blocks, and what a player receives is still only what they see or
// remember on the new grid: no wall, no token and no door far from them.
func TestMR025_TheFogKeepsWhatWasSeen(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	m := c.master
	m.mustPaint(c.campaign, c.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_CLOSED), [2]int32{12, 13}) // in a room nobody saw
	before := map[string][]byte{}
	for name, u := range map[string]*user{"ana": c.ana, "caio": c.caio} {
		before[name] = codes(t, u.mustVision(c.campaign, c.mapID))
	}

	got := m.mustSetCalibration(c.campaign, c.mapID, 24, 2)
	if got.GetGridColumns() != 48 || got.GetGridRows() != 32 || got.GetDrawnColumns() != 24 || got.GetDrawnRows() != 16 {
		t.Fatalf("the cave calibrated to 3 m: grid %d x %d, drawn %d x %d", got.GetGridColumns(), got.GetGridRows(), got.GetDrawnColumns(), got.GetDrawnRows())
	}
	big := grid.Grid{Columns: 48, Rows: 32}
	known := func(state byte) bool { return state != stateUnseen } // seen now, or remembered
	for name, u := range map[string]*user{"ana": c.ana, "caio": c.caio} {
		res := u.mustVision(c.campaign, c.mapID)
		if res.GetGridColumns() != 48 || res.GetGridRows() != 32 {
			t.Fatalf("%s: the vision is %d x %d, want 48 x 32", name, res.GetGridColumns(), res.GetGridRows())
		}
		after := codes(t, res)
		// Memory only grows: whatever the player knew of an old square, they know of its whole block.
		for row := range 16 {
			for col := range 24 {
				if !known(before[name][row*24+col]) {
					continue
				}
				for dr := range 2 {
					for dc := range 2 {
						if !known(after[(2*row+dr)*48+2*col+dc]) {
							t.Fatalf("%s: square (%d, %d) was known and its block's (%d, %d) is not", name, col, row, 2*col+dc, 2*row+dr)
						}
					}
				}
			}
		}
		// RN-10: the walls, the doors and the tokens they receive are inside what they know.
		layers := asApp(t, u.mustLayers(c.campaign, c.mapID))
		d := doorsOf(t, big, layers)
		for row := range big.Rows {
			for col := range big.Columns {
				if (bitOf(layers.GetWall(), big, col, row) || d.Get(col, row) != grid.DoorNone) && !known(after[row*48+col]) {
					t.Fatalf("%s receives a wall or a door at (%d, %d), which they neither see nor remember", name, col, row)
				}
			}
		}
		if d.Get(24, 26) != grid.DoorNone {
			t.Errorf("%s receives the door of the room nobody saw", name)
		}
		for _, tok := range u.mustGetMap(c.campaign, c.mapID).GetTokens() {
			if tok.GetName() == "Goblin Emboscado" {
				t.Errorf("%s receives the hidden goblin", name)
			}
		}
	}
	// The master reads the layers as painted, scaled: the cave's first wall square, (0, 0), is a block of four.
	if w := decodeOn(t, big, m.mustLayers(c.campaign, c.mapID)).walls; !w.Get(0, 0) || !w.Get(1, 1) || w.Get(34, 6) || w.Count() != 4*countWalls() {
		t.Errorf("the master's walls: %d squares, want %d", w.Count(), 4*countWalls())
	}
}

func countWalls() int {
	n := 0
	for _, line := range caveWalls {
		n += strings.Count(line, "#")
	}
	return n
}

// An image swap on a calibrated map keeps the drawn columns and the factor, so a
// taller image could pass the rules' 400 rows: it is refused, naming the limit, and
// nothing changes; one that fits goes through and clears the layers as ever.
func TestMR025_AnImageSwapCannotPassTheRows(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	small := m.mustUpload(s.campaign, "pequeno.png", pngImage(t, 10, 10)).GetId()
	tall := m.mustUpload(s.campaign, "alto.png", pngImage(t, 10, 30)).GetId()
	fits := m.mustUpload(s.campaign, "medio.png", pngImage(t, 10, 15)).GetId()
	id := m.createMap(s.campaign, "A torre", small).GetId()
	m.mustSetCalibration(s.campaign, id, 10, 20) // 200 x 200
	m.mustPaint(s.campaign, id, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{1, 1})

	swap := func(image string) error {
		cur := m.mustGetMap(s.campaign, id).GetMap()
		_, err := m.maps.UpdateMap(t.Context(), connect.NewRequest(&mapsv1.UpdateMapRequest{
			CampaignId: s.campaign, MapId: id, Revision: cur.GetRevision(), ImageId: &image,
		}))
		return err
	}
	err := swap(tall) // 30 drawn rows x 20 = 600
	wantCode(t, "a taller image on a calibrated map", err, connect.CodeInvalidArgument)
	if err != nil && !strings.Contains(err.Error(), "200 x 400") {
		t.Errorf("error = %v, want it to name the limit", err)
	}
	if got := m.mustGetMap(s.campaign, id).GetMap(); got.GetImage().GetId() != small || got.GetGridRows() != 200 {
		t.Errorf("the refused swap left image %s and %d rows", got.GetImage().GetId(), got.GetGridRows())
	}
	if d := decodeOn(t, grid.Grid{Columns: 200, Rows: 200}, m.mustLayers(s.campaign, id)); !d.walls.Get(1, 1) {
		t.Error("the refused swap cleared the layers")
	}
	if err := swap(fits); err != nil { // 15 x 20 = 300 rows
		t.Fatalf("an image that fits: %v", err)
	}
	if got := m.mustGetMap(s.campaign, id).GetMap(); got.GetGridRows() != 300 || got.GetGridColumns() != 200 {
		t.Errorf("after the swap the grid is %d x %d, want 200 x 300", got.GetGridColumns(), got.GetGridRows())
	}
}

// A refresh of what the players see that read the map before the calibration still
// holds the old grid's bitmap: it cannot write it over the scaled memory, because
// the calibration starts a new epoch.
func TestMR025_AStaleRefreshCannotOverwriteTheScaledMemory(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	before := c.h.mustMapRow(t, c.campaign, c.mapID)
	known := func(res *mapsv1.GetMapVisionResponse) int {
		n := 0
		for _, st := range codes(t, res) {
			if st != stateUnseen {
				n++
			}
		}
		return n
	}
	knownBefore := known(c.ana.mustVision(c.campaign, c.mapID))
	c.master.mustSetCalibration(c.campaign, c.mapID, 24, 2)
	after := c.h.mustMapRow(t, c.campaign, c.mapID)
	if after.VisionEpoch <= before.VisionEpoch {
		t.Fatalf("the epoch is %d after the calibration, was %d: it must go up", after.VisionEpoch, before.VisionEpoch)
	}
	stale := make([]byte, grid.LayerSize(caveGrid)) // the old grid's bitmap, all squares unseen
	n, err := c.h.svc.queries.UpsertMapVisionMemory(t.Context(), mapsdb.UpsertMapVisionMemoryParams{
		MapID: c.mapID, UserID: c.ana.id, Seen: stale, Epoch: before.VisionEpoch, UpdatedAt: time.Now().UTC().Truncate(time.Microsecond),
	})
	if err != nil || n != 0 {
		t.Fatalf("a stale refresh wrote %d rows (%v), want none", n, err)
	}
	if got := known(c.ana.mustVision(c.campaign, c.mapID)); got < 4*knownBefore {
		t.Errorf("Ana knows %d squares after the calibration, want at least 4 x %d", got, knownBefore)
	}
}

func (h *harness) mustMapRow(t *testing.T, campaignID, mapID string) mapsdb.Map {
	t.Helper()
	row, err := h.svc.queries.GetMap(t.Context(), mapsdb.GetMapParams{CampaignID: campaignID, ID: mapID})
	if err != nil {
		t.Fatalf("read the map: %v", err)
	}
	return row
}

// Every reader goes through gridOf: an image whose proportions do not divide
// (1200 x 850: 12 drawn columns make 9 drawn rows, and 24 columns would round to 17)
// gives 24 x 18 at factor 2 in the map, the layers, the vision, and the grid a combat
// copies when it starts.
func TestMR025_TheRowsAreTheDrawnRowsTimesTheFactor(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	m := s.master
	img := m.mustUpload(s.campaign, "torre.png", patternImage(t, 1200, 850)).GetId()
	id := m.createMap(s.campaign, "A torre em ruínas", img).GetId()
	m.setMapRevealed(s.campaign, id, true)
	got := m.mustSetCalibration(s.campaign, id, 12, 2)
	if got.GetDrawnRows() != 9 || got.GetGridColumns() != 24 || got.GetGridRows() != 18 {
		t.Fatalf("drawn rows %d, grid %d x %d; want 9, 24 x 18", got.GetDrawnRows(), got.GetGridColumns(), got.GetGridRows())
	}
	if l := m.mustLayers(s.campaign, id); l.GetGridColumns() != 24 || l.GetGridRows() != 18 {
		t.Errorf("the layers' grid is %d x %d", l.GetGridColumns(), l.GetGridRows())
	}
	m.mustPaint(s.campaign, id, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{23, 17})
	if _, err := m.paint(s.campaign, id, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{23, 18}); err == nil {
		t.Error("a square in row 18 was accepted on a grid of 18 rows")
	}
	if seen := s.ana.mustGetMap(s.campaign, id).GetMap(); seen.GetGridRows() != 18 || seen.GetDrawnRows() != 9 {
		t.Errorf("a player reads %d rows, %d drawn", seen.GetGridRows(), seen.GetDrawnRows())
	}
	if _, err := m.maps.SetMapFog(t.Context(), connect.NewRequest(&mapsv1.SetMapFogRequest{CampaignId: s.campaign, MapId: id, FogEnabled: new(true)})); err != nil {
		t.Fatalf("SetMapFog(on) error = %v", err)
	}
	if v := m.mustVision(s.campaign, id); v.GetGridColumns() != 24 || v.GetGridRows() != 18 {
		t.Errorf("the vision is %d x %d, want 24 x 18", v.GetGridColumns(), v.GetGridRows())
	}
	if _, err := m.setCurrentMap(s.campaign, id); err != nil {
		t.Fatalf("SetCurrentMap() error = %v", err)
	}
	started, err := m.combat.StartEncounter(t.Context(), connect.NewRequest(&playv1.StartEncounterRequest{
		CampaignId: s.campaign, IdempotencyKey: newKey(), Name: "Emboscada",
		Participants: []*playv1.Participant{{CharacterId: s.pens.GetId()}},
	}))
	if err != nil {
		t.Fatalf("StartEncounter() error = %v", err)
	}
	if e := started.Msg.GetEncounter(); e.GetGridColumns() != 24 || e.GetGridRows() != 18 {
		t.Errorf("the combat copied a grid of %d x %d, want 24 x 18", e.GetGridColumns(), e.GetGridRows())
	}
}

// The tile route at factor 2: the cave calibrated from 1 to 2 gives Ana tiles of the
// new grid, and no pixel of a square she does not know (RN-10).
func TestMR025_TilesOnACalibratedMap(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.master.mustSetCalibration(c.campaign, c.mapID, 24, 2)
	ana := c.ana.mustVision(c.campaign, c.mapID)
	if ana.GetGridColumns() != 48 || len(ana.GetTiles()) == 0 {
		t.Fatalf("Ana's vision: %d columns, %d tiles", ana.GetGridColumns(), len(ana.GetTiles()))
	}
	partial := false
	for _, tile := range ana.GetTiles() {
		shown, black := checkTileAt(t, "Ana", decodeTile(t, c.ana.get(tileURL(ana, tile))), ana, tile, 5)
		if shown == 0 {
			t.Errorf("tile (%d,%d) is in the index with no known square", tile.GetTx(), tile.GetTy())
		}
		partial = partial || black > 0
	}
	if !partial {
		t.Error("every tile was fully known: the test would not catch a leak")
	}
}

// The door is the square of the drawing: on a map calibrated to 2, painting one
// square of a door paints its 2 x 2 block, a move across opens the whole block
// once (one square comes back), a locked block stays locked together, and all four
// squares stop sight.
func TestMR025_ADoorIsTheWholeDrawnSquare(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	m.mustSetCalibration(s.campaign, s.mapID, 10, 2) // 20 x 16 squares of the rules
	g := grid.Grid{Columns: 20, Rows: 16}
	block := [][2]int{{2, 4}, {3, 4}, {2, 5}, {3, 5}}
	doorsNow := func() *grid.DoorLayer { return doorsOf(t, g, m.mustLayers(s.campaign, s.mapID)) }
	allAre := func(want grid.Door) bool {
		d := doorsNow()
		for _, c := range block {
			if d.Get(c[0], c[1]) != want {
				return false
			}
		}
		return d.Count() == 4
	}

	painted := m.mustPaint(s.campaign, s.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_CLOSED), [2]int32{3, 5})
	if painted.GetChanged() != 4 || !allAre(grid.DoorClosed) {
		t.Fatalf("painting one square of a door changed %d squares; want its block of 4, all closed", painted.GetChanged())
	}
	m.mustPaint(s.campaign, s.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_LOCKED), [2]int32{2, 4})
	if !allAre(grid.DoorLocked) {
		t.Error("locking one square did not lock the whole door")
	}
	m.mustPaint(s.campaign, s.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_CLOSED), [2]int32{2, 5})
	rev := m.mustGetMap(s.campaign, s.mapID).GetMap().GetLayersRevision()
	// The other layers stay square by square.
	if w := m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{3, 5}); w.GetChanged() != 1 {
		t.Errorf("a wall painted on one square changed %d", w.GetChanged())
	}
	rev++

	terrain, err := s.h.svc.Terrain(t.Context(), nil, s.campaign, s.mapID)
	if err != nil {
		t.Fatalf("Terrain() error = %v", err)
	}
	stops := grid.SightWalls(nil, terrain.Doors)
	for _, c := range block {
		if !stops.Get(c[0], c[1]) {
			t.Errorf("square (%d, %d) of a closed door does not stop sight", c[0], c[1])
		}
	}
	// A move that crosses two squares of the block opens it once.
	opened, err := s.h.svc.OpenDoors(t.Context(), nil, s.campaign, s.mapID, []grid.Square{{Col: 2, Row: 4}, {Col: 3, Row: 4}})
	if err != nil || len(opened) != 1 || !allAre(grid.DoorOpen) {
		t.Fatalf("OpenDoors() = %v, %v; want one square, and the whole block open", opened, err)
	}
	if got := m.mustGetMap(s.campaign, s.mapID).GetMap().GetLayersRevision(); got != rev+1 {
		t.Errorf("layers revision %d, want %d: one bump for the block", got, rev+1)
	}
	// An open door again opens nothing.
	if again, err := s.h.svc.OpenDoors(t.Context(), nil, s.campaign, s.mapID, []grid.Square{{Col: 3, Row: 5}}); err != nil || len(again) != 0 {
		t.Errorf("OpenDoors() on an open door = %v, %v", again, err)
	}
}

// Factor 1 paints and opens a door square by square, as before.
func TestMR025_FactorOneDoorsAreUnchanged(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	m.mustSetCalibration(s.campaign, s.mapID, 10, 1)
	if res := m.mustPaint(s.campaign, s.mapID, doors, 2, [2]int32{3, 5}); res.GetChanged() != 1 {
		t.Errorf("a door painted on one square changed %d at factor 1", res.GetChanged())
	}
}

// The drawing's 24 columns of 1,5 m and 12 of 3 m are the same rules' grid when the
// image's proportions agree: the layers stay, and so does the revision.
func TestMR025_TheSameRulesGridKeepsTheLayers(t *testing.T) {
	t.Parallel()
	c := newCave(t) // 240 x 160: 24 columns make 16 rows, 12 make 8 (x 2 = 16)
	rev := c.master.mustGetMap(c.campaign, c.mapID).GetMap().GetLayersRevision()
	got := c.master.mustSetCalibration(c.campaign, c.mapID, 12, 2)
	if got.GetGridColumns() != 24 || got.GetGridRows() != 16 || got.GetDrawnColumns() != 12 || got.GetSquareFactor() != 2 || got.GetLayersRevision() != rev {
		t.Errorf("grid %d x %d, drawn %d, factor %d, revision %d (was %d); want 24 x 16, 12, 2, unchanged", got.GetGridColumns(), got.GetGridRows(), got.GetDrawnColumns(), got.GetSquareFactor(), got.GetLayersRevision(), rev)
	}
	if w := decodeOn(t, caveGrid, c.master.mustLayers(c.campaign, c.mapID)).walls; w.Count() != countWalls() {
		t.Errorf("%d wall squares, want the cave's %d: nothing may be cleared", w.Count(), countWalls())
	}
}
