package maps

import (
	"errors"
	"image"
	"image/color"
	"net/http"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/maps/images/gen"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

// Fix round 1 of slice 10.8b (the review): the textured map takes no characters, the
// portrait of a hidden NPC stays out, "Usar como imagem do mapa" keeps a fog map's image
// its own and refuses a map whose walls changed, and the limits are said in advance.

// wantInvalidField checks `invalid_argument` with the ImageGenerationInvalidField detail.
func wantInvalidField(t *testing.T, call string, err error, field string) {
	t.Helper()
	wantCode(t, call, err, connect.CodeInvalidArgument)
	ce, ok := errors.AsType[*connect.Error](err)
	if !ok {
		return
	}
	for _, d := range ce.Details() {
		if msg, derr := d.Value(); derr == nil {
			if f, ok := msg.(*mapsv1.ImageGenerationInvalidField); ok && f.GetField() == field {
				return
			}
		}
	}
	t.Errorf("%s: no field violation for %q in %v", call, field, err)
}

func grayPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	return encodePNG(t, image.NewGray(image.Rect(0, 0, w, h)))
}

// The textured map becomes the map the players read, so it takes no creature at all: the
// refusal comes before a slot is reserved, the picker is empty, and no character reference
// reaches the model.
func TestMR039_TheTexturedMapTakesNoCharacters(t *testing.T) {
	t.Parallel()
	fake := &gen.Fake{}
	c := newCave(t, withFake(fake, 20))
	m := c.master
	vigia, portrait := c.vigia(grid.Square{Col: 7, Row: 7})

	_, err := m.generateFromMap(c.campaign, c.mapID, kindTexturedAPI, "Uma caverna", func(r *mapsv1.GenerateMapImageRequest) { r.NpcCharacterIds = []string{vigia.GetId()} })
	wantInvalidField(t, "a textured map with an NPC", err, "npc_character_ids")
	_, err = m.generateFromMap(c.campaign, c.mapID, kindTexturedAPI, "Uma caverna", func(r *mapsv1.GenerateMapImageRequest) { r.CharacterImageIds = []string{portrait} })
	wantInvalidField(t, "a textured map with a character image", err, "character_image_ids")
	if used := m.imageStatus(c.campaign).GetUsedThisMonth(); used != 0 || len(fake.Calls()) != 0 {
		t.Errorf("the refusals used %d slots and made %d calls", used, len(fake.Calls()))
	}
	if ref := m.mustMapReference(c.campaign, c.mapID, kindTexturedAPI); len(ref.GetCreatures()) != 0 {
		t.Errorf("the textured map's picker = %v, want it empty", ref.GetCreatures())
	}
	// An object image is still the master's choice.
	scenery := m.newImage(c.campaign)
	wantDone(t, "a textured map", m.mustGenerateFromMap(c.campaign, c.mapID, kindTexturedAPI, "Uma caverna", func(r *mapsv1.GenerateMapImageRequest) { r.ObjectImageIds = []string{scenery} }))
	refs := fake.Calls()[0].Request.References
	for _, r := range refs {
		if r.Character {
			t.Errorf("a character reference reached the model for a textured map")
		}
	}
	if len(refs) != 1 {
		t.Errorf("references = %d, want the one object image", len(refs))
	}
}

