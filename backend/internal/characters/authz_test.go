package characters

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoreflect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// allowed marks a call that must succeed in the authorization matrix.
const allowed connect.Code = 0

// TestAuthorizationMatrix calls every CharacterService and ContentService
// method as each kind of caller and checks who gets in (ADR-0011, and the
// table in docs/arquitetura.md):
//
//	master        the campaign's master
//	owner         the player who owns the character the call is about
//	other player  another player of the same campaign
//	non-member    signed in, but not in the campaign: not_found, so the
//	              campaign's existence does not leak
//	anonymous     unauthenticated, always
//
// A member who may not see a character gets not_found too, so NPC IDs and
// other players' characters do not leak; a member asking for something only
// the master may do gets permission_denied.
func TestAuthorizationMatrix(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, owner, other := h.newUser("Mestre"), h.newUser("Dona"), h.newUser("Outra")
	campaign := h.newCampaign(master, "Mirathel", owner, other)
	pc := owner.createPensantus(t, campaign)
	npc := master.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_BOSS, "Strahd", enemySheet())

	// fresh reads a character's current revision as the master, so every
	// caller's write is judged on its permission, not on a stale revision.
	fresh := func(id string) *charactersv1.Character { return master.get(t, campaign, id) }
	create := func(kind charactersv1.CharacterKind, sheet *charactersv1.CharacterSheet) func(ctx context.Context, u *user) error {
		return func(ctx context.Context, u *user) error {
			_, err := u.api.CreateCharacter(ctx, connect.NewRequest(&charactersv1.CreateCharacterRequest{
				CampaignId: campaign, Kind: kind, Name: "Novo", Sheet: sheet,
			}))
			return err
		}
	}
	get := func(id string) func(ctx context.Context, u *user) error {
		return func(ctx context.Context, u *user) error {
			_, err := u.api.GetCharacter(ctx, connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: campaign, CharacterId: id}))
			return err
		}
	}
	update := func(id string) func(ctx context.Context, u *user) error {
		return func(ctx context.Context, u *user) error {
			c := fresh(id)
			_, err := u.api.UpdateCharacter(ctx, connect.NewRequest(&charactersv1.UpdateCharacterRequest{
				CampaignId: campaign, CharacterId: id, Revision: c.GetRevision(), Name: c.GetName(), Sheet: c.GetSheet(),
			}))
			return err
		}
	}
	updateStory := func(id string) func(ctx context.Context, u *user) error {
		return func(ctx context.Context, u *user) error {
			_, err := u.api.UpdateCharacterStory(ctx, connect.NewRequest(&charactersv1.UpdateCharacterStoryRequest{
				CampaignId: campaign, CharacterId: id, Revision: fresh(id).GetRevision(), Story: &charactersv1.CharacterStory{Allies: "A Ordem"},
			}))
			return err
		}
	}

	type rpc = func(ctx context.Context, u *user) error
	rows := []struct {
		method string // the RPC, to check that every method has a row
		label  string
		before func() // runs once, before the row's calls
		call   rpc
		// The expected code for master, owner, other player, non-member,
		// anonymous.
		want [5]connect.Code
	}{
		// The owner already has a living character (RN-03); the other
		// player has none yet, so they may create theirs.
		{
			"CreateCharacter", "player character", nil, create(charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, pensantusSheet()),
			[5]connect.Code{connect.CodePermissionDenied, connect.CodeFailedPrecondition, allowed, connect.CodeNotFound, connect.CodeUnauthenticated},
		},
		{
			"CreateCharacter", "enemy", nil, create(charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, enemySheet()),
			[5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated},
		},
		{
			"CreateCharacter", "minion", nil, create(charactersv1.CharacterKind_CHARACTER_KIND_MINION, basicSheet()),
			[5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated},
		},

		{
			"GetCharacter", "player character", nil, get(pc.GetId()),
			[5]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated},
		},
		{
			"GetCharacter", "NPC", nil, get(npc.GetId()),
			[5]connect.Code{allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated},
		},

		{"ListCharacters", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.ListCharacters(ctx, connect.NewRequest(&charactersv1.ListCharactersRequest{CampaignId: campaign}))
			return err
		}, [5]connect.Code{allowed, allowed, allowed, connect.CodeNotFound, connect.CodeUnauthenticated}},

		{
			"UpdateCharacter", "draft", nil, update(pc.GetId()),
			[5]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated},
		},
		{
			"UpdateCharacter", "NPC", nil, update(npc.GetId()),
			[5]connect.Code{allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated},
		},

		{
			"UpdateCharacterStory", "draft", nil, updateStory(pc.GetId()),
			[5]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated},
		},
		{
			"UpdateCharacterStory", "NPC", nil, updateStory(npc.GetId()),
			[5]connect.Code{allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated},
		},

		{"SetStoryEditing", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.SetStoryEditing(ctx, connect.NewRequest(&charactersv1.SetStoryEditingRequest{CampaignId: campaign, CharacterId: pc.GetId(), Allowed: true}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated}},

		{"GetMasterNotes", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.GetMasterNotes(ctx, connect.NewRequest(&charactersv1.GetMasterNotesRequest{CampaignId: campaign, CharacterId: pc.GetId()}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated}},

		{"UpdateMasterNotes", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.UpdateMasterNotes(ctx, connect.NewRequest(&charactersv1.UpdateMasterNotesRequest{CampaignId: campaign, CharacterId: pc.GetId(), Notes: "segredo"}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated}},

		{"ListContent", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.content.ListContent(ctx, connect.NewRequest(&rulesv1.ListContentRequest{CampaignId: campaign}))
			return err
		}, [5]connect.Code{allowed, allowed, allowed, connect.CodeNotFound, connect.CodeUnauthenticated}},

		// A game session starts: the sheet locks, and the story permission
		// the master gave above ends (RN-01).
		{
			"UpdateCharacter", "locked", func() { h.lockSheets(campaign) }, update(pc.GetId()),
			[5]connect.Code{allowed, connect.CodeFailedPrecondition, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated},
		},
		{
			"UpdateCharacterStory", "locked", nil, updateStory(pc.GetId()),
			[5]connect.Code{allowed, connect.CodeFailedPrecondition, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated},
		},

		// Last, because it changes the owner's character for good.
		{"MarkCharacterDead", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.MarkCharacterDead(ctx, connect.NewRequest(&charactersv1.MarkCharacterDeadRequest{CampaignId: campaign, CharacterId: pc.GetId()}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated}},
	}

	// Every method of both services must be in the table, so a new RPC
	// cannot ship without its authorization test.
	covered := map[string]bool{}
	for _, r := range rows {
		covered[r.method] = true
	}
	for _, service := range []protoreflect.ServiceDescriptor{
		charactersv1.File_meurpg_characters_v1_characters_proto.Services().ByName("CharacterService"),
		rulesv1.File_meurpg_rules_v1_rules_proto.Services().ByName("ContentService"),
	} {
		for i := range service.Methods().Len() {
			if name := string(service.Methods().Get(i).Name()); !covered[name] {
				t.Errorf("%s.%s is missing from the authorization matrix", service.Name(), name)
			}
		}
	}

	callers := []struct {
		name string
		user *user
	}{
		{"master", master},
		{"owner", owner},
		{"other player", other},
		{"non-member", h.newUser("De fora")},
		{"anonymous", h.anonymous()},
	}
	// Sequential on purpose: some rows change the state the next ones see.
	for _, r := range rows {
		if r.before != nil {
			r.before()
		}
		for i, caller := range callers {
			t.Run(strings.TrimSuffix(r.method+"/"+r.label, "/")+"/"+caller.name, func(t *testing.T) {
				err := r.call(t.Context(), caller.user)
				want := r.want[i]
				switch {
				case want == allowed && err != nil:
					t.Errorf("error = %v, want allowed", err)
				case want != allowed && connect.CodeOf(err) != want:
					t.Errorf("error = %v, want %v", err, want)
				}
			})
		}
	}
}

// TestNonMembersCannotTellCharactersApart: a player asking for a character
// they may not see gets the same answer as for one that does not exist,
// whatever the reason; so does a non-member for any character of the
// campaign.
func TestNonMembersCannotTellCharactersApart(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, owner, other, outsider := h.newUser("Mestre"), h.newUser("Dona"), h.newUser("Outra"), h.newUser("De fora")
	campaign := h.newCampaign(master, "Mirathel", owner, other)
	elsewhere := h.newCampaign(master, "Outra campanha", other)
	pc := owner.createPensantus(t, campaign)
	npc := master.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_STORY, "Taverneiro", basicSheet())
	otherCampaignPC := other.createPensantus(t, elsewhere)

	ids := []string{pc.GetId(), npc.GetId(), otherCampaignPC.GetId(), "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e0ff", "pensantus"}
	var messages []string
	for _, id := range ids {
		_, err := other.api.GetCharacter(t.Context(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: campaign, CharacterId: id}))
		wantCode(t, "other player's GetCharacter("+id+")", err, connect.CodeNotFound)
		messages = append(messages, err.Error())
		_, err = other.api.UpdateCharacterStory(t.Context(), connect.NewRequest(&charactersv1.UpdateCharacterStoryRequest{CampaignId: campaign, CharacterId: id, Revision: 1}))
		wantCode(t, "other player's UpdateCharacterStory("+id+")", err, connect.CodeNotFound)
		messages = append(messages, err.Error())
	}
	for _, msg := range messages[1:] {
		if msg != messages[0] {
			t.Errorf("error messages differ: %q", messages)
			break
		}
	}

	// A non-member cannot tell the campaign apart from one that does not
	// exist either.
	_, errReal := outsider.api.ListCharacters(t.Context(), connect.NewRequest(&charactersv1.ListCharactersRequest{CampaignId: campaign}))
	_, errFake := outsider.api.ListCharacters(t.Context(), connect.NewRequest(&charactersv1.ListCharactersRequest{CampaignId: "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e0ff"}))
	if connect.CodeOf(errReal) != connect.CodeNotFound || errReal.Error() != errFake.Error() {
		t.Errorf("non-member errors differ: %v vs %v", errReal, errFake)
	}
}

// TestResponsesAreNotCached: every answer describes the caller's
// characters, so none may be stored by a browser or a proxy.
func TestResponsesAreNotCached(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, "Mirathel", player)
	c := player.createPensantus(t, campaign)

	res, err := player.api.GetCharacter(t.Context(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: campaign, CharacterId: c.GetId()}))
	if err != nil {
		t.Fatalf("GetCharacter() error = %v", err)
	}
	if got := res.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("GetCharacter Cache-Control = %q, want no-store", got)
	}
	content, err := player.content.ListContent(t.Context(), connect.NewRequest(&rulesv1.ListContentRequest{CampaignId: campaign}))
	if err != nil {
		t.Fatalf("ListContent() error = %v", err)
	}
	if got := content.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("ListContent Cache-Control = %q, want no-store", got)
	}

	_, err = player.api.GetMasterNotes(t.Context(), connect.NewRequest(&charactersv1.GetMasterNotesRequest{CampaignId: campaign, CharacterId: c.GetId()}))
	ce, ok := errors.AsType[*connect.Error](err)
	if !ok || ce.Meta().Get("Cache-Control") != "no-store" {
		t.Errorf("error response %v lacks Cache-Control: no-store", err)
	}
}
