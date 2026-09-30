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
	"google.golang.org/protobuf/types/descriptorpb"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
)

const allowed connect.Code = 0

// TestAuthorizationMatrix calls every PlayService method as each kind of
// caller (ADR-0011): only the master starts and ends sessions; any member
// lists them; a non-member gets not_found; anonymous, unauthenticated.
func TestAuthorizationMatrix(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, "Mirathel", player)
	open := master.start(t, campaign).GetGameSession()

	type client = playv1connect.PlayServiceClient
	rows := []struct {
		name string
		call func(ctx context.Context, c client) error
		// master, player, non-member, anonymous
		want [4]connect.Code
	}{
		{"EndGameSession", func(ctx context.Context, c client) error {
			_, err := c.EndGameSession(ctx, connect.NewRequest(&playv1.EndGameSessionRequest{CampaignId: campaign, GameSessionId: open.GetId()}))
			return err
		}, [4]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated}},
		// After the row above, no session is open, so the master may start
		// one.
		{"StartGameSession", func(ctx context.Context, c client) error {
			_, err := c.StartGameSession(ctx, connect.NewRequest(&playv1.StartGameSessionRequest{CampaignId: campaign}))
			return err
		}, [4]connect.Code{allowed, connect.CodePermissionDenied, connect.CodeNotFound, connect.CodeUnauthenticated}},
		{"ListGameSessions", func(ctx context.Context, c client) error {
			_, err := c.ListGameSessions(ctx, connect.NewRequest(&playv1.ListGameSessionsRequest{CampaignId: campaign}))
			return err
		}, [4]connect.Code{allowed, allowed, connect.CodeNotFound, connect.CodeUnauthenticated}},
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

type noMembers struct{}

func (noMembers) CampaignRole(context.Context, string, string) (authz.Role, error) {
	return "", authz.ErrNotMember
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
	if _, err := New(Config{Sheets: noSheets{}}); err == nil {
		t.Error("New() without a Pool succeeded")
	}
	if _, err := New(Config{Pool: lazyPool(t)}); err == nil {
		t.Error("New() without Sheets succeeded")
	}
}

// TestEveryMethodNeedsASession calls every method signed out, and checks
// that the refusal is not cacheable. Reads are POST-only.
func TestEveryMethodNeedsASession(t *testing.T) {
	t.Parallel()
	svc, err := New(Config{Pool: lazyPool(t), Sheets: noSheets{}, Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	mux := http.NewServeMux()
	svc.Mount(mux.Handle, testSessions, noMembers{}, connect.WithRequireConnectProtocolHeader())
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	c := playv1connect.NewPlayServiceClient(server.Client(), server.URL)
	ctx := t.Context()
	id := "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001"

	calls := map[string]error{}
	_, calls["StartGameSession"] = c.StartGameSession(ctx, connect.NewRequest(&playv1.StartGameSessionRequest{CampaignId: id}))
	_, calls["EndGameSession"] = c.EndGameSession(ctx, connect.NewRequest(&playv1.EndGameSessionRequest{CampaignId: id, GameSessionId: id}))
	_, calls["ListGameSessions"] = c.ListGameSessions(ctx, connect.NewRequest(&playv1.ListGameSessionsRequest{CampaignId: id}))
	methods := playv1.File_meurpg_play_v1_play_proto.Services().ByName("PlayService").Methods()
	if len(calls) != methods.Len() {
		t.Errorf("called %d methods, the service has %d", len(calls), methods.Len())
	}
	for name, err := range calls {
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Errorf("%s signed out: error = %v, want unauthenticated", name, err)
		}
		if ce, ok := errors.AsType[*connect.Error](err); !ok || !slices.Equal(ce.Meta().Values("Cache-Control"), []string{"no-store"}) {
			t.Errorf("%s: error Cache-Control = %q, want no-store, once", name, ce.Meta().Values("Cache-Control"))
		}
	}

	opts, _ := methods.ByName("ListGameSessions").Options().(*descriptorpb.MethodOptions)
	if opts.GetIdempotencyLevel() != descriptorpb.MethodOptions_IDEMPOTENT {
		t.Errorf("ListGameSessions idempotency_level = %v, want IDEMPOTENT", opts.GetIdempotencyLevel())
	}
	target := playv1connect.PlayServiceListGameSessionsProcedure + "?connect=v1&encoding=json&message=" + url.QueryEscape(`{"campaignId":"`+id+`"}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, target, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET ListGameSessions: status = %d, want 405", rec.Code)
	}
}
