package maps

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// Tokens outside combat (plan B, B2) on the cave of the designs (fog_test.go): the master moves
// tokens alone or together and puts the party on the map, a player moves their own token when the
// master allows it, and the "can't pass" layer stops them all. Row 7 is the cave's corridor.

func (u *user) moveTokens(campaignID, mapID string, to grid.Square, ids ...string) (*mapsv1.MoveMapTokensResponse, error) {
	x, y := at(to.Col, to.Row)
	res, err := u.maps.MoveMapTokens(u.h.t.Context(), connect.NewRequest(&mapsv1.MoveMapTokensRequest{
		CampaignId: campaignID, MapId: mapID, CharacterIds: ids, XBp: x, YBp: y,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (u *user) mustMoveTokens(campaignID, mapID string, to grid.Square, ids ...string) *mapsv1.MoveMapTokensResponse {
	u.h.t.Helper()
	res, err := u.moveTokens(campaignID, mapID, to, ids...)
	if err != nil {
		u.h.t.Fatalf("MoveMapTokens(%v) error = %v", to, err)
	}
	return res
}

func (u *user) setPlayersMove(campaignID, mapID string, on bool) (*mapsv1.Map, error) {
	res, err := u.maps.SetMapPlayersMove(u.h.t.Context(), connect.NewRequest(&mapsv1.SetMapPlayersMoveRequest{CampaignId: campaignID, MapId: mapID, Enabled: on}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetMap(), nil
}

// squareOfToken is the square a token stands on.
func squareOfToken(t *mapsv1.MapToken) grid.Square {
	return caveGrid.SquareOf(int(t.GetXBp()), int(t.GetYBp()))
}

func TestB2_TheMasterMovesATokenAndItStopsBeforeACantPassSquare(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.master.mustPaint(c.campaign, c.mapID, mapsv1.MapLayer_MAP_LAYER_NO_PASS, 1, [2]int32{10, 7})

	res := c.master.mustMoveTokens(c.campaign, c.mapID, grid.Square{Col: 15, Row: 7}, c.toren.GetId())
	mv := res.GetMoves()[0]
	if !res.GetMoved() || squareOfToken(mv.GetToken()) != (grid.Square{Col: 9, Row: 7}) || mv.GetSquaresAsked() != 9 || mv.GetSquaresMoved() != 3 {
		t.Fatalf("move = %v, want Toren at (9,7) after 3 of 9 squares", res)
	}
	if mv.GetStoppedBy() != mapsv1.MapMoveStop_MAP_MOVE_STOP_NO_PASS {
		t.Errorf("stopped by %v, want the can't-pass square", mv.GetStoppedBy())
	}
	// A wall: up from (6,7) the cave closes at row 5.
	res = c.master.mustMoveTokens(c.campaign, c.mapID, grid.Square{Col: 3, Row: 4}, c.brisa.GetId())
	if got := res.GetMoves()[0]; got.GetStoppedBy() != mapsv1.MapMoveStop_MAP_MOVE_STOP_WALL || squareOfToken(got.GetToken()).Row != 6 {
		t.Errorf("Brisa: stopped by %v at %v, want a wall on row 5", got.GetStoppedBy(), squareOfToken(got.GetToken()))
	}
}

func TestB2_AGroupMovesTogetherOrNotAtAll(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	m := c.master
	// Toren (6,7) leads two squares to the right; Brisa (4,7) keeps her place beside him.
	res := m.mustMoveTokens(c.campaign, c.mapID, grid.Square{Col: 8, Row: 7}, c.toren.GetId(), c.brisa.GetId())
	if !res.GetMoved() || squareOfToken(res.GetMoves()[0].GetToken()) != (grid.Square{Col: 8, Row: 7}) || squareOfToken(res.GetMoves()[1].GetToken()) != (grid.Square{Col: 6, Row: 7}) {
		t.Fatalf("group move = %v, want Toren (8,7) and Brisa (6,7)", res)
	}
	// A can't-pass square where Brisa would arrive: nobody moves, and the answer names her.
	m.mustPaint(c.campaign, c.mapID, mapsv1.MapLayer_MAP_LAYER_NO_PASS, 1, [2]int32{8, 7})
	res = m.mustMoveTokens(c.campaign, c.mapID, grid.Square{Col: 10, Row: 7}, c.toren.GetId(), c.brisa.GetId())
	if res.GetMoved() || res.GetBlockedCharacterId() != c.brisa.GetId() {
		t.Fatalf("refused group = %v, want nobody moved and Brisa named", res)
	}
	for i, mv := range res.GetMoves() {
		if want := []grid.Square{{Col: 8, Row: 7}, {Col: 6, Row: 7}}[i]; squareOfToken(mv.GetToken()) != want {
			t.Errorf("token %d moved to %v in a refused group", i, squareOfToken(mv.GetToken()))
		}
	}
}

func TestB2_PlayersMoveTheirOwnTokensOnlyWhenTheMasterAllows(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	dest := grid.Square{Col: 7, Row: 7}
	_, err := c.caio.moveTokens(c.campaign, c.mapID, dest, c.toren.GetId())
	wantCode(t, "a player's move while it is off", err, connect.CodePermissionDenied)
	if _, err := c.master.setPlayersMove(c.campaign, c.mapID, true); err != nil {
		t.Fatalf("SetMapPlayersMove() error = %v", err)
	}
	if !c.caio.mustGetMap(c.campaign, c.mapID).GetMap().GetPlayersMoveTokens() {
		t.Error("a player does not read that the players move their tokens")
	}
	_, err = c.caio.moveTokens(c.campaign, c.mapID, dest, c.toren.GetId())
	wantCode(t, "a player's own move while it is on", err, allowed)
	_, err = c.caio.moveTokens(c.campaign, c.mapID, dest, c.pens.GetId())
	wantCode(t, "a player's move of someone else's token", err, connect.CodePermissionDenied)
	_, err = c.caio.moveTokens(c.campaign, c.mapID, dest, c.toren.GetId(), c.brisa.GetId())
	wantCode(t, "a player's move of several tokens", err, connect.CodePermissionDenied)
	_, err = c.caio.moveTokens(c.campaign, c.mapID, dest, c.goblin1.GetId())
	wantCode(t, "a player's move of an NPC's token", err, connect.CodePermissionDenied)
	_, err = c.caio.maps.SetMapPlayersMove(t.Context(), connect.NewRequest(&mapsv1.SetMapPlayersMoveRequest{CampaignId: c.campaign, MapId: c.mapID, Enabled: false}))
	wantCode(t, "a player turning the permission off", err, connect.CodePermissionDenied)
}

// The reason a player's move stopped says what the square in the way is only when the player knows it:
// the "can't pass" layer of a square nobody saw never reaches them, not even in a refusal (RN-10).
func TestB2_APlayerIsNotToldWhatTheyHaveNotSeen(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.master.mustPaint(c.campaign, c.mapID, mapsv1.MapLayer_MAP_LAYER_NO_PASS, 1, [2]int32{10, 7})
	if _, err := c.master.setPlayersMove(c.campaign, c.mapID, true); err != nil {
		t.Fatalf("SetMapPlayersMove() error = %v", err)
	}
	res, err := c.caio.moveTokens(c.campaign, c.mapID, grid.Square{Col: 15, Row: 7}, c.toren.GetId())
	if err != nil {
		t.Fatalf("MoveMapTokens() error = %v", err)
	}
	mv := res.GetMoves()[0]
	if squareOfToken(mv.GetToken()) != (grid.Square{Col: 9, Row: 7}) {
		t.Fatalf("Toren is at %v, want him stopped at (9,7)", squareOfToken(mv.GetToken()))
	}
	known := decodeNoPass(t, c.caio.mustLayers(c.campaign, c.mapID)).Get(10, 7)
	want := mapsv1.MapMoveStop_MAP_MOVE_STOP_BLOCKED
	if known {
		want = mapsv1.MapMoveStop_MAP_MOVE_STOP_NO_PASS
	}
	if mv.GetStoppedBy() != want {
		t.Errorf("stopped by %v, want %v: the reason follows what the layers tell the player", mv.GetStoppedBy(), want)
	}
	if len(res.GetMoves()) != 1 || res.GetBlockedCharacterId() != "" {
		t.Errorf("the answer names more than the player's own token: %v", res)
	}
}

func decodeNoPass(t *testing.T, res *mapsv1.GetMapLayersResponse) *grid.Layer {
	t.Helper()
	b := res.GetNoPass()
	if len(b) == 0 {
		b = make([]byte, grid.LayerSize(caveGrid))
	}
	l, err := grid.DecodeLayer(caveGrid, b)
	if err != nil {
		t.Fatalf("decode the can't-pass layer: %v", err)
	}
	return l
}

func TestB2_TheCantPassLayerIsReadAsTheWallsAre(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.master.mustPaint(c.campaign, c.mapID, mapsv1.MapLayer_MAP_LAYER_NO_PASS, 1, [2]int32{10, 7}, [2]int32{18, 5})
	if got := decodeNoPass(t, c.master.mustLayers(c.campaign, c.mapID)); !got.Get(10, 7) || !got.Get(18, 5) || got.Count() != 2 {
		t.Errorf("the master reads %d squares, want the two painted", got.Count())
	}
	// On a fog map a player gets only the squares they see or remember; with the fog off, all.
	if got := decodeNoPass(t, c.ana.mustLayers(c.campaign, c.mapID)); got.Count() > 2 {
		t.Errorf("a player reads %d squares of the layer", got.Count())
	}
	if _, err := c.master.maps.SetMapFog(t.Context(), connect.NewRequest(&mapsv1.SetMapFogRequest{CampaignId: c.campaign, MapId: c.mapID, FogEnabled: new(false)})); err != nil {
		t.Fatalf("SetMapFog(off) error = %v", err)
	}
	if got := decodeNoPass(t, c.ana.mustLayers(c.campaign, c.mapID)); got.Count() != 2 {
		t.Errorf("a player of a map with no fog reads %d squares, want the two painted", got.Count())
	}
}

func TestB2_NoMovesWhileACombatRunsOnTheMap(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	if _, err := c.master.combat.StartEncounter(t.Context(), connect.NewRequest(&playv1.StartEncounterRequest{
		CampaignId: c.campaign, IdempotencyKey: newKey(), Name: "Emboscada", Participants: []*playv1.Participant{{CharacterId: c.pens.GetId()}},
	})); err != nil {
		t.Fatalf("StartEncounter() error = %v", err)
	}
	_, err := c.master.moveTokens(c.campaign, c.mapID, grid.Square{Col: 8, Row: 7}, c.toren.GetId())
	wantMapBlocked(t, "a move during a combat", err, mapsv1.MapBlockedReason_MAP_BLOCKED_REASON_COMBAT_RUNNING)
	_, err = c.master.maps.PutPartyOnMap(t.Context(), connect.NewRequest(&mapsv1.PutPartyOnMapRequest{
		CampaignId: c.campaign, MapId: c.mapID, CharacterIds: []string{c.toren.GetId()}, XBp: 5000, YBp: 5000,
	}))
	wantMapBlocked(t, "putting the party during a combat", err, mapsv1.MapBlockedReason_MAP_BLOCKED_REASON_COMBAT_RUNNING)
}

func TestB2_MoveRefusals(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	m := c.master
	dest := grid.Square{Col: 8, Row: 7}
	_, err := m.moveTokens(c.campaign, c.mapID, dest)
	wantCode(t, "no characters", err, connect.CodeInvalidArgument)
	_, err = m.moveTokens(c.campaign, c.mapID, dest, c.toren.GetId(), c.toren.GetId())
	wantCode(t, "a repeated character", err, connect.CodeInvalidArgument)
	_, err = m.maps.MoveMapTokens(t.Context(), connect.NewRequest(&mapsv1.MoveMapTokensRequest{CampaignId: c.campaign, MapId: c.mapID, CharacterIds: []string{c.toren.GetId()}, XBp: 10001}))
	wantCode(t, "a position outside the map", err, connect.CodeInvalidArgument)
	other := m.createCharacter(c.campaign, charactersv1.CharacterKind_CHARACTER_KIND_MINION, "Sem token") // an NPC with no token on the map
	_, err = m.moveTokens(c.campaign, c.mapID, dest, other.GetId())
	wantCode(t, "a character with no token on the map", err, connect.CodeNotFound)
	_, err = m.moveTokens(c.campaign, newUUID(), dest, c.toren.GetId())
	wantCode(t, "a map that is not the campaign's", err, connect.CodeNotFound)
	_, err = c.bia.moveTokens(c.campaign, c.probeMap, dest, c.brisa.GetId())
	wantCode(t, "a player on a map they do not see", err, connect.CodeNotFound)
}

func TestB2_PuttingThePartyOnTheMap(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	m := c.master
	m.mustPaint(c.campaign, c.mapID, mapsv1.MapLayer_MAP_LAYER_NO_PASS, 1, [2]int32{9, 7})
	x, y := at(8, 7)
	res, err := m.maps.PutPartyOnMap(t.Context(), connect.NewRequest(&mapsv1.PutPartyOnMapRequest{
		CampaignId: c.campaign, MapId: c.mapID, Formation: mapsv1.MapFormation_MAP_FORMATION_BLOCK, XBp: x, YBp: y,
		CharacterIds: []string{c.toren.GetId(), c.brisa.GetId(), c.pens.GetId(), c.salvia.GetId()},
	}))
	if err != nil {
		t.Fatalf("PutPartyOnMap() error = %v", err)
	}
	// The block is (8,7) (9,7) (8,8) (9,8); (9,7) is a can't-pass square, so Brisa takes the nearest free one.
	want := []grid.Square{{Col: 8, Row: 7}, {Col: 10, Row: 7}, {Col: 8, Row: 8}, {Col: 9, Row: 8}}
	for i, tk := range res.Msg.GetTokens() {
		if got := squareOfToken(tk); got != want[i] {
			t.Errorf("token %d is on %v, want %v", i, got, want[i])
		}
	}
	if res.Msg.GetMovedForRoom() != 1 {
		t.Errorf("moved_for_room = %d, want 1", res.Msg.GetMovedForRoom())
	}
	_, err = c.caio.maps.PutPartyOnMap(t.Context(), connect.NewRequest(&mapsv1.PutPartyOnMapRequest{CampaignId: c.campaign, MapId: c.mapID, CharacterIds: []string{c.toren.GetId()}}))
	wantCode(t, "a player putting the party", err, connect.CodePermissionDenied)
	_, err = m.maps.PutPartyOnMap(t.Context(), connect.NewRequest(&mapsv1.PutPartyOnMapRequest{CampaignId: c.campaign, MapId: c.probeMap, CharacterIds: []string{c.toren.GetId()}}))
	wantMapBlocked(t, "a map with no grid", err, mapsv1.MapBlockedReason_MAP_BLOCKED_REASON_NO_GRID)
}

func TestB2_APartyThatDoesNotFitIsRefused(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	// Every square of the cave but the corridor's first one is a wall or can't be passed.
	var cells [][2]int32
	for row, line := range caveWalls {
		for col, ch := range line {
			if ch != '#' && (col != 0 || row != 7) {
				cells = append(cells, [2]int32{conv.Int32(col), conv.Int32(row)})
			}
		}
	}
	c.master.mustPaint(c.campaign, c.mapID, mapsv1.MapLayer_MAP_LAYER_NO_PASS, 1, cells...)
	_, err := c.master.maps.PutPartyOnMap(t.Context(), connect.NewRequest(&mapsv1.PutPartyOnMapRequest{
		CampaignId: c.campaign, MapId: c.mapID, CharacterIds: []string{c.toren.GetId(), c.brisa.GetId()}, XBp: 100, YBp: 4800,
	}))
	wantMapBlocked(t, "no room for the group", err, mapsv1.MapBlockedReason_MAP_BLOCKED_REASON_NO_ROOM)
}
