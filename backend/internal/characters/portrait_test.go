package characters

import (
	"context"
	"strings"
	"testing"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

// MR-031 (D7, question 62): an NPC's sheet carries a portrait, an image of the
// campaign's gallery. The gallery is package maps's, so the tests give the
// service a stand-in that knows a few images by campaign; the real one is
// exercised with the stage, in package maps.

// fakeGallery says which images each campaign has.
type fakeGallery map[string][]string

func (g fakeGallery) PortraitImage(_ context.Context, campaignID, imageID string) (string, bool, error) {
	for _, id := range g[campaignID] {
		if id == imageID {
			return imageID, true, nil
		}
	}
	return "", false, nil
}

// withPortrait returns a copy of the sheet with the portrait set, full or basic.
func withPortrait(sheet *charactersv1.CharacterSheet, imageID string) *charactersv1.CharacterSheet {
	out := proto.CloneOf(sheet)
	if f := out.GetFull(); f != nil {
		f.PortraitImageId = imageID
	} else {
		out.GetBasic().PortraitImageId = imageID
	}
	return out
}

func TestMR031_AnNPCKeepsAPortraitFromTheCampaignsGallery(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre := h.newUser("Mestre")
	campaign, other := h.newCampaign(mestre, "Mirathel"), h.newCampaign(mestre, "Outra")
	mine, theirs := uuid.New().String(), uuid.New().String()
	h.svc.SetGallery(fakeGallery{campaign: {mine}, other: {theirs}})

	// The portrait is kept on the sheet of an NPC with a full sheet and with a
	// basic one, and read back as it was saved.
	for name, c := range map[string]struct {
		kind  charactersv1.CharacterKind
		sheet *charactersv1.CharacterSheet
	}{
		"enemy":  {charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, enemySheet()},
		"boss":   {charactersv1.CharacterKind_CHARACTER_KIND_BOSS, enemySheet()},
		"minion": {charactersv1.CharacterKind_CHARACTER_KIND_MINION, basicSheet()},
		"story":  {charactersv1.CharacterKind_CHARACTER_KIND_STORY, basicSheet()},
	} {
		created := mestre.create(t, campaign, c.kind, name, withPortrait(c.sheet, mine))
		got := portraitOf(mestre.get(t, campaign, created.GetId()).GetSheet())
		if got != mine {
			t.Errorf("%s: the portrait read back is %q, want %q", name, got, mine)
		}
	}

	// It can be changed and removed on an update.
	npc := mestre.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_MINION, "Goblin", withPortrait(basicSheet(), mine))
	cleared, err := mestre.update(t, npc, "Goblin", withPortrait(npc.GetSheet(), ""))
	if err != nil || portraitOf(cleared.GetSheet()) != "" {
		t.Errorf("UpdateCharacter(remove the portrait) = %v, %v, want no portrait", cleared, err)
	}
}

func TestMR031_APortraitMustBeAnImageOfTheCampaign(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre, jogador := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign, other := h.newCampaign(mestre, "Mirathel", jogador), h.newCampaign(mestre, "Outra")
	mine, theirs := uuid.New().String(), uuid.New().String()
	h.svc.SetGallery(fakeGallery{campaign: {mine}, other: {theirs}})
	minion, player := charactersv1.CharacterKind_CHARACTER_KIND_MINION, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER

	for _, c := range []struct {
		name   string
		kind   charactersv1.CharacterKind
		sheet  *charactersv1.CharacterSheet
		caller *user
		field  string
	}{
		{"an image of another campaign", minion, withPortrait(basicSheet(), theirs), mestre, "sheet.basic.portrait_image_id"},
		{"an image that does not exist", minion, withPortrait(basicSheet(), uuid.New().String()), mestre, "sheet.basic.portrait_image_id"},
		{"something that is not an ID", minion, withPortrait(basicSheet(), "retrato.png"), mestre, "sheet.basic.portrait_image_id"},
		{"another campaign's image on a full sheet", charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, withPortrait(enemySheet(), theirs), mestre, "sheet.full.portrait_image_id"},
		{"a player's character", player, withPortrait(pensantusSheet(), mine), jogador, "sheet.full.portrait_image_id"},
	} {
		_, err := c.caller.api.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{CampaignId: campaign, Kind: c.kind, Name: "Teste", Sheet: c.sheet}))
		wantCode(t, "CreateCharacter("+c.name+")", err, connect.CodeInvalidArgument)
		if !strings.Contains(err.Error(), c.field) {
			t.Errorf("CreateCharacter(%s) error = %v, want it to name %s", c.name, err, c.field)
		}
		if strings.Contains(err.Error(), theirs) {
			t.Errorf("CreateCharacter(%s) error = %v: it names the other campaign's image", c.name, err)
		}
	}

	// The same on an update: an NPC cannot take another campaign's image, and
	// a player's character cannot get one, from the master either.
	goblin := mestre.create(t, campaign, minion, "Goblin", basicSheet())
	_, err := mestre.update(t, goblin, "Goblin", withPortrait(goblin.GetSheet(), theirs))
	wantCode(t, "UpdateCharacter(another campaign's image)", err, connect.CodeInvalidArgument)
	pens := jogador.createPensantus(t, campaign)
	_, err = mestre.update(t, pens, "Pensantus", withPortrait(pens.GetSheet(), mine))
	wantCode(t, "UpdateCharacter(a portrait on a player's character, by the master)", err, connect.CodeInvalidArgument)
	_, err = jogador.update(t, pens, "Pensantus", withPortrait(pens.GetSheet(), mine))
	wantCode(t, "UpdateCharacter(a portrait on a player's character, by the player)", err, connect.CodeInvalidArgument)
}

// MR-031: the master's character list carries each NPC's portrait as a URL, so
// "Pôr em cena" needs no read of every sheet; a player's character has none,
// and an NPC without a portrait has an empty URL.
func TestMR031_TheMastersListCarriesThePortraitURL(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mestre, jogador := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(mestre, "Mirathel", jogador)
	mine := uuid.New().String()
	h.svc.SetGallery(fakeGallery{campaign: {mine}})
	mestre.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_STORY, "Mira", withPortrait(basicSheet(), mine))
	mestre.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_BOSS, "Ivo", withPortrait(enemySheet(), mine))
	mestre.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_MINION, "Goblin", basicSheet())
	jogador.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus", pensantusSheet())

	got := map[string]string{}
	for _, c := range mestre.list(t, campaign) {
		got[c.GetName()] = c.GetPortraitUrl()
	}
	want := map[string]string{"Mira": "/images/" + mine, "Ivo": "/images/" + mine, "Goblin": "", "Pensantus": ""}
	for name, url := range want {
		if got[name] != url {
			t.Errorf("the master's list: %s has portrait_url %q, want %q", name, got[name], url)
		}
	}
	for _, c := range jogador.list(t, campaign) {
		if c.GetPortraitUrl() != "" {
			t.Errorf("the player's list carries a portrait_url: %v", c)
		}
	}
}
