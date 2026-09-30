package play

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1/campaignsv1connect"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1/charactersv1connect"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/campaigns"
	"github.com/PuraFome/meuRPG/backend/internal/characters"
	"github.com/PuraFome/meuRPG/backend/internal/identity"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The database tests run against CockroachDB with the real campaigns and
// characters services next to this one, as cmd/api wires them: set
// MEURPG_TEST_DATABASE_URL (see package dbtest). Without it they skip.

var testRules = sync.OnceValues(rules.LoadSRD)

// testUserHeader names the signed-in user in a test request. fakeSessions
// trusts it; production code has no way to set a caller (see
// docs/arquitetura.md, "Quem está chamando").
const testUserHeader = "Test-User-Id"

type fakeSessions struct{}

type testUserKey struct{}

var testSessions Sessions = fakeSessions{}

func (fakeSessions) Interceptor() connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if userID := req.Header().Get(testUserHeader); userID != "" {
				ctx = context.WithValue(ctx, testUserKey{}, userID)
			}
			return next(ctx, req)
		}
	})
}

func (fakeSessions) UserID(ctx context.Context) (string, error) {
	if userID, ok := ctx.Value(testUserKey{}).(string); ok {
		return userID, nil
	}
	return "", connect.NewError(connect.CodeUnauthenticated, errors.New("sign in to continue"))
}

// fakeClock ticks one microsecond each time it is read, so rows created one
// after the other have different times.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(time.Microsecond)
	return c.now
}

type harness struct {
	t      *testing.T
	pool   *pgxpool.Pool
	users  *identity.PostgresStore
	server *httptest.Server
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	pool := dbtest.NewPool(t, "meurpg_play_test")
	h := &harness{t: t, pool: pool, users: identity.NewPostgresStore(pool)}
	clock := &fakeClock{now: time.Now().Truncate(time.Microsecond)}
	logger := slog.New(slog.DiscardHandler)
	content, err := testRules()
	if err != nil {
		t.Fatalf("rules.LoadSRD() error = %v", err)
	}
	camps, err := campaigns.New(campaigns.Config{Pool: pool, Profiles: h.users, Logger: logger, Now: clock.Now})
	if err != nil {
		t.Fatalf("campaigns.New() error = %v", err)
	}
	chars, err := characters.New(characters.Config{Pool: pool, Profiles: h.users, Members: camps, Rules: content, Logger: logger, Now: clock.Now})
	if err != nil {
		t.Fatalf("characters.New() error = %v", err)
	}
	svc, err := New(Config{Pool: pool, Sheets: chars, Logger: logger, Now: clock.Now})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	mux := http.NewServeMux()
	opt := connect.WithRequireConnectProtocolHeader()
	camps.Mount(mux.Handle, testSessions, opt)
	chars.Mount(mux.Handle, testSessions, camps, opt)
	svc.Mount(mux.Handle, testSessions, camps, opt)
	h.server = httptest.NewServer(mux)
	t.Cleanup(h.server.Close)
	return h
}

type user struct {
	id         string
	campaigns  campaignsv1connect.CampaignServiceClient
	characters charactersv1connect.CharacterServiceClient
	play       playv1connect.PlayServiceClient
}

func (h *harness) newUser(displayName string) *user {
	h.t.Helper()
	id, err := h.users.UpsertUser(h.t.Context(), identity.ExternalIdentity{Issuer: "https://idp.test", Subject: rand.Text()})
	if err != nil {
		h.t.Fatalf("UpsertUser() error = %v", err)
	}
	if err := h.users.SetDisplayName(h.t.Context(), id, displayName); err != nil {
		h.t.Fatalf("SetDisplayName() error = %v", err)
	}
	return h.clients(id)
}

func (h *harness) anonymous() *user { return h.clients("") }

func (h *harness) clients(userID string) *user {
	var opts []connect.ClientOption
	if userID != "" {
		opts = append(opts, connect.WithInterceptors(connect.UnaryInterceptorFunc(
			func(next connect.UnaryFunc) connect.UnaryFunc {
				return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
					req.Header().Set(testUserHeader, userID)
					return next(ctx, req)
				}
			})))
	}
	c, url := h.server.Client(), h.server.URL
	return &user{
		id:         userID,
		campaigns:  campaignsv1connect.NewCampaignServiceClient(c, url, opts...),
		characters: charactersv1connect.NewCharacterServiceClient(c, url, opts...),
		play:       playv1connect.NewPlayServiceClient(c, url, opts...),
	}
}

