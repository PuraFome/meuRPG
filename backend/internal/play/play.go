// Package play is about playing a campaign at the table: it starts, ends
// and lists game sessions (PlayService), which is what locks the players'
// sheets (RN-01, MR-006, MR-011), and runs the live session (Etapa 5,
// live.go): the in-app notice that a session is open (RN-06), the
// session's live stream (ADR-0005), the master's correction of the
// characters' vitals (RN-02), each one recorded in session_events
// (ADR-0007), and what the session shows at the table: the current map and
// a gallery image (onscreen.go). Turns and actions come with combat (Etapa
// 6).
//
// Starting a session locks the sheets in the same transaction that opens
// it, through a SheetLocker, and the vitals live in the characters module
// too, reached through a VitalsKeeper: the characters module owns the
// characters tables, so this package never touches them. The campaigns a
// user belongs to come from the campaigns module (CampaignDirectory). The
// maps module checks and reveals the map the master makes current, and
// reads the gallery image the master shows (MapKeeper); the other way
// round, it reads what the session shows and publishes its changes on the
// live stream through OnScreen and Publish (its maps.LiveSession
// interface). cmd/api connects them; no package imports
// another's code.
//
// The SQL lives in queries.sql, and sqlc turns it into package playdb.
// Every write runs inside db.InTx, which retries CockroachDB's serialization
// errors (40001).
package play

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/platform/nostore"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// SheetLocker locks a campaign's player sheets when a game session starts
// (RN-01). The characters module implements it
// (characters.Service.LockSheets).
type SheetLocker interface {
	// LockSheets locks, inside tx, the sheets of the campaign's living
	// player characters that are still drafts, ends every permission to
	// edit a story in the campaign, and returns how many sheets it locked.
	LockSheets(ctx context.Context, tx pgx.Tx, campaignID string, at time.Time) (int64, error)
}

// VitalsKeeper keeps the player characters' vitals: hit points, spell
// slots, hit dice (RN-02, table character_vitals). The characters module
// implements it (characters.Service), because the vitals belong to the
// character and last from one session to the next; this package declares
// the messages that go through it (playv1.CharacterVitals), and serves them
// live. The methods take no caller: they run after this package's own
// authorization check, and their errors are Connect errors to return as
// they are.
type VitalsKeeper interface {
	// ListVitals returns the vitals of the campaign's living, active player
	// characters, oldest first.
	ListVitals(ctx context.Context, campaignID string) ([]*playv1.CharacterVitals, error)
	// GetVitals returns one living, active player character's vitals, or
	// `not_found`.
	GetVitals(ctx context.Context, campaignID, characterID string) (*playv1.CharacterVitals, error)
	// GetVitalsTx is GetVitals inside tx.
	GetVitalsTx(ctx context.Context, tx pgx.Tx, campaignID, characterID string) (*playv1.CharacterVitals, error)
	// AdjustVitals applies the vitals fields of req inside tx, and returns
	// the vitals before and after: `not_found` for anything but a living,
	// active player character of the campaign; `invalid_argument` for a
	// value outside 0 to its maximum.
	AdjustVitals(ctx context.Context, tx pgx.Tx, campaignID, characterID string, req *playv1.AdjustCharacterVitalsRequest) (before, after *playv1.CharacterVitals, err error)
}

