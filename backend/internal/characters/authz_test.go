package characters

import (
	"context"
	"errors"
	"strings"
	"testing"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
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
//	pending       a pending member (RN-15, MR-024) with their own pending
//	              character: treated as a player in the calls package authz
//	              lets them make (pendingMayCall), where they see only that
//	              character; not_found everywhere else, like a non-member
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
	// Three pending members (RN-15): the "pending" caller, and two whose
	// characters the ApproveCharacter and RejectCharacter rows decide on,
	// so those rows do not change the pending caller.
	pending, approvee, rejectee := h.newUser("Pendente"), h.newUser("Aprovada"), h.newUser("Recusado")
	h.joinPending(master, campaign, pending, approvee, rejectee)
	pendingPC := pending.createPensantus(t, campaign)
	approveePC := approvee.createPensantus(t, campaign)
	rejecteePC := rejectee.createPensantus(t, campaign)

	// fresh reads a character's current revision as the master, so every
	// caller's write is judged on its permission, not on a stale revision.
	fresh := func(id string) *charactersv1.Character { return master.get(t, campaign, id) }
	// gift is a creature the master gave Pensantus, for the creature rows.
	var giftCreature *charactersv1.CharacterCreature
	gift := func() *charactersv1.CharacterCreature {
		if giftCreature == nil {
			res, err := master.api.GiveCreature(t.Context(), connect.NewRequest(&charactersv1.GiveCreatureRequest{CampaignId: campaign, CharacterId: pc.GetId(), MonsterKey: "monster:wolf", Name: "Presa"}))
			if err != nil {
				t.Fatalf("GiveCreature() error = %v", err)
			}
			giftCreature = res.Msg.GetCreature()
		}
		return giftCreature
	}
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
		// anonymous, pending member.
		want [6]connect.Code
	}{
		// The owner already has a living character (RN-03); the other
		// player has none yet, so they may create theirs.
		{
			"CreateCharacter", "player character", nil, create(charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, pensantusSheet()),
			[6]connect.Code{connect.CodePermissionDenied, connect.CodeFailedPrecondition, allowed, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeFailedPrecondition},
		},
		{
			"CreateCharacter", "enemy", nil, create(charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, enemySheet()),
			[6]connect.Code{allowed, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodePermissionDenied},
		},
		{
			"CreateCharacter", "minion", nil, create(charactersv1.CharacterKind_CHARACTER_KIND_MINION, basicSheet()),
			[6]connect.Code{allowed, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodePermissionDenied},
		},

		{
			"GetCharacter", "player character", nil, get(pc.GetId()),
			[6]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound},
		},
		{
			"GetCharacter", "NPC", nil, get(npc.GetId()),
			[6]connect.Code{allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound},
		},

		{"ListCharacters", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.ListCharacters(ctx, connect.NewRequest(&charactersv1.ListCharactersRequest{CampaignId: campaign}))
			return err
		}, [6]connect.Code{allowed, allowed, allowed, connect.CodeNotFound, connect.CodeUnauthenticated, allowed}},

		{
			"UpdateCharacter", "draft", nil, update(pc.GetId()),
			[6]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound},
		},
		{
			"UpdateCharacter", "NPC", nil, update(npc.GetId()),
			[6]connect.Code{allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound},
		},

		{
			"UpdateCharacterStory", "draft", nil, updateStory(pc.GetId()),
			[6]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound},
		},
		{
			"UpdateCharacterStory", "NPC", nil, updateStory(npc.GetId()),
			[6]connect.Code{allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound},
		},

		// The pending member's own pending character (RN-15): they read and
		// edit it; nobody but the master sees it.
		{
			"GetCharacter", "pending character", nil, get(pendingPC.GetId()),
			[6]connect.Code{allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, allowed},
		},
		{
			"UpdateCharacter", "pending character", nil, update(pendingPC.GetId()),
			[6]connect.Code{allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, allowed},
		},
		{
			"UpdateCharacterStory", "pending character", nil, updateStory(pendingPC.GetId()),
			[6]connect.Code{allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, allowed},
		},

		// Only the master settles a pending character. Each row decides on
		// another pending member's character, so the pending caller stays
		// pending for the rows below.
		{"ApproveCharacter", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.ApproveCharacter(ctx, connect.NewRequest(&charactersv1.ApproveCharacterRequest{CampaignId: campaign, CharacterId: approveePC.GetId()}))
			return err
		}, [6]connect.Code{allowed, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"RejectCharacter", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.RejectCharacter(ctx, connect.NewRequest(&charactersv1.RejectCharacterRequest{CampaignId: campaign, CharacterId: rejecteePC.GetId()}))
			return err
		}, [6]connect.Code{allowed, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},

		{"SetStoryEditing", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.SetStoryEditing(ctx, connect.NewRequest(&charactersv1.SetStoryEditingRequest{CampaignId: campaign, CharacterId: pc.GetId(), Allowed: true}))
			return err
		}, [6]connect.Code{allowed, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},

		{"GetMasterNotes", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.GetMasterNotes(ctx, connect.NewRequest(&charactersv1.GetMasterNotesRequest{CampaignId: campaign, CharacterId: pc.GetId()}))
			return err
		}, [6]connect.Code{allowed, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},

		{"UpdateMasterNotes", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.UpdateMasterNotes(ctx, connect.NewRequest(&charactersv1.UpdateMasterNotesRequest{CampaignId: campaign, CharacterId: pc.GetId(), Notes: "segredo"}))
			return err
		}, [6]connect.Code{allowed, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},

		{"ListContent", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.content.ListContent(ctx, connect.NewRequest(&rulesv1.ListContentRequest{CampaignId: campaign}))
			return err
		}, [6]connect.Code{allowed, allowed, allowed, connect.CodeNotFound, connect.CodeUnauthenticated, allowed}},

		{"GetSpellDetails", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.content.GetSpellDetails(ctx, connect.NewRequest(&rulesv1.GetSpellDetailsRequest{CampaignId: campaign, SpellKey: "spell:fire-bolt"}))
			return err
		}, [6]connect.Code{allowed, allowed, allowed, connect.CodeNotFound, connect.CodeUnauthenticated, allowed}},

		// The creatures are public rules, but only for an active member: the
		// pending member gets not_found (RN-15).
		{"ListCreatures", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.content.ListCreatures(ctx, connect.NewRequest(&rulesv1.ListCreaturesRequest{CampaignId: campaign, Query: "lobo"}))
			return err
		}, [6]connect.Code{allowed, allowed, allowed, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},

		{"GetCreature", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.content.GetCreature(ctx, connect.NewRequest(&rulesv1.GetCreatureRequest{CampaignId: campaign, Key: "monster:wolf"}))
			return err
		}, [6]connect.Code{allowed, allowed, allowed, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},

		// The trap and light presets are public rules too, for active members only.
		{"ListTrapPresets", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.content.ListTrapPresets(ctx, connect.NewRequest(&rulesv1.ListTrapPresetsRequest{CampaignId: campaign}))
			return err
		}, [6]connect.Code{allowed, allowed, allowed, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},

		{"ListLightPresets", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.content.ListLightPresets(ctx, connect.NewRequest(&rulesv1.ListLightPresetsRequest{CampaignId: campaign}))
			return err
		}, [6]connect.Code{allowed, allowed, allowed, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},

		// A game session starts: the sheet locks, and the story permission
		// the master gave above ends (RN-01).
		{
			"UpdateCharacter", "locked", func() { h.lockSheets(campaign) }, update(pc.GetId()),
			[6]connect.Code{allowed, connect.CodeFailedPrecondition, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound},
		},
		{
			"UpdateCharacterStory", "locked", nil, updateStory(pc.GetId()),
			[6]connect.Code{allowed, connect.CodeFailedPrecondition, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound},
		},

		// The guided level-up (MR-040): the sheet is locked now, the character
		// has the XP for level 4, and the XP rule is wired. The master and the
		// owner read the options; only the owner rolls and levels up (the
		// master keeps the editor); only the master reads the record.
		{"GetLevelUpOptions", "", func() {
			h.svc.SetLevelUps(xpLevelUps{})
			c := fresh(pc.GetId())
			sheet := proto.CloneOf(c.GetSheet())
			sheet.GetFull().ExperiencePoints = 2700
			if _, err := master.update(t, c, c.GetName(), sheet); err != nil {
				t.Fatalf("give the XP: %v", err)
			}
		}, func(ctx context.Context, u *user) error {
			_, err := u.api.GetLevelUpOptions(ctx, connect.NewRequest(&charactersv1.GetLevelUpOptionsRequest{CampaignId: campaign, CharacterId: pc.GetId()}))
			return err
		}, [6]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"PreviewLevelUp", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.PreviewLevelUp(ctx, connect.NewRequest(&charactersv1.PreviewLevelUpRequest{CampaignId: campaign, CharacterId: pc.GetId(), Choices: pensantusLevelUp()}))
			return err
		}, [6]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"RollLevelUpHitPoints", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.RollLevelUpHitPoints(ctx, connect.NewRequest(&charactersv1.RollLevelUpHitPointsRequest{CampaignId: campaign, CharacterId: pc.GetId(), IdempotencyKey: uuid.New().String()}))
			return err
		}, [6]connect.Code{connect.CodePermissionDenied, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		// The ability scores of a new sheet (RN-24): any player may roll and read their
		// own 4d6, a pending member too (they create their character); the master has
		// none to make. The roll is idempotent, so it can sit anywhere in the table.
		{"GetAbilityRolls", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.GetAbilityRolls(ctx, connect.NewRequest(&charactersv1.GetAbilityRollsRequest{CampaignId: campaign}))
			return err
		}, [6]connect.Code{connect.CodePermissionDenied, allowed, allowed, connect.CodeNotFound, connect.CodeUnauthenticated, allowed}},
		{"RollAbilityScores", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.RollAbilityScores(ctx, connect.NewRequest(&charactersv1.RollAbilityScoresRequest{CampaignId: campaign}))
			return err
		}, [6]connect.Code{connect.CodePermissionDenied, allowed, allowed, connect.CodeNotFound, connect.CodeUnauthenticated, allowed}},
		{"ListLevelUps", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.ListLevelUps(ctx, connect.NewRequest(&charactersv1.ListLevelUpsRequest{CampaignId: campaign}))
			return err
		}, [6]connect.Code{allowed, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		// The owner's call goes through, so it comes after the reads above.
		{"LevelUpCharacter", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.LevelUpCharacter(ctx, connect.NewRequest(&charactersv1.LevelUpCharacterRequest{
				CampaignId: campaign, CharacterId: pc.GetId(), Revision: fresh(pc.GetId()).GetRevision(), Choices: pensantusLevelUp(),
			}))
			return err
		}, [6]connect.Code{connect.CodePermissionDenied, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},

		// The character's creatures (MR-037): the master and the owner's player
		// read them; every other player and the pending member get not_found, as
		// for the character itself; only the master gives and corrects hit points.
		{"GiveCreature", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.GiveCreature(ctx, connect.NewRequest(&charactersv1.GiveCreatureRequest{CampaignId: campaign, CharacterId: pc.GetId(), MonsterKey: "monster:wolf"}))
			return err
		}, [6]connect.Code{allowed, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"ListCharacterCreatures", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.ListCharacterCreatures(ctx, connect.NewRequest(&charactersv1.ListCharacterCreaturesRequest{CampaignId: campaign, CharacterId: pc.GetId()}))
			return err
		}, [6]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		// What the character can summon (MR-037): the same access as the list.
		{"GetSummonOptions", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.GetSummonOptions(ctx, connect.NewRequest(&charactersv1.GetSummonOptionsRequest{CampaignId: campaign, CharacterId: pc.GetId()}))
			return err
		}, [6]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		// The beasts a druid may become (MR-037): the master and the character's own
		// player read them; another player and the pending member get not_found.
		{"ListWildShapeForms", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.ListWildShapeForms(ctx, connect.NewRequest(&charactersv1.ListWildShapeFormsRequest{CampaignId: campaign, CharacterId: pc.GetId()}))
			return err
		}, [6]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"RenameCreature", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.RenameCreature(ctx, connect.NewRequest(&charactersv1.RenameCreatureRequest{CampaignId: campaign, CreatureId: gift().GetId(), Name: "Presa"}))
			return err
		}, [6]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		{"AdjustCreatureHitPoints", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.AdjustCreatureHitPoints(ctx, connect.NewRequest(&charactersv1.AdjustCreatureHitPointsRequest{
				CampaignId: campaign, CreatureId: gift().GetId(), Change: &charactersv1.AdjustCreatureHitPointsRequest_Damage{Damage: 1},
			}))
			return err
		}, [6]connect.Code{allowed, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
		// The master dismisses it; for the owner it is gone already, which changes
		// nothing; another player never sees it.
		{"DismissCreature", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.DismissCreature(ctx, connect.NewRequest(&charactersv1.DismissCreatureRequest{CampaignId: campaign, CreatureId: gift().GetId()}))
			return err
		}, [6]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},

		// A game session does not lock a pending character (RN-15): its
		// player still edits it.
		{
			"UpdateCharacter", "pending character, after a session started", nil, update(pendingPC.GetId()),
			[6]connect.Code{allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated, allowed},
		},
		// A pending character is approved or rejected, never marked dead.
		{"MarkCharacterDead", "pending character", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.MarkCharacterDead(ctx, connect.NewRequest(&charactersv1.MarkCharacterDeadRequest{CampaignId: campaign, CharacterId: pendingPC.GetId()}))
			return err
		}, [6]connect.Code{connect.CodeFailedPrecondition, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},

		// Last, because it changes the owner's character for good.
		{"MarkCharacterDead", "", nil, func(ctx context.Context, u *user) error {
			_, err := u.api.MarkCharacterDead(ctx, connect.NewRequest(&charactersv1.MarkCharacterDeadRequest{CampaignId: campaign, CharacterId: pc.GetId()}))
			return err
		}, [6]connect.Code{allowed, connect.CodePermissionDenied, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated, connect.CodeNotFound}},
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
		{"pending", pending},
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
