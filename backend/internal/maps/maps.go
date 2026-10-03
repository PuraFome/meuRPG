// Package maps is about a campaign's maps and the images they are made of:
// the gallery (MR-019), the images the master uploads to use in maps and in
// the campaign document, and the maps themselves (MR-008, MR-009, MR-012),
// with their points of interest and tokens (MapService, mapservice.go).
//
// Who sees what on a map (RN-10) is decided here, on the server
// (visibility.go): a player never receives a hidden map, point or token,
// not even its ID. Two other modules help, each through a small interface
// that cmd/api connects, so no package imports another's code:
//   - the characters module says which characters may stand on a map as
//     tokens (CharacterDirectory);
//   - the play module says which map is the open session's current one,
//     which the players see too, and which gallery image it shows, and
//     carries the maps' changes to the members watching the session
//     (LiveSession). The other way round, play reveals the map it makes
//     current, and reads the image it shows, through SessionMaps.
//
// An image travels like this:
//
//   - The master uploads it with POST /uploads/images (upload.go), a plain
//     HTTP route, because its body is a file. Package images checks it and
//     encodes it again, which drops every piece of metadata. The image and
//     its thumbnail go to the blob store (package platform/blob), then a
//     gallery_images row records them, inside the transaction that checks
//     the campaign's quota.
//   - The campaign's master fetches it with GET /images/{id} (serve.go); a
//     player too, but only while they see it: on a map they see, or shown
//     in the session (RN-10). Knowing its ID is not enough.
//   - The master lists, renames and deletes images with GalleryService
//     (gallery.go), a Connect service like the others.
//
// The SQL lives in queries.sql, and sqlc turns it into package mapsdb.
// Every write runs inside db.InTx, which retries CockroachDB's serialization
// errors (40001).
package maps

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

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1/mapsv1connect"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/blob"
	"github.com/PuraFome/meuRPG/backend/internal/platform/nostore"
)

// A campaign's gallery limits (a proposal, question 30 of the progress
// doc). The per-image limits are in package images.
const (
	// DefaultMaxImages is how many images a campaign may have.
	DefaultMaxImages = 300
	// DefaultMaxBytes is how many bytes its images may add up to, as
	// stored: 500 MiB.
	DefaultMaxBytes = 500 << 20
)

// The HTTP routes, next to the Connect service.
const (
	// UploadPath receives uploads (POST).
	UploadPath = "/uploads/images"
	// ImagesPath serves images (GET ImagesPath+id, and +"/thumb").
	ImagesPath = "/images/"
)

// maxNameLength is the longest image, map or point name, in characters.
// The gallery_images_name_length, maps_name_length and
// map_points_name_length CHECKs say the same.
const maxNameLength = 80

// CharacterDirectory tells which characters may stand on a map as tokens.
// The characters module implements it (characters.Service.MapCharacters),
// so this package never reads the characters table.
type CharacterDirectory interface {
	// MapCharacters returns those of ids that are living characters of the
	// campaign (a player's that is neither dead nor waiting for approval, or
	// an NPC), players' characters first, then NPCs, each group oldest
	// first. Only id, kind, name and player_user_id are set.
	MapCharacters(ctx context.Context, campaignID string, ids []string) ([]*charactersv1.CharacterSummary, error)
	// ClearPortraits takes the image off the portrait of every NPC of the
	// campaign that has it, inside tx, and returns how many it cleared
	// (MR-031): the master deleted the image from the gallery.
	ClearPortraits(ctx context.Context, tx pgx.Tx, campaignID, imageID string) (int64, error)
}