// MapKeeper is what the session's screen needs from the maps module
// (maps.SessionMaps), whose tables the maps and the gallery images are: it
// checks and reveals the map the master makes current (SetCurrentMap), and
// reads the gallery image the master shows (SetShownImage), and keeps the
// images the master leaves with the players.
type MapKeeper interface {
	// RevealMap reveals the campaign's map inside tx (a revealed map stays
	// as it is), or returns a `not_found` Connect error when mapID is not a
	// map of the campaign.
	RevealMap(ctx context.Context, tx pgx.Tx, campaignID, mapID string, at time.Time) error
	// ShownImage returns the campaign's gallery image imageID as the
	// session shows it, or a `not_found` Connect error when it is not an
	// image of the campaign's gallery.
	ShownImage(ctx context.Context, campaignID, imageID string) (*playv1.ShownImage, error)
	// LeaveImage adds the campaign's image to the images left with the
	// players inside tx. An image already left stays as it is, and one that
	// is not the campaign's anymore is skipped.
	LeaveImage(ctx context.Context, tx pgx.Tx, campaignID, imageID string, at time.Time) error
	// ListLeftImages returns the images left with the players, oldest
	// first.
	ListLeftImages(ctx context.Context, campaignID string) ([]*playv1.ShownImage, error)
	// TakeBackImage removes the image from the left list, or returns a
	// `not_found` Connect error when it is not on it.
	TakeBackImage(ctx context.Context, campaignID, imageID string) error
	// MapGrid returns the battle grid of the campaign's map, the zero Grid
	// when it has none, or a `not_found` Connect error (MR-013).
	MapGrid(ctx context.Context, campaignID, mapID string) (link.Grid, error)
	// BattlePoint returns a battle point of the campaign, or a `not_found`
	// Connect error for any other point.
	BattlePoint(ctx context.Context, campaignID, pointID string) (link.BattlePoint, error)
	// ScenePoint returns a SCENE point of the campaign, hidden or not, with its
	// actions and their DCs (MR-015), and the master's hooks and clues (MR-029,
	// never for a player), or a `not_found` Connect error for any other point.
	ScenePoint(ctx context.Context, campaignID, pointID string) (link.Scene, error)
	// DiscoverScene records inside tx that the group discovered the scene (the
	// master opened it, MR-030); one already discovered stays as it is.
	DiscoverScene(ctx context.Context, tx pgx.Tx, campaignID, pointID string, at time.Time) error
	// MapTokens returns where the map's tokens stand, hidden ones included.
	MapTokens(ctx context.Context, mapID string) ([]link.TokenPosition, error)
	// SetTokenPositions moves the characters' tokens on the map inside tx,
	// creating the ones that are missing.
	SetTokenPositions(ctx context.Context, tx pgx.Tx, mapID string, positions []link.TokenPosition, at time.Time) error
}

// CombatRoster tells who can fight and with which numbers (MR-013). The
// characters module implements it (characters.Service), because the sheets
// are its own. The methods take no caller: they run after this package's
// own authorization check.
type CombatRoster interface {
	// CombatParty returns the campaign's living, active player characters,
	// oldest first.
	CombatParty(ctx context.Context, campaignID string) ([]link.Character, error)
	// CombatCharacters returns those of ids that are living characters of
	// the campaign, players' or NPCs; the others are left out.
	CombatCharacters(ctx context.Context, campaignID string, ids []string) ([]link.Character, error)
	// SessionCharacters returns those of ids that are characters of the
	// campaign whatever their status (a dead one too), with only their name and
	// player filled: the session summary names who fought, even if they died.
	SessionCharacters(ctx context.Context, campaignID string, ids []string) ([]link.Character, error)
	// CombatSheet returns what an attack needs from the sheet of a living
	// character of the campaign, a player's or an NPC's: its armor class, its
	// attacks and the standard actions. Its armor class never goes to a
	// player (RN-20). `not_found` for any other character.
	CombatSheet(ctx context.Context, campaignID, characterID string) (link.Sheet, error)
	// CombatTurnOptions works out what the character can do now (MR-014),
	// from its sheet, what it used this turn and the slots it spent: the rules
	// engine's TurnOptions. `not_found` for any other character.
	CombatTurnOptions(ctx context.Context, campaignID, characterID string, turn link.Turn) (*rulesv1.TurnOptions, error)
	// CombatSpell returns the spell as the character casts it with a slot of
	// slotLevel (0 for a cantrip): its range, attack or save, damage or healing
	// at that level, with the character's attack bonus, save DC and
	// spellcasting modifier. It does not check that the character may cast it
	// (CombatTurnOptions does). `not_found` for any other character or spell.
	CombatSpell(ctx context.Context, campaignID, characterID, spellKey string, slotLevel int) (link.Spell, error)
	// CombatSave returns the character's saving throw bonus for an ability
	// ("dex"). A basic-sheet NPC has none: Known is false.
	CombatSave(ctx context.Context, campaignID, characterID, ability string) (link.Save, error)
	// MarkDead marks a player's character dead inside tx, as the master's
	// MarkCharacterDead does (RN-03): the master confirmed its death in a combat.
	// It is idempotent.
	MarkDead(ctx context.Context, tx pgx.Tx, campaignID, characterID string, at time.Time) error
	// SceneOptions returns, for each key of a scene's checks, the character's
	// bonus and passive value (the rules engine's SceneOptions), in the order of
	// keys. `not_found` for any other character.
	SceneOptions(ctx context.Context, campaignID, characterID string, keys []string) ([]link.SceneOption, error)
	// SceneCheckName is the Portuguese name of a scene check by its key, "" for
	// an unknown one.
	SceneCheckName(key string) string
	// Conditions lists the SRD's conditions (RN-22), with their Portuguese
	// names, sorted by key.
	Conditions() []link.Named
	// NamePT is the Portuguese name of a content key ("spell:shield" is "Escudo
	// Arcano"), or "" for an unknown key.
	NamePT(key string) string
}

