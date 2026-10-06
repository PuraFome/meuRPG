package maps

import (
	"slices"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// The doors layer (MR-010, RN-26, Etapa 10): the master paints it, and each
// player reads it as they know it. The table of the first tests is newScenes':
// a revealed map of 20 x 15 squares; the fog tests use the cave of fog_test.go.

const doors = mapsv1.MapLayer_MAP_LAYER_DOORS

// doorsOf decodes the doors layer of an answer for a grid, as the web does; an
// empty layer is a layer with no door.
func doorsOf(t *testing.T, g grid.Grid, res *mapsv1.GetMapLayersResponse) *grid.DoorLayer {
	t.Helper()
	b := res.GetDoors()
	if len(b) == 0 {
		b = make([]byte, grid.DoorLayerSize(g))
	}
	l, err := grid.DecodeDoorLayer(g, b)
	if err != nil {
		t.Fatalf("decode the doors: %v", err)
	}
	return l
}

// asApp reads an answer the way the app does: through its JSON.
func asApp(t *testing.T, res *mapsv1.GetMapLayersResponse) *mapsv1.GetMapLayersResponse {
	t.Helper()
	out := &mapsv1.GetMapLayersResponse{}
	if err := protojson.Unmarshal([]byte(asJSON(t, res)), out); err != nil {
		t.Fatalf("read the JSON: %v", err)
	}
	return out
}

func TestMR010_PaintingDoors(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	m.mustSetGrid(s.campaign, s.mapID, 20)

	// Every state, in a batch each: the bytes are package rules/grid's.
	for state, col := range map[mapsv1.DoorState]int32{
		mapsv1.DoorState_DOOR_STATE_OPEN: 1, mapsv1.DoorState_DOOR_STATE_CLOSED: 2, mapsv1.DoorState_DOOR_STATE_LOCKED: 3,
		mapsv1.DoorState_DOOR_STATE_BARRED: 4, mapsv1.DoorState_DOOR_STATE_SECRET: 5,
	} {
		m.mustPaint(s.campaign, s.mapID, doors, int32(state), [2]int32{col, 4})
	}
	d := doorsOf(t, testGrid, m.mustLayers(s.campaign, s.mapID))
	for col, want := range []grid.Door{grid.DoorNone, grid.DoorOpen, grid.DoorClosed, grid.DoorLocked, grid.DoorBarred, grid.DoorSecret, grid.DoorNone} {
		if got := d.Get(col, 4); got != want {
			t.Errorf("door (%d, 4) = %d, want %d", col, got, want)
		}
	}
	if got := m.mustLayers(s.campaign, s.mapID).GetDoors(); len(got) != grid.DoorLayerSize(testGrid) {
		t.Errorf("the doors take %d bytes, want %d (4 bits a square)", len(got), grid.DoorLayerSize(testGrid))
	}
	// The master reads them as painted, and the walls are not touched.
	if w := decode(t, m.mustLayers(s.campaign, s.mapID)).walls; w.Count() != 0 {
		t.Errorf("painting doors made %d walls", w.Count())
	}

	// A change bumps the revision, a repeat bumps nothing, the last write wins and
	// 0 clears (revealing a secret door is painting it closed).
	rev := m.mustGetMap(s.campaign, s.mapID).GetMap().GetLayersRevision()
	if again := m.mustPaint(s.campaign, s.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_SECRET), [2]int32{5, 4}); again.GetChanged() != 0 || again.GetLayersRevision() != rev {
		t.Errorf("painting what is there: changed %d, revision %d (was %d)", again.GetChanged(), again.GetLayersRevision(), rev)
	}
	revealed := m.mustPaint(s.campaign, s.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_CLOSED), [2]int32{5, 4})
	if revealed.GetChanged() != 1 || revealed.GetLayersRevision() != rev+1 {
		t.Errorf("revealing a secret door: changed %d, revision %d; want 1 and %d", revealed.GetChanged(), revealed.GetLayersRevision(), rev+1)
	}
	m.mustPaint(s.campaign, s.mapID, doors, 0, [2]int32{1, 4}, [2]int32{2, 4}, [2]int32{3, 4}, [2]int32{4, 4}, [2]int32{5, 4})
	if got := m.mustLayers(s.campaign, s.mapID).GetDoors(); len(got) != 0 {
		t.Errorf("every door cleared leaves %d bytes, want none", len(got))
	}

	// The batch rules: 1 to 400 squares inside the grid, a value 0 to 5, all or nothing.
	var tooMany [][2]int32
	for i := range 401 {
		tooMany = append(tooMany, [2]int32{int32(i % 20), int32(i / 20 % 15)})
	}
	for name, c := range map[string]struct {
		value int32
		cells [][2]int32
	}{
		"6": {6, [][2]int32{{0, 0}}}, "-1": {-1, [][2]int32{{0, 0}}}, "401 squares": {2, tooMany},
		"a square outside, after a good one": {2, [][2]int32{{0, 0}, {20, 0}}}, "no squares": {2, nil},
	} {
		_, err := m.paint(s.campaign, s.mapID, doors, c.value, c.cells...)
		wantCode(t, "paint doors: "+name, err, connect.CodeInvalidArgument)
	}
	if res := m.mustPaint(s.campaign, s.mapID, doors, 2, tooMany[:400]...); res.GetChanged() != 300 {
		t.Errorf("400 squares changed %d, want 300 (the grid has 20 x 15)", res.GetChanged())
	}
	if d := doorsOf(t, testGrid, m.mustLayers(s.campaign, s.mapID)); d.Count() != 300 {
		t.Errorf("%d doors after a full batch, want 300 and none from the refused batches", d.Count())
	}

	// The master's alone.
	_, err := s.ana.paint(s.campaign, s.mapID, doors, 2, [2]int32{0, 0})
	wantCode(t, "a player paints a door", err, connect.CodePermissionDenied)
}