// A portrait of an NPC whose token is on the map and whom the players do not see (hidden,
// or out of sight) cannot come in character_image_ids; any other image can.
func TestMR039_AHiddenNPCsPortraitStaysOut(t *testing.T) {
	t.Parallel()
	c := newCave(t, withFake(&gen.Fake{}, 20))
	m := c.master
	hiddenPortrait := m.mustUpload(c.campaign, "emboscado.png", pngImage(t, 30, 40)).GetId()
	farPortrait := m.mustUpload(c.campaign, "longe.png", pngImage(t, 31, 40)).GetId()
	hidden := m.createNPC(c.campaign, "Emboscado Dois", hiddenPortrait)
	m.placeAt(c.campaign, c.mapID, hidden.GetId(), grid.Square{Col: 8, Row: 8}) // a token starts hidden
	far := m.createNPC(c.campaign, "Vigia Distante", farPortrait)
	m.placeAt(c.campaign, c.mapID, far.GetId(), sqGoblin1)
	m.setTokenHidden(c.campaign, c.mapID, far.GetId(), false) // visible, but where the party does not look
	_, visiblePortrait := c.vigia(grid.Square{Col: 7, Row: 7})
	plain := m.mustUpload(c.campaign, "bardo.png", pngImage(t, 33, 40)).GetId()

	for _, kind := range []mapsv1.ImageGenerationKind{kindMapSceneAPI, kindIsometricAPI} {
		for name, img := range map[string]string{"a hidden NPC": hiddenPortrait, "an NPC out of sight": farPortrait} {
			_, err := m.generateFromMap(c.campaign, c.mapID, kind, "Uma sala", func(r *mapsv1.GenerateMapImageRequest) { r.CharacterImageIds = []string{plain, img} })
			wantInvalidField(t, "the portrait of "+name, err, "character_image_ids")
		}
	}
	if used := m.imageStatus(c.campaign).GetUsedThisMonth(); used != 0 {
		t.Errorf("the refusals used %d slots", used)
	}
	// An image that is no NPC's portrait, and the portrait of an NPC they see, are fine.
	wantDone(t, "an image that is no portrait", m.mustGenerateFromMap(c.campaign, c.mapID, kindMapSceneAPI, "Uma sala", func(r *mapsv1.GenerateMapImageRequest) { r.CharacterImageIds = []string{plain} }))
	wantDone(t, "a visible NPC's portrait", m.mustGenerateFromMap(c.campaign, c.mapID, kindIsometricAPI, "Uma sala", func(r *mapsv1.GenerateMapImageRequest) { r.CharacterImageIds = []string{visiblePortrait} }))
}

// A fog map's image is always its own: "Usar como imagem do mapa" on a picture that is also
// shown to the players, or is an NPC's portrait, gives the map a copy. Nothing leaks (RN-10).
func TestMR039_UseGivesAFogMapACopyOfAPictureUsedElsewhere(t *testing.T) {
	t.Parallel()
	c := newCave(t, withFake(&gen.Fake{}, 20))
	m, ana := c.master, c.ana
	check := func(name, texture string) {
		t.Helper()
		got := m.getMapImage(c.campaign, c.mapID)
		if got == texture || got == "" {
			t.Fatalf("%s: the fog map's image = %q, want a copy of %s", name, got, texture)
		}
		// The picture itself is still where it was used; the copy is the map's, and no
		// player can fetch it, nor does any answer of theirs carry it.
		if res := ana.get("/images/" + got); res.status != http.StatusNotFound {
			t.Errorf("%s: Ana GET of the map's image = %d, want 404", name, res.status)
		}
		for who, msg := range map[string]proto.Message{"GetMap": ana.mustGetMap(c.campaign, c.mapID), "GetMapVision": ana.mustVision(c.campaign, c.mapID)} {
			if json := asJSON(t, msg); strings.Contains(json, got) {
				t.Errorf("%s: Ana's %s has the map's image id: %s", name, who, json)
			}
		}
	}

	// Shown to the players.
	first := wantDone(t, "textured map", m.mustGenerateFromMap(c.campaign, c.mapID, kindTexturedAPI, "Uma caverna")).GetId()
	if _, err := m.play.SetShownImage(t.Context(), connect.NewRequest(&playv1.SetShownImageRequest{CampaignId: c.campaign, ImageId: first})); err != nil {
		t.Fatalf("SetShownImage() error = %v", err)
	}
	if _, err := m.useAsMapImage(c.campaign, first); err != nil {
		t.Fatalf("Use of a shown picture: %v", err)
	}
	check("shown", first)
	if res := ana.get("/images/" + first); res.status != http.StatusOK {
		t.Errorf("the shown picture: Ana GET = %d, want 200 (it is the one shown)", res.status)
	}

	// An NPC's portrait.
	second := wantDone(t, "textured map", m.mustGenerateFromMap(c.campaign, c.mapID, kindTexturedAPI, "Uma caverna")).GetId()
	m.createNPC(c.campaign, "Retrato do Mapa", second)
	if _, err := m.useAsMapImage(c.campaign, second); err != nil {
		t.Fatalf("Use of a portrait: %v", err)
	}
	check("portrait", second)
	// The players' view keeps working: Ana still reads the map's vision.
	if res := ana.mustVision(c.campaign, c.mapID); !res.GetCharacterOnMap() {
		t.Error("Ana's vision stopped working")
	}
}