// DiceForce is what the campaign's dice setting makes a player do (RN-18).
type DiceForce int

const (
	// DiceChoice is the setting "each player chooses", so the player picks
	// on every roll, the app's or a typed result. Their saved preference is only
	// the default the screen highlights.
	DiceChoice DiceForce = iota
	// DiceForcedInApp makes everybody roll in the app, a typed value is refused.
	DiceForcedInApp
	// DiceForcedPhysical makes everybody roll real dice and types the result, the
	// app's roll is refused.
	DiceForcedPhysical
)

// refuses says whether the forced mode does not allow this way of rolling.
func (f DiceForce) refuses(inApp bool) bool {
	return (f == DiceForcedInApp && !inApp) || (f == DiceForcedPhysical && inApp)
}

// DiceModes tells what the campaign's dice setting forces on a player (RN-18).
// cmd/api wires it to campaigns.Service.CampaignDiceMode.
type DiceModes interface {
	// ForcedDice returns what the campaign's setting forces on userID, an
	// active member of the campaign.
	ForcedDice(ctx context.Context, campaignID, userID string) (DiceForce, error)
}

// CampaignDirectory tells which campaigns a user belongs to. The campaigns
// module implements it (campaigns.Service.ActiveCampaigns), so this
// package never reads campaign_members itself.
type CampaignDirectory interface {
	// ActiveCampaigns returns the campaigns userID is an active member of
	// (never a pending one), with their name and the user's role (my_role).
	ActiveCampaigns(ctx context.Context, userID string) ([]*campaignsv1.Campaign, error)
}

// The live stream's timing (docs/operacao.md). Tests shorten them through
// Config.Live.
const (
	// DefaultHeartbeat: a stream with nothing to say sends a heartbeat this
	// often, so the app can tell a dead stream, and proxies and Cloud Run
	// don't take it for an idle one.
	DefaultHeartbeat = 25 * time.Second
	// DefaultRecheck: how often a stream reads the login session and the
	// membership again (ADR-0005).
	DefaultRecheck = 60 * time.Second
	// DefaultMaxLifetime: a stream ends after this long, and the app opens
	// a new one. It bounds what a forgotten tab can hold, and keeps each
	// stream well inside Cloud Run's request timeout.
	DefaultMaxLifetime = 30 * time.Minute
)

// LiveConfig sets the live stream's timing. Zero values mean the defaults.
type LiveConfig struct {
	Heartbeat   time.Duration
	Recheck     time.Duration
	MaxLifetime time.Duration
	// Buffer is how many events a stream may fall behind before it is
	// dropped (live.DefaultBuffer).
	Buffer int
}

// Config holds what the play service needs.
type Config struct {
	// Pool is the CockroachDB connection pool. Required.
	Pool *pgxpool.Pool
	// Sheets locks the players' sheets when a session starts. Required.
	Sheets SheetLocker
	// Vitals keeps the characters' vitals. Required.
	Vitals VitalsKeeper
	// Campaigns lists a user's campaigns, for the session notice. Required.
	Campaigns CampaignDirectory
	// Maps checks and reveals the session's current map, reads the image it
	// shows, and gives a combat its grid and tokens. Required.
	Maps MapKeeper
	// Roster says who can fight, for a combat. Required.
	Roster CombatRoster
	// Dice says where a player rolls (RN-18). Required.
	Dice DiceModes
	// Terrain gives a combat the walls, difficult terrain and cover of its map
	// (RN-21, D2). Optional: cmd/api sets it with SetTerrain once the maps module
	// exists (the two need each other); nil means open floor.
	Terrain TerrainSource
	// Roller rolls the NPCs' dice and the app's rolls. Nil means the
	// operating system's random source (dice.Crypto); tests pass faces.
	Roller dice.Roller
	// Live sets the live stream's timing; the zero value is the defaults.
	Live LiveConfig
	// Logger receives errors, without personal data. Nil means
	// slog.Default().
	Logger *slog.Logger
	// Now returns the current time. Nil means time.Now.
	Now func() time.Time
}

