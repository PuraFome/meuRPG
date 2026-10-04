// Command api is MeuRPG's HTTP server: it serves the Connect RPCs used by the
// web app plus the /healthz and /readyz probes.
//
// Configuration comes from the environment (see internal/platform/config):
//
//	PORT                listen port (default 8080)
//	DATABASE_URL        CockroachDB connection string (optional)
//	LOG_LEVEL           debug, info, warn or error (default info)
//	OIDC_ISSUER         sign-in provider, e.g. https://accounts.google.com (optional)
//	OIDC_CLIENT_ID      this app's client ID at the provider
//	OIDC_CLIENT_SECRET  this app's client secret at the provider
//	OIDC_REDIRECT_URL   https://<this server>/auth/callback
//	OIDC_CA_FILE        extra CA certificates (PEM) to trust, for a local provider (optional)
//	OIDC_MAX_AGE        max_age sent to the provider, e.g. 1h (optional)
//	BLOB_DIR            directory for uploaded images (optional)
//
// Sign-in needs both the OIDC_* variables and DATABASE_URL. Without them
// the API still starts, and the sign-in routes answer 503. CampaignService,
// CampaignDocumentService, CharacterService, ContentService, PlayService,
// ProgressionService,
// GalleryService and MapService need sign-in too; without it, they are not
// mounted. Images also need BLOB_DIR: without it, the image routes and
// GalleryService answer 503 (unavailable), and the rest works (MapService
// too, but no image can be uploaded, so no map can be created).
//
// The rules content (the SRD 5.1 snapshot, package rules) is embedded in
// the binary and loaded at startup, always: a broken snapshot stops the
// server right away instead of failing on a player's sheet.
//
// The modules meet here and nowhere else: campaigns, characters, play and
// maps learn who is calling from identity (the authz.Caller interface; the
// live stream also reads the session again, authz.SessionRechecker), and
// each member's role from campaigns (authz.MembershipSource); identity
// completes the "accept this invite" sign-in intent through campaigns (an
// identity.IntentHandler); maps stores images in the blob store
// (platform/blob), learns who is calling on its plain HTTP routes from
// identity too (maps.Sessions), finds the characters that may stand on a
// map through characters (maps.CharacterDirectory), and reads the session's
// current map and shown image and publishes map changes on the live stream
// through play (maps.LiveSession); play locks the players' sheets and keeps the
// characters' vitals through characters (play.SheetLocker,
// play.VitalsKeeper), lists a user's campaigns through campaigns
// (play.CampaignDirectory), and reveals the map it makes current and reads
// the image it shows through maps (play.MapKeeper, maps.SessionMaps); and
// characters settles a pending
// member's membership through campaigns when the master approves or rejects
// their character (characters.PendingMembers, RN-15), and campaigns approves
// a pending member's character through characters when an invite without
// approval promotes them (campaigns.Characters, Q25). No package imports
// another's internals.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"connectrpc.com/connect"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PuraFome/meuRPG/backend/gen/meurpg/system/v1/systemv1connect"
	"github.com/PuraFome/meuRPG/backend/internal/campaigns"
	"github.com/PuraFome/meuRPG/backend/internal/characters"
	"github.com/PuraFome/meuRPG/backend/internal/identity"
	"github.com/PuraFome/meuRPG/backend/internal/maps"
	"github.com/PuraFome/meuRPG/backend/internal/notes"
	"github.com/PuraFome/meuRPG/backend/internal/platform/blob"
	"github.com/PuraFome/meuRPG/backend/internal/platform/config"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/httpserver"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
	"github.com/PuraFome/meuRPG/backend/internal/play"
	"github.com/PuraFome/meuRPG/backend/internal/progression"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/system"
)

// Build information, replaced at build time with:
//
//	go build -ldflags "-X main.version=v0.1.0 -X main.commit=$(git rev-parse HEAD)"
var (
	version = "dev"
	commit  = "unknown"
)

// maxRequestBytes caps the size of a single RPC message the server will
// read, so a client cannot exhaust memory with one huge request.
const maxRequestBytes = 4 << 20 // 4 MiB

func main() {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		// The configured log level is unknown here, so use the default one.
		logging.New(os.Stdout, config.DefaultLogLevel).Error("cannot start", "error", err)
		os.Exit(2)
	}
	logger := logging.New(os.Stdout, cfg.LogLevel)

	if err := run(logger, cfg); err != nil {
		logger.Error("api stopped with an error", "error", err)
		os.Exit(1)
	}
}