// A character on the map that sees no square leaves no players' view: that is "nobody"
// (PLAYERS_SEE_NOTHING), not a picture of black.
func TestMR039_ASightThatSeesNothingIsNobody(t *testing.T) {
	t.Parallel()
	g := grid.Grid{Columns: 6, Rows: 4}
	if seen, some := seenSquares(g, nil); some || len(seen) != 24 {
		t.Errorf("seenSquares(nil) = %d squares, any %v; want 24 and none", len(seen), some)
	}
	lit, err := vision.Compile(vision.Scene{Grid: g, Walls: grid.NewLayer(g), Base: grid.Bright})
	if err != nil {
		t.Fatal(err)
	}
	if _, some := seenSquares(g, lit.See(vision.Viewer{At: grid.Square{Col: 1, Row: 1}})); !some {
		t.Error("a viewer in the light sees nothing")
	}
}

// The limit of a textured map is said in advance and enforced: the map's image over 16
// megapixels is refused before any slot, the reference says so for the screen.
func TestMR039_AMapTooLargeForATextureIsSaidInAdvance(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withFake(&gen.Fake{}, 5))
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)
	small := master.createMap(campaign, "Pequeno", master.newImage(campaign)).GetId()
	master.mustSetGrid(campaign, small, 8)
	huge := master.createMap(campaign, "Enorme", master.mustUpload(campaign, "enorme.png", grayPNG(t, 4100, 4100)).GetId()).GetId() // 16.8 megapixels
	master.mustSetGrid(campaign, huge, 20)

	if ref := master.mustMapReference(campaign, small, kindTexturedAPI); ref.GetTextureTooLarge() || ref.GetMaxTexturePixels() != 16_000_000 {
		t.Errorf("a small map: too large %v, limit %d", ref.GetTextureTooLarge(), ref.GetMaxTexturePixels())
	}
	ref := master.mustMapReference(campaign, huge, kindTexturedAPI)
	if !ref.GetTextureTooLarge() || ref.GetMaxTexturePixels() != 16_000_000 {
		t.Errorf("a 4100 x 4100 map: too large %v, limit %d", ref.GetTextureTooLarge(), ref.GetMaxTexturePixels())
	}
	_, err := master.generateFromMap(campaign, huge, kindTexturedAPI, "Um salão")
	wantGenerationBlocked(t, "a texture of a 16.8 megapixel map", err, mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_MAP_IMAGE_TOO_LARGE)
	if used := master.imageStatus(campaign).GetUsedThisMonth(); used != 0 {
		t.Errorf("the refusal used %d slots", used)
	}
}

// "Usar como imagem do mapa" is refused when the walls the drawing showed changed since: a
// wall painted, or a secret door revealed. A door opened or locked changes no wall.
func TestMR039_UseIsRefusedWhenTheWallsChanged(t *testing.T) {
	t.Parallel()
	c := newCave(t, withFake(&gen.Fake{}, 20))
	m := c.master
	changed := mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_MAP_CHANGED
	texture := func() string {
		return wantDone(t, "textured map", m.mustGenerateFromMap(c.campaign, c.mapID, kindTexturedAPI, "Uma caverna")).GetId()
	}
	wall := mapsv1.MapLayer_MAP_LAYER_WALL

	first := texture()
	m.mustPaint(c.campaign, c.mapID, wall, 1, [2]int32{10, 8})
	_, err := m.useAsMapImage(c.campaign, first)
	wantGenerationBlocked(t, "Use after a wall was painted", err, changed)

	m.mustPaint(c.campaign, c.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_SECRET), [2]int32{8, 7})
	second := texture()
	m.mustPaint(c.campaign, c.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_CLOSED), [2]int32{8, 7}) // revealed
	_, err = m.useAsMapImage(c.campaign, second)
	wantGenerationBlocked(t, "Use after a secret door was revealed", err, changed)

	// A door locked or opened is floor either way.
	third := texture()
	m.mustPaint(c.campaign, c.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_LOCKED), [2]int32{12, 7})
	m.mustPaint(c.campaign, c.mapID, doors, int32(mapsv1.DoorState_DOOR_STATE_OPEN), [2]int32{13, 7})
	if _, err := m.useAsMapImage(c.campaign, third); err != nil {
		t.Errorf("Use after a door was locked and another opened: %v", err)
	}
}