// Service implements the PlayService Connect API.
type Service struct {
	pool      *pgxpool.Pool
	queries   *playdb.Queries
	sheets    SheetLocker
	vitals    VitalsKeeper
	campaigns CampaignDirectory
	maps      MapKeeper
	roster    CombatRoster
	dice      DiceModes
	terrain   TerrainSource
	roller    dice.Roller
	logger    *slog.Logger
	now       func() time.Time
	// conditionNames are the SRD conditions' Portuguese names, by key, for the
	// labels the master marks (RN-22).
	conditionNames map[string]string

	// hub fans the live events out to the open streams, in memory: one
	// server instance only (docs/operacao.md).
	hub  *live.Hub
	live LiveConfig
}

// nameOf is the Portuguese name of a content key the combat shows: an SRD
// condition's, or any other key's (a spell's) from the rules content.
func (s *Service) nameOf(key string) string {
	if n, ok := s.conditionNames[key]; ok {
		return n
	}
	return s.roster.NamePT(key)
}

// The compiler checks that Service implements the handler.
var (
	_ playv1connect.PlayServiceHandler   = (*Service)(nil)
	_ playv1connect.CombatServiceHandler = (*Service)(nil)
)

// New returns a Service.
func New(cfg Config) (*Service, error) {
	switch {
	case cfg.Pool == nil:
		return nil, errors.New("play: a Pool is required")
	case cfg.Sheets == nil:
		return nil, errors.New("play: Sheets is required")
	case cfg.Vitals == nil:
		return nil, errors.New("play: Vitals is required")
	case cfg.Campaigns == nil:
		return nil, errors.New("play: Campaigns is required")
	case cfg.Maps == nil:
		return nil, errors.New("play: Maps is required")
	case cfg.Roster == nil:
		return nil, errors.New("play: Roster is required")
	case cfg.Dice == nil:
		return nil, errors.New("play: Dice is required")
	}
	s := &Service{
		pool:      cfg.Pool,
		queries:   playdb.New(cfg.Pool),
		sheets:    cfg.Sheets,
		vitals:    cfg.Vitals,
		campaigns: cfg.Campaigns,
		maps:      cfg.Maps,
		roster:    cfg.Roster,
		dice:      cfg.Dice,
		terrain:   cfg.Terrain,
		roller:    cfg.Roller,
		logger:    cfg.Logger,
		now:       cfg.Now,
		hub:       live.New(cfg.Live.Buffer),
		live:      cfg.Live,
	}
	s.conditionNames = map[string]string{}
	for _, c := range cfg.Roster.Conditions() {
		s.conditionNames[c.Key] = c.NamePT
	}
	if s.logger == nil {
		s.logger = slog.Default()
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.roller == nil {
		s.roller = dice.Crypto{}
	}
	if s.live.Heartbeat <= 0 {
		s.live.Heartbeat = DefaultHeartbeat
	}
	if s.live.Recheck <= 0 {
		s.live.Recheck = DefaultRecheck
	}
	if s.live.MaxLifetime <= 0 {
		s.live.MaxLifetime = DefaultMaxLifetime
	}
	return s, nil
}

// Close ends every open live stream, and refuses new ones. cmd/api calls it
// when the server's graceful shutdown starts: a stream never finishes on
// its own, so the shutdown would otherwise wait for its whole deadline.
// Each app reconnects, to the next server. Closing twice is fine.
func (s *Service) Close() { s.hub.Close() }

// Sessions is what this package needs to know who is calling: an
// interceptor that finds the caller's session, and the authz.Caller that
// reads it back and, for the live stream, reads the session again
// (authz.SessionRechecker). *identity.Service is the real one; tests pass
// a fake.
type Sessions interface {
	// Interceptor finds the caller's session (from the session cookie).
	Interceptor() connect.Interceptor
	authz.SessionRechecker
}

// Mount registers PlayService and CombatService on a mux. handle is usually
// httpserver.Server.Handle or http.ServeMux.Handle.
//
// sessions tells who is calling (the identity service in production), and
// members tells each caller's role in a campaign (the campaigns service).
// Mount adds, in this order, an interceptor that marks every response
// `Cache-Control: no-store`, the sessions interceptor, and the authz
// interceptor. opts are the Connect options shared by every service.
func (s *Service) Mount(handle func(pattern string, handler http.Handler), sessions Sessions, members authz.MembershipSource, opts ...connect.HandlerOption) {
	// Clip so append copies instead of writing into the caller's array.
	opts = append(slices.Clip(opts), connect.WithInterceptors(
		nostore.Interceptor(),                          // no response is cacheable
		sessions.Interceptor(),                         // who is calling
		authz.Interceptor(sessions, members, s.logger), // what they may do, memoized per request
	))
	handle(playv1connect.NewPlayServiceHandler(s, opts...))
	handle(playv1connect.NewCombatServiceHandler(s, opts...))
}
