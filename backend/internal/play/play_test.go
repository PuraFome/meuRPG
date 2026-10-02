package play

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
)

const allowed connect.Code = 0

// TestAuthorizationMatrix calls every PlayService method as each kind of
// caller (ADR-0011): only the master starts and ends sessions and corrects
// vitals; any member lists sessions and watches the live one; a non-member
// and a pending member (RN-15) get not_found; anonymous, unauthenticated.
func TestAuthorizationMatrix(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player, pending := h.newUser("Mestre"), h.newUser("Jogadora"), h.newUser("Pendente")
	campaign := h.newCampaign(master, "Mirathel", player)
	h.joinPending(master, campaign, pending)
	pc := player.createCharacter(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus")
	open := master.start(t, campaign).GetGameSession()

	type client = playv1connect.PlayServiceClient
	rows := []struct {
		name string
		call func(ctx context.Context, c client) error
		// master, player, non-member, pending member, anonymous
		want [5]connect.Code
	}{
		{"EndGameSession", func(ctx context.Context, c client) error {
			_, err := c.EndGameSession(ctx, connect.NewRequest(&playv1.EndGameSessionRequest{CampaignId: campaign, GameSessionId: open.GetId()}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
		// After the row above, no session is open, so the master may start
		// one; the rows after it run during that session.
		{"StartGameSession", func(ctx context.Context, c client) error {
			_, err := c.StartGameSession(ctx, connect.NewRequest(&playv1.StartGameSessionRequest{CampaignId: campaign}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
		{"ListGameSessions", func(ctx context.Context, c client) error {
			_, err := c.ListGameSessions(ctx, connect.NewRequest(&playv1.ListGameSessionsRequest{CampaignId: campaign}))
			return err
		}, [5]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
		// Anyone signed in; each sees only their own campaigns.
		{"ListOpenGameSessions", func(ctx context.Context, c client) error {
			_, err := c.ListOpenGameSessions(ctx, connect.NewRequest(&playv1.ListOpenGameSessionsRequest{}))
			return err
		}, [5]connect.Code{allowed, allowed, allowed, allowed, connect.CodeUnauthenticated}},
		{"GetLiveSession", func(ctx context.Context, c client) error {
			_, err := c.GetLiveSession(ctx, connect.NewRequest(&playv1.GetLiveSessionRequest{CampaignId: campaign}))
			return err
		}, [5]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
		{"WatchGameSession", func(ctx context.Context, c client) error {
			return firstEventError(ctx, c, campaign)
		}, [5]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
		{"AdjustCharacterVitals", func(ctx context.Context, c client) error {
			_, err := c.AdjustCharacterVitals(ctx, connect.NewRequest(&playv1.AdjustCharacterVitalsRequest{
				CampaignId: campaign, CharacterId: pc.GetId(), IdempotencyKey: newKey(), HitPointsTemporary: proto.Int32(1),
			}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
		// Clearing needs no map nor image; setting them is in package maps'
		// tests, which have maps and images.
		{"SetCurrentMap", func(ctx context.Context, c client) error {
			_, err := c.SetCurrentMap(ctx, connect.NewRequest(&playv1.SetCurrentMapRequest{CampaignId: campaign}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
		{"SetShownImage", func(ctx context.Context, c client) error {
			_, err := c.SetShownImage(ctx, connect.NewRequest(&playv1.SetShownImageRequest{CampaignId: campaign}))
			return err
		}, [5]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeNotFound, connect.CodeUnauthenticated}},
	}

	covered := map[string]bool{}
	for _, r := range rows {
		covered[r.name] = true
	}
	methods := playv1.File_meurpg_play_v1_play_proto.Services().ByName("PlayService").Methods()
	for i := range methods.Len() {
		if name := string(methods.Get(i).Name()); !covered[name] {
			t.Errorf("PlayService.%s is missing from the authorization matrix", name)
		}
	}

	callers := []struct {
		name   string
		client client
	}{
		{"master", master.play},
		{"player", player.play},
		{"non-member", h.newUser("De fora").play},
		{"pending member", pending.play},
		{"anonymous", h.anonymous().play},
	}
	for _, r := range rows {
		for i, caller := range callers {
			t.Run(r.name+"/"+caller.name, func(t *testing.T) {
				err := r.call(t.Context(), caller.client)
				switch want := r.want[i]; {
				case want == allowed && err != nil:
					t.Errorf("error = %v, want allowed", err)
				case want != allowed && connect.CodeOf(err) != want:
					t.Errorf("error = %v, want %v", err, want)
				}
			})
		}
	}
}

// RN-01 (and MR-011's third criterion): when the first session of a
// campaign starts, its players' sheets lock, in the same transaction. Only
// living player characters of that campaign lock: NPCs never do, a dead
// character keeps its state, and other campaigns are untouched.
func TestRN01_StartingTheFirstSessionLocksPlayerSheetsOnly(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player, fallen := h.newUser("Mestre"), h.newUser("Jogadora"), h.newUser("Caída")
	campaign := h.newCampaign(master, "Mirathel", player, fallen)
	other := h.newCampaign(master, "Outra", player)

	pc := player.createCharacter(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus")
	dead := fallen.createCharacter(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Morta")
	if _, err := master.characters.MarkCharacterDead(t.Context(), connect.NewRequest(&charactersv1.MarkCharacterDeadRequest{CampaignId: campaign, CharacterId: dead.GetId()})); err != nil {
		t.Fatalf("MarkCharacterDead() error = %v", err)
	}
	npc := master.createCharacter(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, "Orc")
	minion := master.createCharacter(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_MINION, "Goblin")
	elsewhere := player.createCharacter(t, other, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus")

	res := master.start(t, campaign)
	s := res.GetGameSession()
	if s.GetSessionNumber() != 1 || s.GetCampaignId() != campaign || s.GetStartedAt() == nil || s.GetEndedAt() != nil {
		t.Errorf("StartGameSession() = %v, want open session 1", s)
	}
	if res.GetLockedSheetCount() != 1 {
		t.Errorf("locked_sheet_count = %d, want 1 (only the living player character)", res.GetLockedSheetCount())
	}

	got := master.character(t, pc)
	if got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_LOCKED || !got.GetSheetLockedAt().AsTime().Equal(s.GetStartedAt().AsTime()) {
		t.Errorf("player character = %v at %v, want locked at the session's start %v", got.GetState(), got.GetSheetLockedAt(), s.GetStartedAt())
	}
	if got := master.character(t, dead); got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_DEAD || got.GetSheetLockedAt() != nil {
		t.Errorf("dead character = %v, locked at %v; want dead, never locked", got.GetState(), got.GetSheetLockedAt())
	}
	for _, c := range []*charactersv1.Character{npc, minion} {
		if got := master.character(t, c); got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_DRAFT || !got.GetCanEdit() {
			t.Errorf("NPC %s = %v, want a draft the master edits", c.GetName(), got.GetState())
		}
	}
	if got := player.character(t, elsewhere); got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_DRAFT {
		t.Errorf("the player's character in another campaign = %v, want a draft", got.GetState())
	}
}

// TestOnlyOneOpenSessionPerCampaign: a second start while a session is
// open fails, even when the starts race; after the end, the next start gets
// the next number.
func TestOnlyOneOpenSessionPerCampaign(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master, "Mirathel")

	first := master.start(t, campaign).GetGameSession()
	_, err := master.play.StartGameSession(t.Context(), connect.NewRequest(&playv1.StartGameSessionRequest{CampaignId: campaign}))
	wantCode(t, "StartGameSession() with a session open", err, connect.CodeFailedPrecondition)
	master.end(t, first)

	var wg sync.WaitGroup
	results := make([]error, 5)
	for i := range results {
		wg.Go(func() {
			_, results[i] = master.play.StartGameSession(t.Context(), connect.NewRequest(&playv1.StartGameSessionRequest{CampaignId: campaign}))
		})
	}
	wg.Wait()
	started := 0
	for _, err := range results {
		switch connect.CodeOf(err) {
		case connect.CodeFailedPrecondition:
		default:
			if err != nil {
				t.Errorf("racing StartGameSession() error = %v, want ok or failed_precondition", err)
			}
			started++
		}
	}
	if started != 1 {
		t.Errorf("%d of 5 racing starts succeeded, want exactly 1", started)
	}

	res, err := master.play.ListGameSessions(t.Context(), connect.NewRequest(&playv1.ListGameSessionsRequest{CampaignId: campaign}))
	if err != nil {
		t.Fatalf("ListGameSessions() error = %v", err)
	}
	sessions := res.Msg.GetGameSessions()
	if len(sessions) != 2 || sessions[0].GetSessionNumber() != 2 || sessions[0].GetEndedAt() != nil ||
		sessions[1].GetId() != first.GetId() || sessions[1].GetEndedAt() == nil {
		t.Errorf("ListGameSessions() = %v, want session 2 (open) then session 1 (ended)", sessions)
	}
}

// RN-01 and MR-006's third criterion: a character created after the first
// session (a replacement, or a late player's) stays a draft until the next
// session starts, and then locks.
func TestCharacterCreatedLaterLocksAtTheNextSession(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player, late := h.newUser("Mestre"), h.newUser("Jogadora"), h.newUser("Atrasada")
	campaign := h.newCampaign(master, "Mirathel", player)
	player.createCharacter(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus")
	master.end(t, master.start(t, campaign).GetGameSession())

	h.join(master, campaign, late)
	c := late.createCharacter(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Nova")
	if got := late.character(t, c); got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_DRAFT || !got.GetCanEdit() {
		t.Fatalf("a character created after the first session = %v, want an editable draft", got.GetState())
	}

	res := master.start(t, campaign)
	if res.GetLockedSheetCount() != 1 || res.GetGameSession().GetSessionNumber() != 2 {
		t.Errorf("second StartGameSession() = %v, want session 2 locking 1 sheet", res)
	}
	if got := late.character(t, c); got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_LOCKED || got.GetCanEdit() {
		t.Errorf("after the next session the character = %v, want locked", got.GetState())
	}
}

// RN-01: starting a session ends every story permission the master gave
// (story_editing_allowed), in the same transaction as the lock.
func TestStartingASessionTurnsStoryEditingOff(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, "Mirathel", player)
	c := player.createCharacter(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus")
	master.end(t, master.start(t, campaign).GetGameSession())

	if _, err := master.characters.SetStoryEditing(t.Context(), connect.NewRequest(&charactersv1.SetStoryEditingRequest{
		CampaignId: campaign, CharacterId: c.GetId(), Allowed: true,
	})); err != nil {
		t.Fatalf("SetStoryEditing() error = %v", err)
	}
	if got := player.character(t, c); !got.GetStoryEditingAllowed() || !got.GetCanEditStory() {
		t.Fatalf("after SetStoryEditing(true) the player may not edit the story: %v", got)
	}

	master.start(t, campaign)
	if got := player.character(t, c); got.GetStoryEditingAllowed() || got.GetCanEditStory() {
		t.Errorf("after the next session story_editing_allowed = %v, can_edit_story = %v; want both false",
			got.GetStoryEditingAllowed(), got.GetCanEditStory())
	}
}

// TestEndGameSession: ending twice keeps the first ended_at; a session of
// another campaign, or an ID that is not one, is not found; ending never
// unlocks a sheet.
func TestEndGameSession(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, "Mirathel", player)
	other := h.newCampaign(master, "Outra")
	c := player.createCharacter(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus")
	s := master.start(t, campaign).GetGameSession()

	ended := master.end(t, s)
	again := master.end(t, s)
	if ended.GetEndedAt() == nil || !again.GetEndedAt().AsTime().Equal(ended.GetEndedAt().AsTime()) {
		t.Errorf("ending twice: %v then %v, want the first ended_at kept", ended.GetEndedAt(), again.GetEndedAt())
	}
	if got := player.character(t, c); got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_LOCKED {
		t.Errorf("after the session ended the character = %v, want still locked", got.GetState())
	}
	for _, req := range []*playv1.EndGameSessionRequest{
		{CampaignId: other, GameSessionId: s.GetId()},
		{CampaignId: campaign, GameSessionId: "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e0ff"},
		{CampaignId: campaign, GameSessionId: "sessão 1"},
	} {
		_, err := master.play.EndGameSession(t.Context(), connect.NewRequest(req))
		wantCode(t, "EndGameSession("+req.GetGameSessionId()+")", err, connect.CodeNotFound)
	}
	if res, err := master.play.ListGameSessions(t.Context(), connect.NewRequest(&playv1.ListGameSessionsRequest{CampaignId: other})); err != nil || len(res.Msg.GetGameSessions()) != 0 {
		t.Errorf("ListGameSessions(other) = %v, %v; want none", res, err)
	}
}

// TestResponsesAreNotCached: game sessions describe the caller's campaign.
func TestResponsesAreNotCached(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master, "Mirathel")
	res, err := master.play.ListGameSessions(t.Context(), connect.NewRequest(&playv1.ListGameSessionsRequest{CampaignId: campaign}))
	if err != nil {
		t.Fatalf("ListGameSessions() error = %v", err)
	}
	if got := res.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

// Tests below need no database.

type noSheets struct{}

func (noSheets) LockSheets(context.Context, pgx.Tx, string, time.Time) (int64, error) {
	return 0, errors.New("not in this test")
}

type noVitals struct{}

func (noVitals) ListVitals(context.Context, string) ([]*playv1.CharacterVitals, error) {
	return nil, errors.New("not in this test")
}

func (noVitals) GetVitals(context.Context, string, string) (*playv1.CharacterVitals, error) {
	return nil, errors.New("not in this test")
}

func (noVitals) AdjustVitals(context.Context, pgx.Tx, string, string, *playv1.AdjustCharacterVitalsRequest) (before, after *playv1.CharacterVitals, err error) {
	return nil, nil, errors.New("not in this test")
}

type noMaps struct{}

func (noMaps) RevealMap(context.Context, pgx.Tx, string, string, time.Time) error {
	return errors.New("not in this test")
}

func (noMaps) ShownImage(context.Context, string, string) (*playv1.ShownImage, error) {
	return nil, errors.New("not in this test")
}

func (noMaps) MapGrid(context.Context, string, string) (link.Grid, error) {
	return link.Grid{}, errors.New("not in this test")
}

func (noMaps) BattlePoint(context.Context, string, string) (link.BattlePoint, error) {
	return link.BattlePoint{}, errors.New("not in this test")
}

func (noMaps) MapTokens(context.Context, string) ([]link.TokenPosition, error) {
	return nil, errors.New("not in this test")
}

func (noMaps) SetTokenPositions(context.Context, pgx.Tx, string, []link.TokenPosition, time.Time) error {
	return errors.New("not in this test")
}

type noRoster struct{}

func (noRoster) CombatParty(context.Context, string) ([]link.Character, error) {
	return nil, errors.New("not in this test")
}

func (noRoster) CombatCharacters(context.Context, string, []string) ([]link.Character, error) {
	return nil, errors.New("not in this test")
}

type noDice struct{}

func (noDice) RollsPhysical(context.Context, string, string) (bool, error) {
	return false, errors.New("not in this test")
}

type noCampaigns struct{}

func (noCampaigns) ActiveCampaigns(context.Context, string) ([]*campaignsv1.Campaign, error) {
	return nil, errors.New("not in this test")
}

type noMembers struct{}

func (noMembers) CampaignMembership(context.Context, string, string) (authz.Role, authz.Status, error) {
	return "", "", authz.ErrNotMember
}

func lazyPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), "postgresql://nobody@127.0.0.1:1/none")
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestNewValidatesItsConfig(t *testing.T) {
	t.Parallel()
	pool := lazyPool(t)
	for name, cfg := range map[string]Config{
		"Pool":      {Sheets: noSheets{}, Vitals: noVitals{}, Campaigns: noCampaigns{}, Maps: noMaps{}, Roster: noRoster{}, Dice: noDice{}},
		"Sheets":    {Pool: pool, Vitals: noVitals{}, Campaigns: noCampaigns{}, Maps: noMaps{}, Roster: noRoster{}, Dice: noDice{}},
		"Vitals":    {Pool: pool, Sheets: noSheets{}, Campaigns: noCampaigns{}, Maps: noMaps{}, Roster: noRoster{}, Dice: noDice{}},
		"Campaigns": {Pool: pool, Sheets: noSheets{}, Vitals: noVitals{}, Maps: noMaps{}, Roster: noRoster{}, Dice: noDice{}},
		"Maps":      {Pool: pool, Sheets: noSheets{}, Vitals: noVitals{}, Campaigns: noCampaigns{}, Roster: noRoster{}, Dice: noDice{}},
		"Roster":    {Pool: pool, Sheets: noSheets{}, Vitals: noVitals{}, Campaigns: noCampaigns{}, Maps: noMaps{}, Dice: noDice{}},
		"Dice":      {Pool: pool, Sheets: noSheets{}, Vitals: noVitals{}, Campaigns: noCampaigns{}, Maps: noMaps{}, Roster: noRoster{}},
	} {
		if _, err := New(cfg); err == nil {
			t.Errorf("New() without %s succeeded", name)
		}
	}
}

// TestEveryMethodNeedsASession calls every method signed out, and checks
// that the refusal is not cacheable. Reads are POST-only.
func TestEveryMethodNeedsASession(t *testing.T) {
	t.Parallel()
	svc, err := New(Config{Pool: lazyPool(t), Sheets: noSheets{}, Vitals: noVitals{}, Campaigns: noCampaigns{}, Maps: noMaps{}, Roster: noRoster{}, Dice: noDice{}, Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	mux := http.NewServeMux()
	svc.Mount(mux.Handle, testSessions, noMembers{}, connect.WithRequireConnectProtocolHeader())
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	c := playv1connect.NewPlayServiceClient(server.Client(), server.URL)
	cc := playv1connect.NewCombatServiceClient(server.Client(), server.URL)
	ctx := t.Context()
	id := "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001"

	calls := map[string]error{}
	_, calls["StartGameSession"] = c.StartGameSession(ctx, connect.NewRequest(&playv1.StartGameSessionRequest{CampaignId: id}))
	_, calls["EndGameSession"] = c.EndGameSession(ctx, connect.NewRequest(&playv1.EndGameSessionRequest{CampaignId: id, GameSessionId: id}))
	_, calls["ListGameSessions"] = c.ListGameSessions(ctx, connect.NewRequest(&playv1.ListGameSessionsRequest{CampaignId: id}))
	_, calls["ListOpenGameSessions"] = c.ListOpenGameSessions(ctx, connect.NewRequest(&playv1.ListOpenGameSessionsRequest{}))
	_, calls["GetLiveSession"] = c.GetLiveSession(ctx, connect.NewRequest(&playv1.GetLiveSessionRequest{CampaignId: id}))
	_, calls["AdjustCharacterVitals"] = c.AdjustCharacterVitals(ctx, connect.NewRequest(&playv1.AdjustCharacterVitalsRequest{CampaignId: id, CharacterId: id, IdempotencyKey: id}))
	_, calls["SetCurrentMap"] = c.SetCurrentMap(ctx, connect.NewRequest(&playv1.SetCurrentMapRequest{CampaignId: id, MapId: id}))
	_, calls["SetShownImage"] = c.SetShownImage(ctx, connect.NewRequest(&playv1.SetShownImageRequest{CampaignId: id, ImageId: id}))
	calls["WatchGameSession"] = firstEventError(ctx, c, id)
	methods := playv1.File_meurpg_play_v1_play_proto.Services().ByName("PlayService").Methods()
	if len(calls) != methods.Len() {
		t.Errorf("called %d methods, the service has %d", len(calls), methods.Len())
	}

	// CombatService (MR-013) is mounted with the same checks.
	combat := map[string]error{}
	_, combat["StartEncounter"] = cc.StartEncounter(ctx, connect.NewRequest(&playv1.StartEncounterRequest{CampaignId: id}))
	_, combat["GetEncounter"] = cc.GetEncounter(ctx, connect.NewRequest(&playv1.GetEncounterRequest{CampaignId: id}))
	_, combat["SubmitInitiative"] = cc.SubmitInitiative(ctx, connect.NewRequest(&playv1.SubmitInitiativeRequest{CampaignId: id}))
	_, combat["SetInitiativeOrder"] = cc.SetInitiativeOrder(ctx, connect.NewRequest(&playv1.SetInitiativeOrderRequest{CampaignId: id}))
	_, combat["BeginCombat"] = cc.BeginCombat(ctx, connect.NewRequest(&playv1.BeginCombatRequest{CampaignId: id}))
	_, combat["EndTurn"] = cc.EndTurn(ctx, connect.NewRequest(&playv1.EndTurnRequest{CampaignId: id}))
	_, combat["MoveCombatant"] = cc.MoveCombatant(ctx, connect.NewRequest(&playv1.MoveCombatantRequest{CampaignId: id}))
	_, combat["SetCombatantHidden"] = cc.SetCombatantHidden(ctx, connect.NewRequest(&playv1.SetCombatantHiddenRequest{CampaignId: id}))
	_, combat["AddCombatants"] = cc.AddCombatants(ctx, connect.NewRequest(&playv1.AddCombatantsRequest{CampaignId: id}))
	_, combat["RemoveCombatant"] = cc.RemoveCombatant(ctx, connect.NewRequest(&playv1.RemoveCombatantRequest{CampaignId: id}))
	_, combat["EndEncounter"] = cc.EndEncounter(ctx, connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: id}))
	combatMethods := playv1.File_meurpg_play_v1_combat_proto.Services().ByName("CombatService").Methods()
	if len(combat) != combatMethods.Len() {
		t.Errorf("called %d combat methods, the service has %d", len(combat), combatMethods.Len())
	}
	for name, err := range combat {
		calls[name] = err
	}
	for name, err := range calls {
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Errorf("%s signed out: error = %v, want unauthenticated", name, err)
		}
		if ce, ok := errors.AsType[*connect.Error](err); !ok || !slices.Equal(ce.Meta().Values("Cache-Control"), []string{"no-store"}) {
			t.Errorf("%s: error Cache-Control = %q, want no-store, once", name, ce.Meta().Values("Cache-Control"))
		}
	}

	for method, want := range map[string]descriptorpb.MethodOptions_IdempotencyLevel{
		"ListGameSessions":     descriptorpb.MethodOptions_IDEMPOTENT,
		"GetLiveSession":       descriptorpb.MethodOptions_IDEMPOTENT,
		"ListOpenGameSessions": descriptorpb.MethodOptions_NO_SIDE_EFFECTS, // an empty request
	} {
		opts, _ := methods.ByName(protoreflect.Name(method)).Options().(*descriptorpb.MethodOptions)
		if opts.GetIdempotencyLevel() != want {
			t.Errorf("%s idempotency_level = %v, want %v", method, opts.GetIdempotencyLevel(), want)
		}
	}
	for _, procedure := range []string{playv1connect.PlayServiceListGameSessionsProcedure, playv1connect.PlayServiceGetLiveSessionProcedure} {
		target := procedure + "?connect=v1&encoding=json&message=" + url.QueryEscape(`{"campaignId":"`+id+`"}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, target, nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET %s: status = %d, want 405", procedure, rec.Code)
		}
	}
}

// firstEventError opens WatchGameSession and returns the error that ends
// it before or at its first event (nil if the first event arrived).
func firstEventError(ctx context.Context, c playv1connect.PlayServiceClient, campaignID string) error {
	stream, err := c.WatchGameSession(ctx, connect.NewRequest(&playv1.WatchGameSessionRequest{CampaignId: campaignID}))
	if err != nil {
		return err
	}
	defer stream.Close() //nolint:errcheck // the test only needs the first event
	if stream.Receive() {
		return nil
	}
	return stream.Err()
}