func TestMR010_AGridOrImageChangeClearsTheDoors(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	m.mustSetGrid(s.campaign, s.mapID, 20)
	rev := m.mustPaint(s.campaign, s.mapID, doors, 3, [2]int32{2, 2}).GetLayersRevision()

	m.mustSetGrid(s.campaign, s.mapID, 20) // the same columns keep them
	if res := m.mustLayers(s.campaign, s.mapID); len(res.GetDoors()) == 0 || res.GetLayersRevision() != rev {
		t.Fatalf("the same grid cleared the doors: %v", res)
	}
	m.mustSetGrid(s.campaign, s.mapID, 25)
	if res := m.mustLayers(s.campaign, s.mapID); len(res.GetDoors()) != 0 || res.GetLayersRevision() != rev+1 {
		t.Errorf("after other columns: %d door bytes, revision %d; want none and %d", len(res.GetDoors()), res.GetLayersRevision(), rev+1)
	}

	m.mustSetGrid(s.campaign, s.mapID, 20)
	rev = m.mustPaint(s.campaign, s.mapID, doors, 3, [2]int32{2, 2}).GetLayersRevision()
	current := m.mustGetMap(s.campaign, s.mapID).GetMap()
	other := m.newImage(s.campaign)
	if _, err := m.maps.UpdateMap(t.Context(), connect.NewRequest(&mapsv1.UpdateMapRequest{
		CampaignId: s.campaign, MapId: s.mapID, Revision: current.GetRevision(), ImageId: &other,
	})); err != nil {
		t.Fatalf("UpdateMap(image) error = %v", err)
	}
	if res := m.mustLayers(s.campaign, s.mapID); len(res.GetDoors()) != 0 || res.GetLayersRevision() != rev+1 {
		t.Errorf("after a new image: %d door bytes, revision %d; want none and %d", len(res.GetDoors()), res.GetLayersRevision(), rev+1)
	}
}