// After "Redesenhar" the dungeon has the generator's new image, so the textured map made
// before no longer fits; and when the two race, one wins and the other is refused.
func TestMR039_UseAfterAndRacingRedraw(t *testing.T) {
	t.Parallel()
	dbtest.PoolSize(t, 4)
	d := newDungeonTable(t, withFake(&gen.Fake{}, 20))
	seed, _ := testDungeonSeed(t)
	mapID := d.master.createDungeon(d.campaign, "Masmorra", testDungeonOptions(), seed).GetMap().GetId()
	changed := mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_MAP_CHANGED

	after := wantDone(t, "textured map", d.master.mustGenerateFromMap(d.campaign, mapID, kindTexturedAPI, "Uma masmorra")).GetId()
	if _, err := d.master.redraw(d.campaign, mapID); err != nil {
		t.Fatalf("redraw: %v", err)
	}
	_, err := d.master.useAsMapImage(d.campaign, after)
	wantGenerationBlocked(t, "Use after Redesenhar", err, changed)

	// Racing: a fresh texture, then both at once. At most one of them wins.
	racing := wantDone(t, "textured map", d.master.mustGenerateFromMap(d.campaign, mapID, kindTexturedAPI, "Uma masmorra")).GetId()
	var wg sync.WaitGroup
	var useErr, redrawErr error
	start := dbtest.NewBarrier(2)
	wg.Add(2)
	go func() { defer wg.Done(); start.Wait(); _, useErr = d.master.useAsMapImage(d.campaign, racing) }()
	go func() { defer wg.Done(); start.Wait(); _, redrawErr = d.master.redraw(d.campaign, mapID) }()
	wg.Wait()
	if useErr == nil && redrawErr == nil {
		t.Error("Use and Redesenhar both succeeded on the same image")
	}
	now := d.master.mustGetMap(d.campaign, mapID).GetMap().GetImage().GetId()
	if useErr == nil && now != racing {
		t.Errorf("Use succeeded but the map's image is %s", now)
	}
	if useErr != nil && connect.CodeOf(useErr) != connect.CodeFailedPrecondition {
		t.Errorf("the refused Use: %v, want failed_precondition", useErr)
	}
	if redrawErr != nil && connect.CodeOf(redrawErr) != connect.CodeFailedPrecondition {
		t.Errorf("the refused Redesenhar: %v, want failed_precondition", redrawErr)
	}
}

// RN-10 on a map without the fog: the reference and the isometric view have no hidden
// token (no disc where the master hid a goblin), and show the visible one.
func TestRN10_AMapWithoutFogDrawsNoHiddenToken(t *testing.T) {
	t.Parallel()
	fake := &gen.Fake{}
	c := newCave(t, withFake(fake, 20))
	if _, err := c.master.maps.SetMapFog(t.Context(), connect.NewRequest(&mapsv1.SetMapFogRequest{CampaignId: c.campaign, MapId: c.mapID, FogEnabled: new(false)})); err != nil {
		t.Fatal(err)
	}
	wantDone(t, "isometric view", c.master.mustGenerateFromMap(c.campaign, c.mapID, kindIsometricAPI, "Uma caverna"))
	req := fake.Calls()[0].Request
	drawing := decodePNG(t, req.Drawing.Data)
	b := drawing.Bounds()
	side := b.Dx() / 24
	at := func(sq grid.Square) color.Color { return drawing.At(sq.Col*side+side/2, sq.Row*side+side/2) }
	red := color.RGBA{R: 0xC2, G: 0x33, B: 0x1F, A: 0xFF}
	if got := at(sqGoblin2); !sameColor(got, red) {
		t.Errorf("the visible goblin's square is %v, want the NPC's disc", got)
	}
	for _, sq := range []grid.Square{{Col: 20, Row: 8}} { // the goblin the master hid
		if got := at(sq); sameColor(got, red) || sameColor(got, color.RGBA{R: 0x2F, G: 0x6F, B: 0xDE, A: 0xFF}) {
			t.Errorf("the hidden goblin's square %v has a disc: %v", sq, got)
		}
	}
	ref := c.master.mustMapReference(c.campaign, c.mapID, kindMapSceneAPI)
	for _, cr := range ref.GetCreatures() {
		if cr.GetCharacterId() == c.hiddenGoblin.GetId() {
			t.Error("the hidden goblin is offered")
		}
	}
}

