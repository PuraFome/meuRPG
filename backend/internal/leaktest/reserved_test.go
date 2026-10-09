package leaktest

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
)

// A reserved character (MR-049) is invisible to every player until one claims it: the
// matrix (matrix_test.go) holds its name, story and id as canaries and finds them in no
// player's answer. These tests add the surfaces the matrix cannot reach by asking a
// read, and the positive control: once a player claims it, that player reads it, and
// nobody else does.

func TestReservedCharacterIsOnNoLiveSurface(t *testing.T) {
	w := newWorld(t)
	ctx := t.Context()

	// The master's own read finds it (the control that the character exists).
	mine := must(w.master.characters.ListCharacters(ctx, rq(&charactersv1.ListCharactersRequest{CampaignId: w.campaign})))
	var found *charactersv1.CharacterSummary
	for _, c := range mine.GetCharacters() {
		if c.GetId() == w.reserved.GetId() {
			found = c
		}
	}
	if found == nil || !found.GetReserved() || found.GetClaimState() != charactersv1.ClaimState_CLAIM_STATE_SENT {
		t.Fatalf("the master's list = %v; want the reserved character with a link sent", found)
	}
	// It is no combatant: the combat the fixture started takes the living party.
	for _, c := range w.encounter.GetCombatants() {
		if c.GetCharacterId() == w.reserved.GetId() || strings.Contains(c.GetLabel(), "LEAKCANARY-reserved") {
			t.Errorf("the combat holds the reserved character: %v", c)
		}
	}
	// It is no token: a map takes the living characters of the campaign.
	x, y := at(10, 13)
	_, err := w.master.maps.PlaceMapToken(ctx, rq(&mapsv1.PlaceMapTokenRequest{CampaignId: w.campaign, MapId: w.fogMap, CharacterId: w.reserved.GetId(), XBp: x, YBp: y}))
	if err == nil {
		t.Error("the master placed a reserved character's token on a map")
	}
	// No player reads it by id, with or without a session of their own in the campaign.
	for _, p := range w.players() {
		_, err := p.characters.GetCharacter(ctx, rq(&charactersv1.GetCharacterRequest{CampaignId: w.campaign, CharacterId: w.reserved.GetId()}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("%s reading the reserved character by id: %v, want not_found", p.name, err)
		}
	}
}

func TestClaimedCharacterIsReadByItsNewOwnerAndNoOneElse(t *testing.T) {
	w := newWorld(t)
	ctx := t.Context()
	// Eva is no member: the link is all she holds.
	if got, err := w.stranger.characters.GetCharacter(ctx, rq(&charactersv1.GetCharacterRequest{CampaignId: w.campaign, CharacterId: w.reserved.GetId()})); err == nil {
		t.Fatalf("a stranger read the reserved character before claiming it: %v", got)
	}
	card := must(w.stranger.characters.PreviewClaim(ctx, rq(&charactersv1.PreviewClaimRequest{Token: w.claimToken})))
	if card.GetCard().GetCampaignName() != "Mirathel" || card.GetCard().GetCharacterName() == "" {
		t.Fatalf("PreviewClaim() = %v; want the public card", card)
	}
	// Previewing is not claiming.
	if _, err := w.stranger.characters.GetCharacter(ctx, rq(&charactersv1.GetCharacterRequest{CampaignId: w.campaign, CharacterId: w.reserved.GetId()})); err == nil {
		t.Fatal("previewing the link made the stranger the owner")
	}
	must(w.stranger.characters.ClaimCharacter(ctx, rq(&charactersv1.ClaimCharacterRequest{Token: w.claimToken})))

	// The positive control: the owner reads it, and the master still does.
	for _, p := range []*person{w.stranger, w.master} {
		got, err := p.characters.GetCharacter(ctx, rq(&charactersv1.GetCharacterRequest{CampaignId: w.campaign, CharacterId: w.reserved.GetId()}))
		if err != nil || got.Msg.GetCharacter().GetReserved() {
			t.Errorf("%s reading the claimed character: %v, %v; want it, no longer reserved", p.name, got, err)
		}
	}
	// Nobody else does: not the other players, not the pending member.
	for _, p := range []*person{w.ana, w.caio, w.pending} {
		_, err := p.characters.GetCharacter(ctx, rq(&charactersv1.GetCharacterRequest{CampaignId: w.campaign, CharacterId: w.reserved.GetId()}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("%s reading Eva's character: %v, want not_found", p.name, err)
		}
		list := must(p.characters.ListCharacters(ctx, rq(&charactersv1.ListCharactersRequest{CampaignId: w.campaign})))
		for _, c := range list.GetCharacters() {
			if c.GetId() == w.reserved.GetId() {
				t.Errorf("%s lists Eva's character", p.name)
			}
		}
	}
	// And the link is spent: the same refusal as any link that does not work.
	_, err := w.caio.characters.ClaimCharacter(ctx, rq(&charactersv1.ClaimCharacterRequest{Token: w.claimToken}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a second claim of the link: %v, want not_found", err)
	}
}

func TestGivingACharacterBackToTheReserveHidesItFromItsFormerOwner(t *testing.T) {
	w := newWorld(t)
	ctx := t.Context()
	must(w.stranger.characters.ClaimCharacter(ctx, rq(&charactersv1.ClaimCharacterRequest{Token: w.claimToken})))
	must(w.master.characters.ReturnCharacterToReserve(ctx, rq(&charactersv1.ReturnCharacterToReserveRequest{CampaignId: w.campaign, CharacterId: w.reserved.GetId()})))
	for _, p := range w.players() {
		_, err := p.characters.GetCharacter(ctx, rq(&charactersv1.GetCharacterRequest{CampaignId: w.campaign, CharacterId: w.reserved.GetId()}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("%s reading the character given back: %v, want not_found", p.name, err)
		}
		list := must(p.characters.ListCharacters(ctx, rq(&charactersv1.ListCharactersRequest{CampaignId: w.campaign})))
		for _, c := range list.GetCharacters() {
			if c.GetId() == w.reserved.GetId() {
				t.Errorf("%s lists the character given back", p.name)
			}
		}
	}
}
