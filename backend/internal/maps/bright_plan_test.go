package maps

import (
	"os"
	"strings"
	"testing"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// The base light "Claro" shows the players the whole floor plan of a fog map (the
// image, the walls), while creatures and points still follow what the characters
// see (MR-036, RN-10). The cave is the table (fog_test.go): the goblin captain and
// the guardhouse are in the guard room, behind a wall from every character.

func (c *cave) setBaseLight(level mapsv1.LightLevel) {
	c.h.t.Helper()
	if _, err := c.master.maps.SetMapFog(c.h.t.Context(), connect.NewRequest(&mapsv1.SetMapFogRequest{CampaignId: c.campaign, MapId: c.mapID, BaseLight: level.Enum()})); err != nil {
		c.h.t.Fatalf("SetMapFog(base light) error = %v", err)
	}
}

func TestMR036_BrightShowsTheWholePlanButOnlyWhatIsInSight(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.setBaseLight(mapsv1.LightLevel_LIGHT_LEVEL_BRIGHT)
	guardWall := [2]int{15, 3} // the guard room's west wall (row 3 is "################.......#"), seen by no character

	// The terrain: every square is seen, there are tiles for the whole image, and the
	// walls of the guard room are in the layers.
	vis := c.ana.mustVision(c.campaign, c.mapID)
	for n, code := range codes(t, vis) {
		if code == 0 || code == 5 {
			t.Fatalf("square %d is %d on a Claro map, want every square drawn as seen\n%v", n, code, picture(t, vis, grid.Square{Col: -1, Row: -1}))
		}
	}
	if got, want := len(vis.GetTiles()), 2; got != want { // 24 x 16 squares, 16 per tile
		t.Errorf("tiles = %d, want %d: the whole image", got, want)
	}
	layers := decode2(t, c.ana.mustLayers(c.campaign, c.mapID))
	if !layers.walls.Get(guardWall[0], guardWall[1]) {
		t.Errorf("the wall at %v is not in Ana's layers on a Claro map", guardWall)
	}

	// The creatures and the points: by sight, as before. Goblin 2 stands in the
	// corridor's light and everyone sees it (fog_test.go); Goblin 1 and the captain in
	// the guard room, the hidden goblin and the guardhouse do not reach Ana.
	got := c.ana.mustGetMap(c.campaign, c.mapID)
	for _, name := range tokenNames(got) {
		switch name {
		case "Capitão Goblin", "Goblin 1", "Goblin Emboscado":
			t.Errorf("Ana received %q, who is behind a wall (RN-10): %v", name, tokenNames(got))
		}
	}
	for _, p := range got.GetPoints() {
		if p.GetId() == c.guardhouse.GetId() {
			t.Errorf("Ana received the guardhouse, which no character sees")
		}
	}
	// Positive control: the master sees them all.
	master := tokenNames(c.master.mustGetMap(c.campaign, c.mapID))
	for _, name := range []string{"Capitão Goblin", "Goblin Emboscado"} {
		if !contains(master, name) {
			t.Errorf("the master does not see %q: the control is broken (%v)", name, master)
		}
	}
}

func TestMR036_DimKeepsTheFogOfTheTerrain(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.setBaseLight(mapsv1.LightLevel_LIGHT_LEVEL_DIM)
	vis := c.ana.mustVision(c.campaign, c.mapID)
	if seenNow(t, vis) >= int(vis.GetGridColumns()*vis.GetGridRows()) {
		t.Fatalf("Ana sees the whole map in Penumbra")
	}
	unseen := 0
	for _, code := range codes(t, vis) {
		if code == 0 {
			unseen++
		}
	}
	if unseen == 0 {
		t.Errorf("no unseen square in Penumbra: the fog of the terrain is gone")
	}
	if layers := decode2(t, c.ana.mustLayers(c.campaign, c.mapID)); layers.walls.Get(16, 3) {
		t.Errorf("the guard room's wall reached Ana in Penumbra: she has not seen it")
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// decode2 is decode for the cave's grid.
func decode2(t *testing.T, res *mapsv1.GetMapLayersResponse) decoded {
	t.Helper()
	walls, err := grid.DecodeLayer(caveGrid, padTo(res.GetWall(), grid.LayerSize(caveGrid)))
	if err != nil {
		t.Fatalf("decode the walls: %v", err)
	}
	return decoded{walls: walls}
}

func padTo(b []byte, size int) []byte {
	if len(b) == 0 {
		return make([]byte, size)
	}
	return b
}

// A generated dungeon is created in Penumbra, and migration 00245 moves the ones still
// in Claro to it, only them, and is safe to run twice.
func TestMR010_TheDungeonsBaseLightMigrationMovesOnlyDungeonsAndRunsTwice(t *testing.T) {
	t.Parallel()
	d := newDungeonTable(t)
	m := d.master
	seed, _ := testDungeonSeed(t)
	dungeon := m.createDungeon(d.campaign, "Masmorra", testDungeonOptions(), seed).GetMap().GetId()
	hand := m.createMap(d.campaign, "Mapa à mão", m.newImage(d.campaign)).GetId()
	bright := func(id string) {
		t.Helper()
		if _, err := m.maps.SetMapFog(t.Context(), connect.NewRequest(&mapsv1.SetMapFogRequest{CampaignId: d.campaign, MapId: id, BaseLight: mapsv1.LightLevel_LIGHT_LEVEL_BRIGHT.Enum()})); err != nil {
			t.Fatalf("SetMapFog() error = %v", err)
		}
	}
	// The hand-made map needs no grid for the column to change.
	bright(hand)
	if _, err := d.h.pool.Exec(t.Context(), "UPDATE maps SET base_light = 'bright' WHERE id = $1", dungeon); err != nil {
		t.Fatal(err)
	}

	file, err := os.ReadFile("../../migrations/00245_generated_dungeons_dim_base_light.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(strings.TrimPrefix(string(file), "-- +goose Up"), "-- +goose Down")
	for range 2 {
		if _, err := d.h.pool.Exec(t.Context(), up); err != nil {
			t.Fatalf("the migration failed: %v", err)
		}
		light := func(id string) string {
			var l string
			if err := d.h.pool.QueryRow(t.Context(), "SELECT base_light FROM maps WHERE id = $1", id).Scan(&l); err != nil {
				t.Fatal(err)
			}
			return l
		}
		if got := light(dungeon); got != "dim" {
			t.Errorf("the dungeon's base light = %q, want dim", got)
		}
		if got := light(hand); got != "bright" {
			t.Errorf("the hand-made map's base light = %q, want bright: not a generated dungeon", got)
		}
	}
}