// LiveSession is what this package needs from the live session. The play
// module implements it (play.Service), because the sessions and their
// stream are its own. The events are play's API messages (playv1), as the
// vitals are for the characters module: this package builds them, and
// never imports package play.
type LiveSession interface {
	// OnScreen returns what the campaign's open game session shows: the
	// IDs of its current map, which a player sees even if it is hidden
	// (RN-10), and of the gallery image the master shows the players
	// (MR-028). Each is "" when there is none, both when no session is
	// open.
	OnScreen(ctx context.Context, campaignID string) (currentMapID, shownImageID string, err error)
	// Publish sends ev to the streams of the campaign's master and, when
	// players is true, of its players too. Without an open session nobody
	// is watching, and nothing happens.
	Publish(campaignID string, players bool, ev *playv1.WatchGameSessionResponse)
	// OpenScenePoint returns the map point of the RP scene open in the
	// campaign's open game session (MR-015), "" when none is open or no
	// session is.
	OpenScenePoint(ctx context.Context, campaignID string) (pointID string, err error)
	// ImageOnStage reports whether the image is the portrait of an NPC on the
	// stage of the open scene of the campaign's open game session (MR-031).
	// False when no session or no scene is open.
	ImageOnStage(ctx context.Context, campaignID, imageID string) (bool, error)
}

// Config holds what the maps service needs.
type Config struct {
	// Pool is the CockroachDB connection pool. Required.
	Pool *pgxpool.Pool
	// Blobs stores the image files. Nil means images are off on this
	// server: uploads, downloads and GalleryService answer `unavailable`.
	// MapService works without it, but no image can be uploaded, so no map
	// can be created.
	Blobs blob.Store
	// Characters says which characters may stand on a map. Required.
	Characters CharacterDirectory
	// Live is the live session: the current map, and where map changes
	// go. Required.
	Live LiveSession
	// Rules says which checks a scene may ask for (MR-015): *rules.Content.
	// Required.
	Rules SceneChecks
	// Logger receives errors, without personal data. Nil means
	// slog.Default().
	Logger *slog.Logger
	// Now returns the current time. Nil means time.Now.
	Now func() time.Time
	// MaxImages and MaxBytes are a campaign's gallery quota. Zero means
	// DefaultMaxImages and DefaultMaxBytes; tests set small ones.
	MaxImages int32
	MaxBytes  int32
	// MaxMaps and MaxPointsPerMap bound a campaign's maps and a map's
	// points. Zero means DefaultMaxMaps and DefaultMaxPointsPerMap.
	MaxMaps         int32
	MaxPointsPerMap int32
}

// Service implements GalleryService, MapService and the image routes.
type Service struct {
	pool       *pgxpool.Pool
	queries    *mapsdb.Queries
	blobs      blob.Store
	characters CharacterDirectory
	live       LiveSession
	checks     SceneChecks
	logger     *slog.Logger
	now        func() time.Time
	maxImages  int32
	maxBytes   int32
	maxMaps    int32
	maxPoints  int32

	// processing lets one image at a time be decoded, so a few uploads at
	// once cannot take all the server's memory (package images bounds
	// what one image may use).
	processing chan struct{}
}

// The compiler checks that Service implements both handlers.
var (
	_ mapsv1connect.GalleryServiceHandler = (*Service)(nil)
	_ mapsv1connect.MapServiceHandler     = (*Service)(nil)
)

