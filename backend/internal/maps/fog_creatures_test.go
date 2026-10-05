package maps

import (
	"slices"
	"strings"
	"testing"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// What a druid in Wild Shape sees, the familiar's eyes and the tokens of a character's
// creatures on a map with the fog of war on (MR-036, MR-037, Etapa 9, slice 9.10).
// The table is the cave of the designs (fog_test.go) with two more players: Folha, a
// level 5 druid (half-elf: darkvision 18 m), and Mago, a level 5 wizard with a
// familiar owl (Nanquim). Every response is read as the app's JSON where it matters.

// sheetOf is a full sheet of a class at a level, with spells.
func (u *user) levelFive(campaignID, name, race, class string, known []string) *charactersv1.Character {
	u.h.t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 12, Constitution: 14, Intelligence: 16, Wisdom: 16, Charisma: 8},
		RaceKey:    race, Classes: []*charactersv1.ClassLevel{{ClassKey: class, Level: 5}},
		KnownSpellKeys: known, PreparedSpellKeys: known,
	}}}
	res, err := u.characters.CreateCharacter(u.h.t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: name, Sheet: sheet,
	}))
	if err != nil {
		u.h.t.Fatalf("CreateCharacter(%s) error = %v", name, err)
	}
	return res.Msg.GetCharacter()
}

// seenNow counts the squares a view sees now: a wall, grey, dim or bright light.
func seenNow(t *testing.T, res *mapsv1.GetMapVisionResponse) int {
	t.Helper()
	n := 0
	for _, code := range codes(t, res) {
		if code >= 1 && code <= 4 {
			n++
		}
	}
	return n
}

// remembered counts the squares a view remembers and no longer sees.
func remembered(t *testing.T, res *mapsv1.GetMapVisionResponse) int {
	t.Helper()
	return strings.Count(string(codes(t, res)), string([]byte{5}))
}

// stateAt is a square's code in a view.
func stateAt(t *testing.T, res *mapsv1.GetMapVisionResponse, col, row int) byte {
	t.Helper()
	return codes(t, res)[row*int(res.GetGridColumns())+col]
}

// TestMR036_AWolfSeesNothingInTheDark: Folha, a half-elf druid, sees 18 m in the dark
// with her darkvision; as a wolf she has none (the beast's senses replace hers), so she
// sees nothing new in a dark room and what she saw stays remembered; back in her own
// shape she sees as before. The other players' views do not change.
func TestMR036_AWolfSeesNothingInTheDark(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	eve := c.h.newUser("Eva")
	c.h.join(c.master, c.campaign, false, eve)
	folha := eve.levelFive(c.campaign, "Folha", "race:half-elf", "class:druid", nil)
	room := grid.Square{Col: 8, Row: 12} // the dark room south of the cave: no light, no wall between
	c.master.placeAt(c.campaign, c.mapID, folha.GetId(), room)

	human := eve.mustVision(c.campaign, c.mapID)
	if n := seenNow(t, human); n < 30 {
		t.Fatalf("Folha with darkvision sees %d squares, want her 18 m of the dark room (a circle of 12 squares)\n%s", n, strings.Join(picture(t, human, room), "\n"))
	}
	// Pensantus's view, for the comparison at the end.
	pens := c.ana.mustVision(c.campaign, c.mapID)

	wolf, err := eve.play.AssumeWildShape(t.Context(), connect.NewRequest(&playv1.AssumeWildShapeRequest{
		CampaignId: c.campaign, CharacterId: folha.GetId(), BeastKey: "monster:wolf", IdempotencyKey: rand32(),
	}))
	if err != nil {
		t.Fatalf("AssumeWildShape() error = %v", err)
	}
	if wolf.Msg.GetVitals().GetWildShape().GetBeastKey() != "monster:wolf" {
		t.Fatalf("form = %v, want a wolf", wolf.Msg.GetVitals().GetWildShape())
	}
	asWolf := eve.mustVision(c.campaign, c.mapID)
	// Only the square she stands on: the dark is black to a creature with no darkvision.
	if n := seenNow(t, asWolf); n != 1 {
		t.Errorf("the wolf sees %d squares in the dark, want only her own: no darkvision\n%s", n, strings.Join(picture(t, asWolf, room), "\n"))
	}
	// What she saw as a druid stays remembered (it only ever grows), but for her own square.
	if got, want := remembered(t, asWolf), seenNow(t, human)-1; got < want {
		t.Errorf("remembered = %d squares, want at least the %d she saw as a druid", got, want)
	}
	// Nobody else's view moved because of it.
	if got := c.ana.mustVision(c.campaign, c.mapID); string(codes(t, got)) != string(codes(t, pens)) {
		t.Errorf("Pensantus's view changed when Folha became a wolf")
	}
	// Her party token stays on the map for everyone.
	if !slices.Contains(tokenNames(c.ana.mustGetMap(c.campaign, c.mapID)), "Folha") {
		t.Errorf("Folha's token is hidden from the party: %v", tokenNames(c.ana.mustGetMap(c.campaign, c.mapID)))
	}

	// Back in her own shape: the same view as before.
	if _, err := eve.play.LeaveWildShape(t.Context(), connect.NewRequest(&playv1.LeaveWildShapeRequest{CampaignId: c.campaign, CharacterId: folha.GetId(), IdempotencyKey: rand32()})); err != nil {
		t.Fatalf("LeaveWildShape() error = %v", err)
	}
	if got := eve.mustVision(c.campaign, c.mapID); seenNow(t, got) != seenNow(t, human) {
		t.Errorf("back in her own shape she sees %d squares, want %d", seenNow(t, got), seenNow(t, human))
	}
}