// Use is idempotent: a repeat of a Use that happened, while the image it set is still the
// map's, answers the same with no new copy; once the map's image changed it is MAP_CHANGED.
func TestMR039_UseIsIdempotent(t *testing.T) {
	t.Parallel()
	c := newCave(t, withFake(&gen.Fake{}, 20))
	m := c.master
	copies := func() int {
		var n int
		if err := c.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM gallery_images WHERE campaign_id = $1 AND name LIKE '%(névoa)'`, c.campaign).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// A fog map that needs a copy (the picture is shown): twice, the same map, one copy.
	first := wantDone(t, "textured map", m.mustGenerateFromMap(c.campaign, c.mapID, kindTexturedAPI, "Uma caverna")).GetId()
	if _, err := m.play.SetShownImage(t.Context(), connect.NewRequest(&playv1.SetShownImageRequest{CampaignId: c.campaign, ImageId: first})); err != nil {
		t.Fatal(err)
	}
	a, err := m.useAsMapImage(c.campaign, first)
	if err != nil {
		t.Fatal(err)
	}
	before := copies()
	b, err := m.useAsMapImage(c.campaign, first)
	if err != nil {
		t.Fatalf("the second Use: %v, want the same success", err)
	}
	if a.GetMap().GetImage().GetId() == first || b.GetMap().GetImage().GetId() != a.GetMap().GetImage().GetId() || b.GetMap().GetRevision() != a.GetMap().GetRevision() {
		t.Errorf("Use twice: image %s then %s (revision %d then %d), want the same copy",
			a.GetMap().GetImage().GetId(), b.GetMap().GetImage().GetId(), a.GetMap().GetRevision(), b.GetMap().GetRevision())
	}
	if before != 1 || copies() != 1 {
		t.Errorf("copies after the first Use: %d, after the second: %d, want 1 and 1", before, copies())
	}

	// A plain map (no fog, nothing shown): twice.
	plain := m.createMap(c.campaign, "Corredor", m.mustUpload(c.campaign, "corredor.png", patternImage(t, 200, 100)).GetId()).GetId()
	m.mustSetGrid(c.campaign, plain, 20)
	t2 := wantDone(t, "textured map", m.mustGenerateFromMap(c.campaign, plain, kindTexturedAPI, "Um corredor")).GetId()
	p1, err := m.useAsMapImage(c.campaign, t2)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := m.useAsMapImage(c.campaign, t2)
	if err != nil || p2.GetMap().GetImage().GetId() != t2 || p2.GetMap().GetRevision() != p1.GetMap().GetRevision() {
		t.Errorf("Use twice on a plain map: %v, %v", p2.GetMap(), err)
	}

	// Use, then another image on the map, then Use again: the map changed.
	cur := m.mustGetMap(c.campaign, plain).GetMap()
	other := m.mustUpload(c.campaign, "outra.png", patternImage(t, 200, 100)).GetId()
	if _, err := m.maps.UpdateMap(t.Context(), connect.NewRequest(&mapsv1.UpdateMapRequest{CampaignId: c.campaign, MapId: plain, Revision: cur.GetRevision(), ImageId: new(other)})); err != nil {
		t.Fatal(err)
	}
	_, err = m.useAsMapImage(c.campaign, t2)
	wantGenerationBlocked(t, "Use after another image", err, mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_MAP_CHANGED)
}