// newCampaign creates a campaign whose master is master, lets the players
// in, and returns its ID.
func (h *harness) newCampaign(master *user, name string, players ...*user) string {
	h.t.Helper()
	ctx := h.t.Context()
	res, err := master.campaigns.CreateCampaign(ctx, connect.NewRequest(&campaignsv1.CreateCampaignRequest{Name: name, XpMode: campaignsv1.XpMode_XP_MODE_MILESTONES}))
	if err != nil {
		h.t.Fatalf("CreateCampaign() error = %v", err)
	}
	id := res.Msg.GetCampaign().GetId()
	h.join(master, id, players...)
	return id
}

func (h *harness) join(master *user, campaignID string, players ...*user) {
	h.t.Helper()
	ctx := h.t.Context()
	inv, err := master.campaigns.CreateInvite(ctx, connect.NewRequest(&campaignsv1.CreateInviteRequest{CampaignId: campaignID, MaxUses: campaigns.MaxInviteUses}))
	if err != nil {
		h.t.Fatalf("CreateInvite() error = %v", err)
	}
	for _, p := range players {
		if _, err := p.campaigns.AcceptInvite(ctx, connect.NewRequest(&campaignsv1.AcceptInviteRequest{Token: inv.Msg.GetToken()})); err != nil {
			h.t.Fatalf("AcceptInvite() error = %v", err)
		}
	}
}

// start calls StartGameSession as u, or fails the test.
func (u *user) start(t *testing.T, campaignID string) *playv1.StartGameSessionResponse {
	t.Helper()
	res, err := u.play.StartGameSession(t.Context(), connect.NewRequest(&playv1.StartGameSessionRequest{CampaignId: campaignID}))
	if err != nil {
		t.Fatalf("StartGameSession() error = %v", err)
	}
	return res.Msg
}

// end calls EndGameSession as u, or fails the test.
func (u *user) end(t *testing.T, s *playv1.GameSession) *playv1.GameSession {
	t.Helper()
	res, err := u.play.EndGameSession(t.Context(), connect.NewRequest(&playv1.EndGameSessionRequest{CampaignId: s.GetCampaignId(), GameSessionId: s.GetId()}))
	if err != nil {
		t.Fatalf("EndGameSession() error = %v", err)
	}
	return res.Msg.GetGameSession()
}

// createCharacter creates a character as u, or fails the test.
func (u *user) createCharacter(t *testing.T, campaignID string, kind charactersv1.CharacterKind, name string) *charactersv1.Character {
	t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8},
		RaceKey:    "race:gnome",
		Classes:    []*charactersv1.ClassLevel{{ClassKey: "class:wizard", Level: 1}},
	}}}
	if kind == charactersv1.CharacterKind_CHARACTER_KIND_MINION {
		sheet = &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Basic{Basic: &charactersv1.BasicSheet{HitPointsMax: 7, ArmorClass: 12, SpeedFt: 30}}}
	}
	res, err := u.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{CampaignId: campaignID, Kind: kind, Name: name, Sheet: sheet}))
	if err != nil {
		t.Fatalf("CreateCharacter(%v) error = %v", kind, err)
	}
	return res.Msg.GetCharacter()
}

// character reads a character as u, or fails the test.
func (u *user) character(t *testing.T, c *charactersv1.Character) *charactersv1.Character {
	t.Helper()
	res, err := u.characters.GetCharacter(t.Context(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: c.GetCampaignId(), CharacterId: c.GetId()}))
	if err != nil {
		t.Fatalf("GetCharacter() error = %v", err)
	}
	return res.Msg.GetCharacter()
}

func wantCode(t *testing.T, call string, err error, want connect.Code) {
	t.Helper()
	if got := connect.CodeOf(err); err == nil || got != want {
		t.Fatalf("%s error = %v, want %v", call, err, want)
	}
}