// RN-26, RN-10: a player never sees a secret door (their walls have a wall there
// and their doors have nothing) and never learns a door is locked before trying
// it (it reads closed). Read as the app reads them: through the JSON.
func TestRN10_PlayersNeverSeeASecretDoor(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	m.mustSetGrid(s.campaign, s.mapID, 20)
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{9, 9})
	for state, col := range map[mapsv1.DoorState]int32{
		mapsv1.DoorState_DOOR_STATE_OPEN: 1, mapsv1.DoorState_DOOR_STATE_CLOSED: 2, mapsv1.DoorState_DOOR_STATE_LOCKED: 3,
		mapsv1.DoorState_DOOR_STATE_BARRED: 4, mapsv1.DoorState_DOOR_STATE_SECRET: 5,
	} {
		m.mustPaint(s.campaign, s.mapID, doors, int32(state), [2]int32{col, 4})
	}

	master := m.mustLayers(s.campaign, s.mapID)
	md := doorsOf(t, testGrid, master)
	if md.Get(3, 4) != grid.DoorLocked || md.Get(5, 4) != grid.DoorSecret || decode(t, master).walls.Get(5, 4) {
		t.Fatal("the master does not read the doors as painted, with no wall where the secret door is")
	}
	for _, u := range []*user{s.ana, s.caio} {
		res := asApp(t, u.mustLayers(s.campaign, s.mapID))
		d, walls := doorsOf(t, testGrid, res), decode(t, res).walls
		for col, want := range map[int]grid.Door{1: grid.DoorOpen, 2: grid.DoorClosed, 3: grid.DoorClosed, 4: grid.DoorBarred, 5: grid.DoorNone} {
			if got := d.Get(col, 4); got != want {
				t.Errorf("a player reads the door at (%d, 4) as %d, want %d", col, got, want)
			}
		}
		if !walls.Get(5, 4) || walls.Get(3, 4) || !walls.Get(9, 9) || walls.Count() != 2 {
			t.Errorf("a player's walls: secret door (5,4) = %v, locked door (3,4) = %v, count %d; want a wall at the secret door only, besides the painted one", walls.Get(5, 4), walls.Get(3, 4), walls.Count())
		}
		for n := 0; n < testGrid.Squares(); n++ { // no nibble anywhere says locked or secret
			if v := res.GetDoors()[n/2] >> (4 * (n % 2)) & 15; v == byte(grid.DoorLocked) || v == byte(grid.DoorSecret) {
				t.Fatalf("a player's doors hold %d at square %d", v, n)
			}
		}
	}
	// "Ver como" a player's character reads as the player does.
	as, err := m.maps.GetMapLayers(t.Context(), connect.NewRequest(&mapsv1.GetMapLayersRequest{CampaignId: s.campaign, MapId: s.mapID, AsCharacterId: s.pens.GetId()}))
	if err != nil {
		t.Fatalf("GetMapLayers(as) error = %v", err)
	}
	if d := doorsOf(t, testGrid, as.Msg); d.Get(3, 4) != grid.DoorClosed || d.Get(5, 4) != grid.DoorNone || !decode(t, as.Msg).walls.Get(5, 4) {
		t.Error("the master seeing as a character reads the locked and secret doors as the player does")
	}

	// Revealing the secret door shows it, as closed; the wall at its square goes.
	m.mustPaint(s.campaign, s.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_CLOSED), [2]int32{5, 4})
	res := s.ana.mustLayers(s.campaign, s.mapID)
	if doorsOf(t, testGrid, res).Get(5, 4) != grid.DoorClosed || decode(t, res).walls.Get(5, 4) {
		t.Error("a revealed secret door does not read as a closed door")
	}
}

