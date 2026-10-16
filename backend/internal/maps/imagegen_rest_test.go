package maps

import (
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/maps/images/gen"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// wantMapImageRefusal checks a refusal of GenerateMapImage by its code and message.
func wantMapImageRefusal(t *testing.T, call string, err error, code connect.Code, message string) {
	t.Helper()
	ce, ok := errors.AsType[*connect.Error](err)
	if !ok || ce.Code() != code || !strings.Contains(ce.Message(), message) {
		t.Errorf("%s error = %v, want %v containing %q", call, err, code, message)
	}
}

// GenerateMapImage checks the request's fields in a fixed order, before it reads
// the map or an image, each with its code and message.
func TestMR039_GenerateMapImageChecksItsFields(t *testing.T) {
	t.Parallel()
	fake := &gen.Fake{}
	c := newCave(t, withFake(fake, 20))
	m := c.master
	a := m.newImage(c.campaign)
	b := m.newImage(c.campaign)
	const ghost = "00000000-0000-4000-8000-000000000001"
	const field = "npc_character_ids"

	cases := []struct {
		name   string
		kind   mapsv1.ImageGenerationKind
		mapID  string
		edit   func(*mapsv1.GenerateMapImageRequest)
		code   connect.Code
		reason string
	}{
		{"a kind that is no map's", mapsv1.ImageGenerationKind_IMAGE_GENERATION_KIND_SCENE, c.mapID, nil, connect.CodeInvalidArgument, "kind must be MAP_SCENE, ISOMETRIC or TEXTURED_MAP"},
		{"an empty key", kindMapSceneAPI, c.mapID, func(r *mapsv1.GenerateMapImageRequest) { r.IdempotencyKey = "" }, connect.CodeInvalidArgument, ""},
		{"an empty prompt", kindMapSceneAPI, c.mapID, func(r *mapsv1.GenerateMapImageRequest) { r.Prompt = "  " }, connect.CodeInvalidArgument, ""},
		{"a map id that is no UUID", kindMapSceneAPI, "nope", nil, connect.CodeNotFound, "map not found"},
		{"an image as object and character", kindMapSceneAPI, c.mapID, func(r *mapsv1.GenerateMapImageRequest) {
			r.ObjectImageIds, r.CharacterImageIds = []string{a}, []string{a}
		}, connect.CodeInvalidArgument, "an image is listed twice as a reference"},
		{"too many NPCs", kindMapSceneAPI, c.mapID, func(r *mapsv1.GenerateMapImageRequest) {
			r.NpcCharacterIds = []string{ghost, ghost, ghost, ghost, ghost}
		}, connect.CodeInvalidArgument, field + " has at most 4 characters"},
		{"an NPC that is no UUID", kindMapSceneAPI, c.mapID, func(r *mapsv1.GenerateMapImageRequest) { r.NpcCharacterIds = []string{"nope"} }, connect.CodeInvalidArgument, field + " has an ID that is not a UUID"},
		{"an NPC twice", kindMapSceneAPI, c.mapID, func(r *mapsv1.GenerateMapImageRequest) { r.NpcCharacterIds = []string{ghost, ghost} }, connect.CodeInvalidArgument, "an NPC is listed twice"},
		{"a textured map with an NPC", kindTexturedAPI, c.mapID, func(r *mapsv1.GenerateMapImageRequest) { r.NpcCharacterIds = []string{ghost} }, connect.CodeInvalidArgument, "must be empty for a textured map"},
		{"a textured map with a character image", kindTexturedAPI, c.mapID, func(r *mapsv1.GenerateMapImageRequest) { r.CharacterImageIds = []string{b} }, connect.CodeInvalidArgument, "must be empty for a textured map"},
		{"a name that is too long", kindMapSceneAPI, c.mapID, func(r *mapsv1.GenerateMapImageRequest) { r.Name = strings.Repeat("x", 300) }, connect.CodeInvalidArgument, ""},
	}
	for _, tc := range cases {
		edit := tc.edit
		if edit == nil {
			edit = func(*mapsv1.GenerateMapImageRequest) {}
		}
		_, err := m.generateFromMap(c.campaign, tc.mapID, tc.kind, "Uma sala", edit)
		wantMapImageRefusal(t, "GenerateMapImage with "+tc.name, err, tc.code, tc.reason)
	}
	// The order: the map id is judged before the images, the images before the NPCs.
	_, err := m.generateFromMap(c.campaign, "nope", kindMapSceneAPI, "Uma sala", func(r *mapsv1.GenerateMapImageRequest) { r.NpcCharacterIds = []string{"nope"} })
	wantMapImageRefusal(t, "a bad map id and a bad NPC", err, connect.CodeNotFound, "map not found")
	if used := m.imageStatus(c.campaign).GetUsedThisMonth(); used != 0 || len(fake.Calls()) != 0 {
		t.Errorf("the refusals used %d slots and made %d calls", used, len(fake.Calls()))
	}
}

// Two portraits the players must not get, one picked as a character and one as an
// object: the refusal names the field of the portrait whose NPC has the smaller ID
// (the NPCs are read in ID order, not in the order of a Go map).
func TestMR039_TwoHiddenPortraitsAreRefusedInTheOrderOfTheirNPCs(t *testing.T) {
	t.Parallel()
	c := newCave(t, withFake(&gen.Fake{}, 20))
	m := c.master
	asCharacter := m.mustUpload(c.campaign, "um.png", pngImage(t, 30, 40)).GetId()
	asObject := m.mustUpload(c.campaign, "dois.png", pngImage(t, 31, 40)).GetId()
	one := m.createNPC(c.campaign, "Emboscado Um", asCharacter)
	two := m.createNPC(c.campaign, "Emboscado Dois", asObject)
	m.placeAt(c.campaign, c.mapID, one.GetId(), grid.Square{Col: 8, Row: 8}) // a token starts hidden
	m.placeAt(c.campaign, c.mapID, two.GetId(), grid.Square{Col: 9, Row: 8})

	want := "character_image_ids"
	if two.GetId() < one.GetId() {
		want = "object_image_ids"
	}
	_, err := m.generateFromMap(c.campaign, c.mapID, kindMapSceneAPI, "Uma sala", func(r *mapsv1.GenerateMapImageRequest) {
		r.CharacterImageIds, r.ObjectImageIds = []string{asCharacter}, []string{asObject}
	})
	wantInvalidField(t, "two hidden portraits", err, want)
}