// run wires the application together. Keeping it apart from main means
// deferred cleanups (closing the pool) run before the process exits.
func run(logger *slog.Logger, cfg config.Config) error {
	// Cancel ctx on Ctrl+C (SIGINT) or SIGTERM, which is how Cloud Run asks
	// a container to stop. Canceling ctx starts the graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// After the first signal, restore the default behavior so a second
	// Ctrl+C kills the process right away instead of waiting for the drain.
	context.AfterFunc(ctx, stop)

	logger.Info("starting api", "version", version, "commit", commit)

	// Loading the rules content checks every entry and compiles every
	// formula (ADR-0008). An error is a bug in the embedded data, so stop.
	rulesContent, err := rules.LoadSRD()
	if err != nil {
		return fmt.Errorf("load the rules content: %w", err)
	}
	logger.Info("rules content loaded", "version", rulesContent.Version())

	// database stays a nil interface when DATABASE_URL is empty. It must not
	// hold a nil *pgxpool.Pool: an interface holding a typed nil pointer is
	// not == nil, and /readyz would then call Ping on it.
	var database httpserver.Pinger
	var pool *pgxpool.Pool
	if cfg.DatabaseURL == "" {
		logger.Warn("DATABASE_URL is not set; running without a database")
	} else {
		pool, err = db.NewPool(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()

		// A failed ping is not fatal: the database may come up later, and
		// until it does /readyz reports it as down.
		if err := pool.Ping(ctx); err != nil {
			logger.Warn("database is not reachable yet", "error", err)
		}
		database = pool
	}

	// Images need somewhere to live. blobs stays a nil interface without
	// BLOB_DIR (never a nil *blob.FS, for the reason given for database
	// above), and the maps module then answers 503 for images.
	var blobs blob.Store
	if cfg.BlobDir == "" {
		logger.Warn("BLOB_DIR is not set; images are off: uploads, image downloads and the gallery answer 503")
	} else {
		fs, err := blob.NewFS(cfg.BlobDir)
		if err != nil {
			return err
		}
		defer func() { _ = fs.Close() }()
		blobs = fs
		logger.Info("images are stored on disk", "dir", cfg.BlobDir)
	}

	// Sign-in needs a provider and a database. identityService stays nil
	// when either is missing, and the sign-in routes then answer 503.
	// Campaigns, characters and game sessions need to know who is calling,
	// so they come with sign-in.
	var identityService *identity.Service
	var campaignsService *campaigns.Service
	var charactersService *characters.Service
	var playService *play.Service
	var mapsService *maps.Service
	var progressionService *progression.Service
	var notesService *notes.Service
	switch {
	case !cfg.OIDC.Configured():
		logger.Warn("OIDC_ISSUER is not set; sign-in is disabled")
	case pool == nil:
		logger.Warn("sign-in is disabled: it needs DATABASE_URL")
	default:
		users := identity.NewPostgresStore(pool)
		campaignsService, err = campaigns.New(campaigns.Config{
			Pool:     pool,
			Profiles: users, // display names come from the identity module
			Logger:   logger,
		})
		if err != nil {
			return err
		}
		charactersService, err = characters.New(characters.Config{
			Pool:     pool,
			Profiles: users,
			Members:  campaignsService, // approving or rejecting a character settles the membership (RN-15)
			Rules:    rulesContent,
			Logger:   logger,
		})
		if err != nil {
			return err
		}
		// campaigns and characters need each other too, so campaigns gets
		// characters now that it exists: an invite without approval accepted
		// by a pending member approves their character (RN-15).
		campaignsService.SetCharacters(charactersService)
		// maps.SessionMaps needs nothing but the database, so characters gets
		// it now too: an NPC's portrait must be an image of the campaign's
		// gallery (MR-031).
		sessionMaps := maps.NewSessionMaps(pool)
		charactersService.SetGallery(sessionMaps)
		// play and maps need each other: play reveals the map it makes
		// current and reads the image it shows, and maps reads what the
		// session shows and publishes on play's live stream. play gets the
		// SessionMaps made above first, and maps then gets play.
		playService, err = play.New(play.Config{
			Pool:      pool,
			Sheets:    charactersService,           // starting a session locks the sheets (RN-01)
			Vitals:    charactersService,           // the characters' hit points, slots and hit dice (RN-02)
			Campaigns: campaignsService,            // the caller's campaigns, for the session notice (RN-06)
			Maps:      sessionMaps,                 // the current map (RN-10), the shown image (MR-028), the grid and tokens (MR-013)
			Roster:    charactersService,           // who can fight, with which numbers (MR-013)
			Dice:      diceModes{campaignsService}, // where a player rolls (RN-18)
			Logger:    logger,
		})
		if err != nil {
			return err
		}
		mapsService, err = maps.New(maps.Config{
			Pool:       pool,
			Blobs:      blobs,             // nil: images are off
			Characters: charactersService, // the characters that may stand on a map (MR-012)
			Live:       playService,       // the current map, and where map changes go (RN-10)
			Rules:      rulesContent,      // which checks an RP scene may ask for (MR-015)
			Logger:     logger,
		})
		if err != nil {
			return err
		}
		// progression and characters need each other too (the XP lives on the
		// sheets, and a sheet shows "pode subir de nível"): characters gets
		// progression once it exists.
		progressionService, err = progression.New(progression.Config{
			Pool:      pool,
			Party:     charactersService, // the party, and the XP on the sheets (MR-016)
			Combats:   playService,       // a combat's defeated NPCs and their XP
			Log:       playService,       // the session's history and the xp_changed hint
			Campaigns: campaignsService,  // how the campaign levels (RN-09)
			Profiles:  users,             // who gave each award
			Logger:    logger,
		})
		if err != nil {
			return err
		}
		charactersService.SetLevelUps(progressionService)
		// The players' private notes (MR-030) read the scenes the group
		// discovered and the clues revealed to each player from the maps
		// module's tables, through SessionMaps.
		notesService, err = notes.New(notes.Config{
			Pool:   pool,
			Scenes: sessionMaps, // the discovered scenes and the received clues (MR-029, MR-030)
			Logger: logger,
		})
		if err != nil {
			return err
		}
		identityService, err = identity.New(ctx, identity.Config{
			OIDC:   cfg.OIDC,
			Store:  users,
			Logger: logger,
			// On Cloud Run the sign-in rate limit reads the client IP from
			// X-Forwarded-For; anywhere else, from the connection.
			BehindCloudRun: cfg.CloudRun,
			// What a user may ask to finish right after signing in
			// (POST /auth/login): today, accepting a campaign invite.
			Intents: map[string]identity.IntentHandler{
				campaigns.InviteIntentKind: campaignsService.InviteIntent(),
			},
		})
		if err != nil {
			return err
		}
		// The issuer and client ID are not secret; the client secret is
		// never logged (config.Secret prints as [REDACTED] anyway).
		logger.Info("sign-in is enabled", "issuer", cfg.OIDC.IssuerURL, "client_id", cfg.OIDC.ClientID, "max_age", cfg.OIDC.MaxAge.String())
	}

	srv := httpserver.New(httpserver.Config{
		Addr:   ":" + strconv.Itoa(cfg.Port),
		Logger: logger,
		DB:     database,
	})

	connectOpts := []connect.HandlerOption{
		connect.WithReadMaxBytes(maxRequestBytes),
		// Unary Connect requests must carry the Connect-Protocol-Version
		// header (or connect=v1 in a GET's query). A browser cannot add that
		// header to a cross-origin request without a CORS preflight, which
		// this server never grants: one more layer against CSRF on top of
		// SameSite cookies and http.CrossOriginProtection.
		connect.WithRequireConnectProtocolHeader(),
	}
	srv.Handle(systemv1connect.NewSystemServiceHandler(system.NewService(version, commit), connectOpts...))
	if identityService != nil {
		identityService.Mount(srv.Handle, connectOpts...)
		// identityService is who is calling: its interceptor finds the
		// session, and its UserID reads it back (authz.Caller). This mounts
		// CampaignService and the campaign document's
		// CampaignDocumentService (MR-018), with the same interceptors.
		campaignsService.Mount(srv.Handle, identityService, connectOpts...)
		// campaignsService says who belongs to each campaign, and with which
		// role (authz.MembershipSource).
		charactersService.Mount(srv.Handle, identityService, campaignsService, connectOpts...)
		playService.Mount(srv.Handle, identityService, campaignsService, connectOpts...)
		progressionService.Mount(srv.Handle, identityService, campaignsService, connectOpts...)
		notesService.Mount(srv.Handle, identityService, campaignsService, connectOpts...)
		// GalleryService and MapService, plus the upload and download
		// routes, which find the session with
		// identityService.AuthenticateRequest.
		mapsService.Mount(srv.Handle, identityService, campaignsService, connectOpts...)
		// Live streams never end on their own: end them when the graceful
		// shutdown starts, instead of holding it until its deadline.
		srv.OnShutdown(playService.Close)
	} else {
		identity.MountDisabled(srv.Handle, connectOpts...)
		logger.Warn("campaigns, characters, game sessions, images and maps are disabled: they need sign-in")
	}

	// Forms on the app's pages (the invite page's POST to /auth/login) are
	// redirected to the provider, so its origin must pass the CSP's
	// form-action. The issuer and its authorization endpoint share an origin
	// for Google and for the local providers.
	var staticOpts []httpserver.StaticOption
	if cfg.OIDC.Configured() {
		staticOpts = append(staticOpts, httpserver.WithFormActionOrigin(cfg.OIDC.IssuerURL))
	}
	static, ok, err := httpserver.NewStatic(cfg.WebDir, staticOpts...)
	switch {
	case err != nil:
		return err
	case ok:
		srv.Handle("/", static)
		logger.Info("serving the web app", "dir", cfg.WebDir)
	default:
		logger.Info("no web build found; running API-only", "dir", cfg.WebDir)
	}

	return srv.Run(ctx)
}

// diceModes adapts the campaigns service to play's DiceModes: play only needs
// to know what the campaign's setting forces on a player, not campaigns' own types.
type diceModes struct{ campaigns *campaigns.Service }

func (d diceModes) ForcedDice(ctx context.Context, campaignID, userID string) (play.DiceForce, error) {
	mode, err := d.campaigns.CampaignDiceMode(ctx, campaignID, userID)
	switch mode {
	case campaigns.DiceModeApp:
		return play.DiceForcedInApp, err
	case campaigns.DiceModePhysical:
		return play.DiceForcedPhysical, err
	}
	return play.DiceChoice, err
}
