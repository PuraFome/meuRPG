package maps

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// The master's light warning (RN-10): which player characters see nothing where they
// stand. The cave is the table (fog_test.go): Pensantus (gnome) and Sálvia (half-elf)
// have darkvision, Toren (human, 6,7) and Brisa (halfling, 4,7) do not, the base light
// is "Escuro" and the only light is the guard room's torch, behind walls.

func (u *user) inTheDarkRaw(campaignID, mapID string) (*mapsv1.ListCharactersInTheDarkResponse, error) {
	u.h.t.Helper()
	res, err := u.maps.ListCharactersInTheDark(u.h.t.Context(), connect.NewRequest(&mapsv1.ListCharactersInTheDarkRequest{CampaignId: campaignID, MapId: mapID}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// inTheDark is the names of the characters the master is warned of.
func (c *cave) inTheDark() []string {
	c.h.t.Helper()
	res, err := c.master.inTheDarkRaw(c.campaign, c.mapID)
	if err != nil {
		c.h.t.Fatalf("ListCharactersInTheDark() error = %v", err)
	}
	var names []string
	for _, ch := range res.GetCharacters() {
		names = append(names, ch.GetCharacterName())
	}
	slices.Sort(names)
	return names
}

func (c *cave) wantInTheDark(why string, want ...string) {
	c.h.t.Helper()
	slices.Sort(want)
	if got := c.inTheDark(); !slices.Equal(got, want) {
		c.h.t.Errorf("%s: in the dark = %v, want %v", why, got, want)
	}
}

func (c *cave) luz(col, row int, preset string) {
	c.h.t.Helper()
	x, y := at(col, row)
	c.master.createPoint(&mapsv1.CreateMapPointRequest{
		CampaignId: c.campaign, MapId: c.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_LIGHT, Name: "Luz", XBp: x, YBp: y,
		Light: &mapsv1.LightSpec{PresetKey: preset},
	})
}

func TestRN10_DarkWarning_DarkMapListsWhoSeesNothing(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	res, err := c.master.inTheDarkRaw(c.campaign, c.mapID)
	if err != nil {
		t.Fatalf("ListCharactersInTheDark() error = %v", err)
	}
	// Darkvision (gnome, half-elf) covers the square; the human and the halfling see nothing.
	got := map[string]*mapsv1.CharacterInTheDark{}
	for _, ch := range res.GetCharacters() {
		got[ch.GetCharacterName()] = ch
	}
	if len(got) != 2 || got["Toren"] == nil || got["Brisa"] == nil {
		t.Fatalf("in the dark = %v, want Toren and Brisa only", res.GetCharacters())
	}
	for name, want := range map[string]struct {
		id  string
		sq  grid.Square
		who string
	}{"Toren": {c.toren.GetId(), sqToren, "Toren"}, "Brisa": {c.brisa.GetId(), sqBrisa, "Brisa"}} {
		ch := got[name]
		if ch.GetCharacterId() != want.id || int(ch.GetCol()) != want.sq.Col || int(ch.GetRow()) != want.sq.Row ||
			ch.GetCause() != mapsv1.DarknessCause_DARKNESS_CAUSE_NO_LIGHT_NO_DARKVISION {
			t.Errorf("%s = %v, want id %s at %v with the cause no light, no darkvision", name, ch, want.id, want.sq)
		}
	}
}

func TestRN10_DarkWarning_NobodyInTheDarkIsAnEmptyList(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.master.carry(c.campaign, c.mapID, c.toren.GetId(), "light:torch")
	c.master.carry(c.campaign, c.mapID, c.brisa.GetId(), "light:torch")
	c.wantInTheDark("everyone has a light or darkvision")
}

func TestRN10_DarkWarning_OnlyTheDarkBaseLightWarns(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.setBaseLight(mapsv1.LightLevel_LIGHT_LEVEL_DIM)
	c.wantInTheDark("penumbra: everyone sees, with no light")
	c.setBaseLight(mapsv1.LightLevel_LIGHT_LEVEL_BRIGHT)
	c.wantInTheDark("claro")
	c.setBaseLight(mapsv1.LightLevel_LIGHT_LEVEL_DARK)
	c.wantInTheDark("escuro again", "Brisa", "Toren")
}

func TestRN10_DarkWarning_NoFogNoWarning(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	if _, err := c.master.maps.SetMapFog(t.Context(), connect.NewRequest(&mapsv1.SetMapFogRequest{CampaignId: c.campaign, MapId: c.mapID, FogEnabled: new(false)})); err != nil {
		t.Fatalf("SetMapFog(off) error = %v", err)
	}
	c.wantInTheDark("fog off: with no fog everyone sees everything")
}

func TestRN10_DarkWarning_ALightThatReachesTheSquareCounts(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	// A torch 25 ft (5 squares) from Toren and 35 ft from Brisa: bright 20 ft, dim 20 ft more.
	c.luz(11, 7, "light:torch")
	c.wantInTheDark("a torch on the map lights both")
	c = newCave(t)
	// A torch 50 ft from Toren reaches nobody here.
	c.luz(16, 7, "light:torch")
	c.wantInTheDark("a torch too far away", "Brisa", "Toren")
}

func TestRN10_DarkWarning_ACarriedLightThatGoesOutBringsTheWarningBack(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	// Toren's torch lights him and Brisa, two squares away.
	c.caio.carry(c.campaign, c.mapID, c.toren.GetId(), "light:torch")
	c.wantInTheDark("Toren carries a torch")
	c.caio.carry(c.campaign, c.mapID, c.toren.GetId(), "") // "Nenhuma": the torch went out
	c.wantInTheDark("the torch went out, at once", "Brisa", "Toren")
	c.bia.carry(c.campaign, c.mapID, c.brisa.GetId(), "light:torch")
	c.wantInTheDark("Brisa's torch lights both")
	c.bia.carry(c.campaign, c.mapID, c.brisa.GetId(), "")
	c.wantInTheDark("and went out too", "Brisa", "Toren")
}

func TestRN10_DarkWarning_TheLightOfAHiddenNPCCounts(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	// A hidden NPC with a torch stands next to Brisa and Toren: it lights them for the
	// warning although no player sees the NPC.
	npc := c.master.createCharacter(c.campaign, charactersv1.CharacterKind_CHARACTER_KIND_MINION, "Vigia Escondido")
	c.master.placeAt(c.campaign, c.mapID, npc.GetId(), grid.Square{Col: 5, Row: 7})
	c.master.carry(c.campaign, c.mapID, npc.GetId(), "light:torch")
	c.wantInTheDark("the hidden NPC's torch lights them")
	res, err := c.master.inTheDarkRaw(c.campaign, c.mapID)
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range res.GetCharacters() {
		if ch.GetCharacterId() == npc.GetId() {
			t.Errorf("an NPC is listed: %v", ch)
		}
	}
}

func TestRN10_DarkWarning_OnlyCharactersOnThisMapAreListed(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.master.removeToken(c.campaign, c.mapID, c.brisa.GetId())
	c.wantInTheDark("Brisa has no token on the map", "Toren")
	c.master.removeToken(c.campaign, c.mapID, c.toren.GetId())
	c.wantInTheDark("nobody is on the map")
	res, err := c.master.inTheDarkRaw(c.campaign, c.probeMap)
	if err != nil || len(res.GetCharacters()) != 0 {
		t.Errorf("a map with no fog: %v, %v; want an empty list", res, err)
	}
}

func TestRN10_DarkWarning_AMapHiddenFromThePlayersStillAnswersTheMaster(t *testing.T) {
	t.Parallel()
	c := newCaveWith(t, false)
	c.wantInTheDark("the master prepares the map", "Brisa", "Toren")
}

func TestRN10_DarkWarning_OnlyTheMasterReadsIt(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	for name, u := range map[string]*user{"Ana": c.ana, "Caio": c.caio, "Bia": c.bia} {
		res, err := u.inTheDarkRaw(c.campaign, c.mapID)
		if connect.CodeOf(err) != connect.CodePermissionDenied || res != nil {
			t.Errorf("%s: res = %v, err = %v; want permission_denied and nothing", name, res, err)
		}
	}
	if _, err := c.master.inTheDarkRaw(c.campaign, "not-a-map"); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a map that does not exist: %v, want not_found", err)
	}
}

// What the master's screen refetches on: every change that moves a light reaches the
// master's stream as map_changed (or token_moved for a character's own move).
func TestRN10_DarkWarning_TheMastersStreamSaysWhenALightChanges(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	w := c.master.watch(c.campaign)
	c.caio.carry(c.campaign, c.mapID, c.toren.GetId(), "light:torch")
	w.mapChanged(c.mapID) // the torch lit
	c.caio.carry(c.campaign, c.mapID, c.toren.GetId(), "")
	w.mapChanged(c.mapID) // and went out
	npc := c.master.createCharacter(c.campaign, charactersv1.CharacterKind_CHARACTER_KIND_MINION, "Vigia Escondido")
	c.master.placeAt(c.campaign, c.mapID, npc.GetId(), grid.Square{Col: 5, Row: 7})
	w.mapChanged(c.mapID) // placed (hidden)
	c.master.carry(c.campaign, c.mapID, npc.GetId(), "light:torch")
	w.mapChanged(c.mapID) // a hidden NPC's torch
	c.luz(15, 7, "light:torch")
	w.mapChanged(c.mapID) // a Luz point
	x, y := at(sqToren.Col+1, sqToren.Row)
	c.master.placeToken(c.campaign, c.mapID, c.toren.GetId(), x, y)
	w.tokenMoved(c.toren.GetId(), x, y) // a character walks into the dark
}