// The fog: a player reads the doors of the squares their character sees or
// remembers, as they know them, and a closed door hides what is behind it from
// the vision, and opening it shows it again.
func TestRN10_FogFiltersTheDoors(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	m := c.master
	g := caveGrid
	// The corridor between the party's cave and the guard room is two squares high
	// at column 11: a secret door and a locked one close it.
	m.mustPaint(c.campaign, c.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_SECRET), [2]int32{11, 7})
	m.mustPaint(c.campaign, c.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_LOCKED), [2]int32{11, 8})
	// And a door in the lower room, which nobody has seen.
	m.mustPaint(c.campaign, c.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_CLOSED), [2]int32{12, 13})

	// Ana (Pensantus, darkvision) sees the corridor to the doors: the locked one
	// reads closed, the secret one is a wall, and the door in the lower room, which
	// she never saw, is not there.
	res := asApp(t, c.ana.mustLayers(c.campaign, c.mapID))
	d := doorsOf(t, g, res)
	if !res.GetFogWithheld() || d.Get(11, 8) != grid.DoorClosed || d.Get(11, 7) != grid.DoorNone || d.Get(12, 13) != grid.DoorNone || d.Count() != 1 {
		t.Errorf("Ana's doors: (11,8) %d, (11,7) %d, (12,13) %d, %d in all; want closed, none, none, 1", d.Get(11, 8), d.Get(11, 7), d.Get(12, 13), d.Count())
	}
	if !bitOf(res.GetWall(), g, 11, 7) || bitOf(res.GetWall(), g, 11, 8) {
		t.Error("Ana's walls: the secret door must be a wall and the locked one not")
	}
	// Caio (Toren, no darkvision) sees only what is lit: no door at all.
	if got := c.caio.mustLayers(c.campaign, c.mapID).GetDoors(); len(got) != 0 {
		t.Errorf("Caio, who sees the dark corridor not at all, got %d bytes of doors", len(got))
	}
	// The master reads them all as painted.
	if md := doorsOf(t, g, m.mustLayers(c.campaign, c.mapID)); md.Get(11, 7) != grid.DoorSecret || md.Get(11, 8) != grid.DoorLocked || md.Get(12, 13) != grid.DoorClosed {
		t.Error("the master does not read the doors as painted")
	}

	// A closed door stops sight: with the corridor shut, Goblin 2 (in the guard
	// room's light) is out of sight; open, it is seen again.
	m.mustPaint(c.campaign, c.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_CLOSED), [2]int32{11, 7})
	if slices.Contains(tokenNames(c.caio.mustGetMap(c.campaign, c.mapID)), "Goblin 2") {
		t.Error("Caio sees Goblin 2 through a closed door")
	}
	m.mustPaint(c.campaign, c.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_OPEN), [2]int32{11, 7}, [2]int32{11, 8})
	if !slices.Contains(tokenNames(c.caio.mustGetMap(c.campaign, c.mapID)), "Goblin 2") {
		t.Error("Caio does not see Goblin 2 through an open door")
	}
	// A barred door lets sight through too.
	m.mustPaint(c.campaign, c.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_BARRED), [2]int32{11, 7}, [2]int32{11, 8})
	if !slices.Contains(tokenNames(c.caio.mustGetMap(c.campaign, c.mapID)), "Goblin 2") {
		t.Error("Caio does not see Goblin 2 through bars")
	}
}

// What the combat walks over is the truth: the master's layers as painted, the
// locked and secret doors included. The player's known terrain is the filtered one.
func TestMR010_TerrainHasTheDoors(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	m := s.master
	m.mustSetGrid(s.campaign, s.mapID, 20)
	m.mustPaint(s.campaign, s.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_LOCKED), [2]int32{3, 2})
	got, err := s.h.svc.Terrain(t.Context(), nil, s.campaign, s.mapID)
	if err != nil || got.Doors.Get(3, 2) != grid.DoorLocked || got.Validate() != nil {
		t.Fatalf("Terrain() = doors %d, %v; want the locked door", got.Doors.Get(3, 2), err)
	}
}