// rand32 is a fresh idempotency key.
func rand32() string { return uuid.New().String() }

func newUUID() string { return uuid.New().String() }

// TestMR036_TheFamiliarsEyesGiveItsView: Mago looks through Nanquim's eyes: the owl is in
// the lit guard room 30 m or less from him, and the player then sees what the owl sees
// there, with the owl's senses; the other players see nothing of it; at more than 30 m
// the owl's view goes away; stopping brings his own view back. The owl's token is a
// party token the others see wherever it is.
func TestMR036_TheFamiliarsEyesGiveItsView(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	gus := c.h.newUser("Gustavo")
	c.h.join(c.master, c.campaign, false, gus)
	mago := gus.levelFive(c.campaign, "Mago", "race:human", "class:wizard", []string{"spell:find-familiar"})
	stand := grid.Square{Col: 6, Row: 14} // the dark room's far corner, off every light
	c.master.placeAt(c.campaign, c.mapID, mago.GetId(), stand)

	if _, err := gus.play.CastSummon(t.Context(), connect.NewRequest(&playv1.CastSummonRequest{
		CampaignId: c.campaign, CharacterId: mago.GetId(), SpellKey: "spell:find-familiar", Ritual: true, IdempotencyKey: rand32(),
		Summon: &playv1.SummonChoice{CreatureKeys: []string{"monster:owl"}, Names: []string{"Nanquim"}},
	})); err != nil {
		t.Fatalf("CastSummon() error = %v", err)
	}
	list, err := gus.characters.ListCharacterCreatures(t.Context(), connect.NewRequest(&charactersv1.ListCharacterCreaturesRequest{CampaignId: c.campaign, CharacterId: mago.GetId()}))
	if err != nil || len(list.Msg.GetCreatures()) != 1 {
		t.Fatalf("ListCharacterCreatures() = %v, %v, want the owl", list, err)
	}
	owl := list.Msg.GetCreatures()[0].GetId()

	// The master places the owl's token; nobody else may.
	place := func(u *user, req *mapsv1.PlaceMapTokenRequest) (*mapsv1.PlaceMapTokenResponse, error) {
		res, err := u.maps.PlaceMapToken(t.Context(), connect.NewRequest(req))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	x, y := at(18, 6) // the guard room, in the torch's light
	_, err = place(gus, &mapsv1.PlaceMapTokenRequest{CampaignId: c.campaign, MapId: c.mapID, CreatureId: owl, XBp: x, YBp: y})
	wantCode(t, "PlaceMapToken of a creature by a player", err, connect.CodePermissionDenied)
	_, err = place(c.master, &mapsv1.PlaceMapTokenRequest{CampaignId: c.campaign, MapId: c.mapID, CreatureId: owl, CharacterId: mago.GetId(), XBp: x, YBp: y})
	wantCode(t, "PlaceMapToken with a character and a creature", err, connect.CodeInvalidArgument)
	_, err = place(c.master, &mapsv1.PlaceMapTokenRequest{CampaignId: c.campaign, MapId: c.mapID, XBp: x, YBp: y})
	wantCode(t, "PlaceMapToken with neither", err, connect.CodeInvalidArgument)
	_, err = place(c.master, &mapsv1.PlaceMapTokenRequest{CampaignId: c.campaign, MapId: c.mapID, CreatureId: newUUID(), XBp: x, YBp: y})
	wantCode(t, "PlaceMapToken of a creature that does not exist", err, connect.CodeNotFound)
	placed, err := place(c.master, &mapsv1.PlaceMapTokenRequest{CampaignId: c.campaign, MapId: c.mapID, CreatureId: owl, XBp: x, YBp: y})
	if err != nil {
		t.Fatalf("PlaceMapToken(creature) error = %v", err)
	}
	if tok := placed.GetToken(); tok.GetCreatureId() != owl || tok.GetCharacterId() != mago.GetId() || tok.GetName() != "Nanquim" || tok.GetHidden() {
		t.Errorf("the owl's token = %v, want the creature, its owner's character, its name, not hidden", tok)
	}

	// A creature's token is a party token: every player gets it, in the dark guard room
	// too; the hidden NPC next to it stays hidden.
	for name, u := range map[string]*user{"Ana": c.ana, "Caio": c.caio, "Mago's player": gus} {
		got := u.mustGetMap(c.campaign, c.mapID)
		var found *mapsv1.MapToken
		for _, tok := range got.GetTokens() {
			if tok.GetCreatureId() == owl {
				found = tok
			}
		}
		if found == nil || found.GetHidden() || found.GetName() != "Nanquim" {
			t.Errorf("%s's tokens = %v, want the owl's party token", name, tokenNames(got))
		}
		if slices.Contains(tokenNames(got), "Goblin Emboscado") {
			t.Errorf("%s received the hidden goblin", name)
		}
	}
	if got := c.master.mustGetMap(c.campaign, c.mapID); len(got.GetTokens()) != 10 { // 8 + Mago + the owl
		t.Errorf("the master's tokens = %d, want 10", len(got.GetTokens()))
	}

	// His own eyes: the dark corner.
	own := gus.mustVision(c.campaign, c.mapID)
	guard := func(res *mapsv1.GetMapVisionResponse) byte { return stateAt(t, res, 19, 3) } // beside the torch
	if guard(own) != 0 {
		t.Fatalf("Mago sees the guard room from the dark corner: state %d", guard(own))
	}
	anaBefore := c.ana.mustVision(c.campaign, c.mapID)

	// He looks through the owl's eyes: the lit guard room is his.
	start := func() (*playv1.StartFamiliarSightResponse, error) {
		res, err := gus.play.StartFamiliarSight(t.Context(), connect.NewRequest(&playv1.StartFamiliarSightRequest{CampaignId: c.campaign, CharacterId: mago.GetId(), IdempotencyKey: rand32()}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	if _, err := start(); err != nil {
		t.Fatalf("StartFamiliarSight() error = %v", err)
	}
	through := gus.mustVision(c.campaign, c.mapID)
	if guard(through) < 3 { // dim or bright: lit by the torch
		t.Errorf("through the owl's eyes the torch's square is state %d, want lit\n%s", guard(through), strings.Join(picture(t, through, stand), "\n"))
	}
	// Blind and deaf with regard to his own senses (SRD): he no longer sees the dark corner
	// he stands in, only what the owl sees.
	if nowSeen(stateAt(t, through, stand.Col, stand.Row)) {
		t.Errorf("through the owl Mago still sees his own square (state %d), want only the owl's view", stateAt(t, through, stand.Col, stand.Row))
	}
	// Nobody else sees through it.
	if got := c.ana.mustVision(c.campaign, c.mapID); string(codes(t, got)) != string(codes(t, anaBefore)) {
		t.Errorf("Pensantus's view changed when Mago looked through the owl")
	}
	if g := c.caio.mustVision(c.campaign, c.mapID); stateAt(t, g, 19, 3) != 0 {
		t.Errorf("Toren sees the guard room through Mago's owl")
	}

	// The owl is a creature of the character: its token follows the master's hand. At more
	// than 30 m from Mago (here 17 squares across and 12 down) the owl's view is gone,
	// but what was seen is remembered.
	far, farY := at(23, 2)
	if _, err := place(c.master, &mapsv1.PlaceMapTokenRequest{CampaignId: c.campaign, MapId: c.mapID, CreatureId: owl, XBp: far, YBp: farY}); err != nil {
		t.Fatalf("PlaceMapToken(creature) error = %v", err)
	}
	beyond := gus.mustVision(c.campaign, c.mapID)
	if guard(beyond) != 5 {
		t.Errorf("with the owl beyond 30 m the torch's square is state %d, want 5 (remembered)", guard(beyond))
	}
	// Back within range, it sees again; stopping gives his own eyes back.
	if _, err := place(c.master, &mapsv1.PlaceMapTokenRequest{CampaignId: c.campaign, MapId: c.mapID, CreatureId: owl, XBp: x, YBp: y}); err != nil {
		t.Fatalf("PlaceMapToken(creature) error = %v", err)
	}
	if guard(gus.mustVision(c.campaign, c.mapID)) < 3 {
		t.Errorf("the owl in range again gives no view")
	}
	// With "Visão do grupo" the other characters' views still count for the party, but not
	// his own eyes. (Last: the group's view adds to everyone's memory.)
	if _, err := c.master.maps.SetMapFog(t.Context(), connect.NewRequest(&mapsv1.SetMapFogRequest{CampaignId: c.campaign, MapId: c.mapID, GroupVision: proto.Bool(true)})); err != nil {
		t.Fatalf("SetMapFog(group) error = %v", err)
	}
	group := gus.mustVision(c.campaign, c.mapID)
	if !nowSeen(stateAt(t, group, sqToren.Col, sqToren.Row)) || nowSeen(stateAt(t, group, stand.Col, stand.Row)) {
		t.Errorf("with the group's vision Mago sees Toren's square: %d, his own: %d; want the party's view and not his own eyes",
			stateAt(t, group, sqToren.Col, sqToren.Row), stateAt(t, group, stand.Col, stand.Row))
	}
	if _, err := gus.play.StopFamiliarSight(t.Context(), connect.NewRequest(&playv1.StopFamiliarSightRequest{CampaignId: c.campaign, CharacterId: mago.GetId(), IdempotencyKey: rand32()})); err != nil {
		t.Fatalf("StopFamiliarSight() error = %v", err)
	}
	if guard(gus.mustVision(c.campaign, c.mapID)) != 5 {
		t.Errorf("after stopping the torch's square is state %d, want remembered", guard(gus.mustVision(c.campaign, c.mapID)))
	}

	// Taking the token off the map: the player's own view, and the party token goes.
	if _, err := c.master.maps.RemoveMapToken(t.Context(), connect.NewRequest(&mapsv1.RemoveMapTokenRequest{CampaignId: c.campaign, MapId: c.mapID, CreatureId: owl})); err != nil {
		t.Fatalf("RemoveMapToken(creature) error = %v", err)
	}
	for _, tok := range c.ana.mustGetMap(c.campaign, c.mapID).GetTokens() {
		if tok.GetCreatureId() != "" {
			t.Errorf("the owl's token is still on the map: %v", tok)
		}
	}
	_, err = c.master.maps.RemoveMapToken(t.Context(), connect.NewRequest(&mapsv1.RemoveMapTokenRequest{CampaignId: c.campaign, MapId: c.mapID, CreatureId: owl}))
	wantCode(t, "RemoveMapToken twice", err, connect.CodeNotFound)
}

// TestMR037_ACreatureOfAnotherCharacterDoesNotSeeForItsOwner: only the familiar sees for
// its player, and only while the player looks through its eyes: a gifted wolf on the
// map lights nothing for its owner, but its token is the party's.
func TestMR037_ACreatureDoesNotSeeForItsOwnerUnlessItIsTheFamiliarsEyes(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	res, err := c.master.characters.GiveCreature(t.Context(), connect.NewRequest(&charactersv1.GiveCreatureRequest{
		CampaignId: c.campaign, CharacterId: c.salvia.GetId(), MonsterKey: "monster:wolf", Name: "Presa",
	}))
	if err != nil {
		t.Fatalf("GiveCreature() error = %v", err)
	}
	wolf := res.Msg.GetCreature().GetId()
	x, y := at(18, 6)
	if _, err := c.master.maps.PlaceMapToken(t.Context(), connect.NewRequest(&mapsv1.PlaceMapTokenRequest{CampaignId: c.campaign, MapId: c.mapID, CreatureId: wolf, XBp: x, YBp: y})); err != nil {
		t.Fatalf("PlaceMapToken(creature) error = %v", err)
	}
	// Sálvia's player does not see the lit guard room because her wolf stands in it.
	if got := c.dani.mustVision(c.campaign, c.mapID); stateAt(t, got, 19, 3) != 0 {
		t.Errorf("Dani sees the guard room through her gifted wolf: state %d", stateAt(t, got, 19, 3))
	}
	// But the wolf's token is a party token: Ana sees where it stands, never hidden.
	var found bool
	for _, tok := range c.ana.mustGetMap(c.campaign, c.mapID).GetTokens() {
		found = found || (tok.GetCreatureId() == wolf && !tok.GetHidden() && tok.GetCharacterId() == c.salvia.GetId())
	}
	if !found {
		t.Errorf("the wolf's token is not a party token for Ana")
	}
	// A familiar that is not one: the eyes are refused (no familiar), whatever the creature.
	_, err = c.dani.play.StartFamiliarSight(t.Context(), connect.NewRequest(&playv1.StartFamiliarSightRequest{CampaignId: c.campaign, CharacterId: c.salvia.GetId(), IdempotencyKey: rand32()}))
	wantCode(t, "StartFamiliarSight with a gifted wolf", err, connect.CodeFailedPrecondition)
}

// TestMR036_TheWolfFallingTellsThePlayerTheirViewChanged: when the beast falls (here the
// master takes it to 0) the druid's own senses are back, and the player gets
// `vision_changed` for the map.
func TestMR036_TheWolfFallingTellsThePlayerTheirViewChanged(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	eve := c.h.newUser("Eva")
	c.h.join(c.master, c.campaign, false, eve)
	folha := eve.levelFive(c.campaign, "Folha", "race:half-elf", "class:druid", nil)
	c.master.placeAt(c.campaign, c.mapID, folha.GetId(), grid.Square{Col: 8, Row: 12})
	if _, err := eve.play.AssumeWildShape(t.Context(), connect.NewRequest(&playv1.AssumeWildShapeRequest{CampaignId: c.campaign, CharacterId: folha.GetId(), BeastKey: "monster:wolf", IdempotencyKey: rand32()})); err != nil {
		t.Fatalf("AssumeWildShape() error = %v", err)
	}
	w := eve.watch(c.campaign)
	drain := func() []*playv1.WatchGameSessionResponse {
		c.master.setMapRevealed(c.campaign, c.probeMap, !c.master.mustGetMap(c.campaign, c.probeMap).GetMap().GetRevealed())
		return w.drain(c.probeMap)
	}
	drain()
	zero := int32(0)
	if _, err := c.master.play.AdjustCharacterVitals(t.Context(), connect.NewRequest(&playv1.AdjustCharacterVitalsRequest{
		CampaignId: c.campaign, CharacterId: folha.GetId(), IdempotencyKey: rand32(), WildShapeHitPointsCurrent: &zero,
	})); err != nil {
		t.Fatalf("AdjustCharacterVitals() error = %v", err)
	}
	var vision bool
	for _, ev := range drain() {
		vision = vision || ev.GetVisionChanged().GetMapId() == c.mapID
	}
	if !vision {
		t.Errorf("the player was not told their view changed when the wolf fell")
	}
	if n := seenNow(t, eve.mustVision(c.campaign, c.mapID)); n < 30 {
		t.Errorf("back in her own shape she sees %d squares, want her darkvision's", n)
	}
}

// nowSeen says a square's state is "seen now": a wall, grey, dim or bright light.
func nowSeen(code byte) bool { return code >= 1 && code <= 4 }