// New returns a Service.
func New(cfg Config) (*Service, error) {
	switch {
	case cfg.Pool == nil:
		return nil, errors.New("maps: a Pool is required")
	case cfg.Characters == nil:
		return nil, errors.New("maps: Characters is required")
	case cfg.Live == nil:
		return nil, errors.New("maps: Live is required")
	case cfg.Rules == nil:
		return nil, errors.New("maps: Rules is required")
	}
	s := &Service{
		pool:       cfg.Pool,
		queries:    mapsdb.New(cfg.Pool),
		blobs:      cfg.Blobs,
		characters: cfg.Characters,
		live:       cfg.Live,
		checks:     cfg.Rules,
		logger:     cfg.Logger,
		now:        cfg.Now,
		maxImages:  cfg.MaxImages,
		maxBytes:   cfg.MaxBytes,
		maxMaps:    cfg.MaxMaps,
		maxPoints:  cfg.MaxPointsPerMap,
		processing: make(chan struct{}, 1),
	}
	if s.logger == nil {
		s.logger = slog.Default()
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.maxImages <= 0 {
		s.maxImages = DefaultMaxImages
	}
	if s.maxBytes <= 0 {
		s.maxBytes = DefaultMaxBytes
	}
	if s.maxMaps <= 0 {
		s.maxMaps = DefaultMaxMaps
	}
	if s.maxPoints <= 0 {
		s.maxPoints = DefaultMaxPointsPerMap
	}
	return s, nil
}

// Sessions is what this package needs to know who is calling: the Connect
// interceptor and its twin for plain HTTP routes, which both find the
// caller's session, and the authz.Caller that reads it back.
// *identity.Service is the real one; tests pass a fake.
type Sessions interface {
	// Interceptor finds the caller's session (from the session cookie), for
	// the Connect service.
	Interceptor() connect.Interceptor
	// AuthenticateRequest does the same for a plain HTTP request: it
	// returns the request's context with the caller's session in it.
	AuthenticateRequest(r *http.Request) (context.Context, error)
	authz.Caller
}

// Mount registers GalleryService, MapService and the image routes on a mux. handle is
// usually httpserver.Server.Handle or http.ServeMux.Handle.
//
// sessions tells who is calling (the identity service in production), and
// members tells each caller's role in a campaign (the campaigns service).
// The Connect service gets, in this order, an interceptor that marks every
// response `Cache-Control: no-store`, the sessions interceptor, and the
// authz interceptor; the HTTP routes get the same two checks as
// middleware. opts are the Connect options shared by every service.
func (s *Service) Mount(handle func(pattern string, handler http.Handler), sessions Sessions, members authz.MembershipSource, opts ...connect.HandlerOption) {
	// Clip so append copies instead of writing into the caller's array.
	opts = append(slices.Clip(opts), connect.WithInterceptors(
		nostore.Interceptor(),                          // no response is cacheable
		sessions.Interceptor(),                         // who is calling
		authz.Interceptor(sessions, members, s.logger), // what they may do, memoized per request
	))
	handle(mapsv1connect.NewGalleryServiceHandler(s, opts...))
	handle(mapsv1connect.NewMapServiceHandler(s, opts...))

	withAuthz := authz.Middleware(sessions, members, s.logger)
	route := func(h http.HandlerFunc) http.Handler {
		return s.imagesOn(s.withSession(sessions, withAuthz(h)))
	}
	handle("POST "+UploadPath, route(s.handleUpload))
	handle("GET "+ImagesPath+"{id}", route(s.handleImage))
	handle("GET "+ImagesPath+"{id}/thumb", route(s.handleThumbnail))
}

// imagesOn answers 503 while images are off (no blob store), before
// anything else: there is nothing to check a session for.
func (s *Service) imagesOn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.blobs == nil {
			s.writeError(w, r, errImagesOff())
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withSession finds the caller's session, like the Connect interceptor.
func (s *Service) withSession(sessions Sessions, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, err := sessions.AuthenticateRequest(r)
		if err != nil {
			s.writeError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// imageURL and thumbnailURL are where an image and its thumbnail are
// served.
func imageURL(id string) string     { return ImagesPath + id }
func thumbnailURL(id string) string { return ImagesPath + id + "/thumb" }

// blobKeys are the keys of an image's files in the blob store. The
// campaign comes first, so all of a campaign's files share a prefix.
func blobKeys(campaignID, imageID string) (image, thumbnail string) {
	image = "campaigns/" + campaignID + "/images/" + imageID
	return image, image + ".thumb"
}

// deleteFiles deletes an image's files, for an image whose row is gone or
// was never written. It goes on even if the request was canceled, and a
// failure only leaves unreachable files behind, so it is logged, not
// returned.
func (s *Service) deleteFiles(ctx context.Context, campaignID, imageID string) {
	ctx = context.WithoutCancel(ctx)
	image, thumbnail := blobKeys(campaignID, imageID)
	for _, key := range []string{image, thumbnail} {
		if err := s.blobs.Delete(ctx, key); err != nil {
			s.logger.ErrorContext(ctx, "maps: cannot delete an image file; it is left behind", "error", err)
		}
	}
}

// errImagesOff is the answer while no blob store is configured.
func errImagesOff() error {
	return connect.NewError(connect.CodeUnavailable, errors.New("images are not configured on this server"))
}