// OpenDoors is the seam a move uses, inside its transaction: it opens the doors
// that are still closed (and only those), raises the layers' revision once, and
// the watchers are told after the commit.
func TestMR010_OpenDoorsForAMove(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	m := s.master
	m.mustSetGrid(s.campaign, s.mapID, 20)
	m.mustPaint(s.campaign, s.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_CLOSED), [2]int32{1, 1}, [2]int32{2, 1})
	m.mustPaint(s.campaign, s.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_LOCKED), [2]int32{3, 1})
	m.mustPaint(s.campaign, s.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_OPEN), [2]int32{4, 1})
	rev := m.mustGetMap(s.campaign, s.mapID).GetMap().GetLayersRevision()
	anaWatch, masterWatch := s.ana.watch(s.campaign), m.watch(s.campaign)

	var opened []grid.Square
	run := func(campaign string, sqs ...grid.Square) error {
		return db.InTx(t.Context(), s.h.pool, func(tx pgx.Tx) error {
			var err error
			opened, err = s.h.svc.OpenDoors(t.Context(), tx, campaign, s.mapID, sqs)
			return err
		})
	}
	sq := func(c, r int) grid.Square { return grid.Square{Col: c, Row: r} }
	if err := run(s.campaign, sq(1, 1), sq(3, 1), sq(4, 1), sq(5, 5), sq(2, 1)); err != nil {
		t.Fatalf("OpenDoors() error = %v", err)
	}
	if !slices.Equal(opened, []grid.Square{sq(1, 1), sq(2, 1)}) {
		t.Errorf("opened %v, want the two closed doors only (a locked and an open one, and a square with none, stay)", opened)
	}
	res := m.mustLayers(s.campaign, s.mapID)
	d := doorsOf(t, testGrid, res)
	if d.Get(1, 1) != grid.DoorOpen || d.Get(2, 1) != grid.DoorOpen || d.Get(3, 1) != grid.DoorLocked || d.Get(5, 5) != grid.DoorNone {
		t.Errorf("doors after: (1,1) %d (2,1) %d (3,1) %d (5,5) %d", d.Get(1, 1), d.Get(2, 1), d.Get(3, 1), d.Get(5, 5))
	}
	if got := m.mustGetMap(s.campaign, s.mapID).GetMap().GetLayersRevision(); got != rev+1 {
		t.Errorf("layers_revision = %d, want %d: one for the whole move", got, rev+1)
	}
	// Nothing to open: no write, no revision.
	if err := run(s.campaign, sq(1, 1), sq(3, 1)); err != nil || len(opened) != 0 {
		t.Errorf("opening nothing: %v, %v", opened, err)
	}
	if got := m.mustGetMap(s.campaign, s.mapID).GetMap().GetLayersRevision(); got != rev+1 {
		t.Errorf("layers_revision = %d after opening nothing, want %d", got, rev+1)
	}
	// Another campaign's map is not found.
	other := s.h.newUser("Outra")
	if err := run(s.h.newCampaign(other), sq(1, 1)); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("OpenDoors on another campaign's map = %v, want not_found", err)
	}

	// The hint goes out after the commit: the master and the players (no fog).
	s.h.svc.DoorsChanged(t.Context(), s.campaign, s.mapID)
	masterWatch.mapChanged(s.mapID)
	anaWatch.mapChanged(s.mapID)
}

// RN-10: a change of the doors that no player can see is the master's alone, as
// the light is: locking a closed door, or painting a secret door where a wall
// is, moves no number a player reads and sends them no hint (the master hears and
// reads the change); opening the door is something they see.
func TestRN10_ADoorChangeNoPlayerSeesLeavesThemAlone(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	m := s.master
	m.mustSetGrid(s.campaign, s.mapID, 20)
	m.mustPaint(s.campaign, s.mapID, mapsv1.MapLayer_MAP_LAYER_WALL, 1, [2]int32{3, 3})
	m.mustPaint(s.campaign, s.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_CLOSED), [2]int32{6, 6})
	probeMap := m.createMap(s.campaign, "Sonda", m.newImage(s.campaign)).GetId()
	anaWatch, masterWatch := s.ana.watch(s.campaign), m.watch(s.campaign)
	playerRev := func() int32 { return s.ana.mustLayers(s.campaign, s.mapID).GetLayersRevision() }
	masterRev := func() int32 { return m.mustLayers(s.campaign, s.mapID).GetLayersRevision() }
	anaBefore, masterBefore := playerRev(), masterRev()
	before := s.ana.mustLayers(s.campaign, s.mapID)

	m.mustPaint(s.campaign, s.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_SECRET), [2]int32{3, 3}) // on the wall
	m.mustPaint(s.campaign, s.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_LOCKED), [2]int32{6, 6}) // closed to locked
	masterWatch.mapChanged(s.mapID)
	masterWatch.mapChanged(s.mapID)
	s.probe(probeMap)
	if got := anaWatch.drain(probeMap); len(got) != 0 {
		t.Errorf("Ana's stream got %v for changes she cannot see, want nothing", got)
	}
	if got := playerRev(); got != anaBefore {
		t.Errorf("the player's layers_revision went %d to %d for changes she cannot see", anaBefore, got)
	}
	if got := masterRev(); got != masterBefore+2 {
		t.Errorf("the master's revision went %d to %d, want two more", masterBefore, got)
	}
	if after := s.ana.mustLayers(s.campaign, s.mapID); !proto.Equal(before, after) {
		t.Errorf("Ana's layers changed: %v then %v", before, after)
	}

	// Opening the door is news to her.
	m.mustPaint(s.campaign, s.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_OPEN), [2]int32{6, 6})
	anaWatch.mapChanged(s.mapID)
	if playerRev() == anaBefore {
		t.Error("the player's revision did not move when a door opened")
	}
}
