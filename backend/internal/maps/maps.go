// Package maps is about a campaign's maps and the images they are made of.
// Today it holds the gallery (MR-019): the images the master uploads, to
// use in maps and in the campaign document. Maps, their points of interest
// and tokens come next.
//
// An image travels like this:
//
//   - The master uploads it with POST /uploads/images (upload.go), a plain
//     HTTP route, because its body is a file. Package images checks it and
//     encodes it again, which drops every piece of metadata. The image and
//     its thumbnail go to the blob store (package platform/blob), then a
//     gallery_images row records them, inside the transaction that checks
//     the campaign's quota.
//   - Any active member of the campaign fetches it with GET /images/{id}
//     (serve.go). Players included: they only learn an image's ID from a
//     response they may see (RN-10).
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
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1/mapsv1connect"
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

// maxNameLength is the longest image name, in characters. The
// gallery_images_name_length CHECK says the same.
const maxNameLength = 80

// Config holds what the maps service needs.
type Config struct {
	// Pool is the CockroachDB connection pool. Required.
	Pool *pgxpool.Pool
	// Blobs stores the image files. Nil means images are off on this
	// server: uploads, downloads and GalleryService answer `unavailable`.
	Blobs blob.Store
	// Logger receives errors, without personal data. Nil means
	// slog.Default().
	Logger *slog.Logger
	// Now returns the current time. Nil means time.Now.
	Now func() time.Time
	// MaxImages and MaxBytes are a campaign's gallery quota. Zero means
	// DefaultMaxImages and DefaultMaxBytes; tests set small ones.
	MaxImages int32
	MaxBytes  int32
}

// Service implements GalleryService and the image routes.
type Service struct {
	pool      *pgxpool.Pool
	queries   *mapsdb.Queries
	blobs     blob.Store
	logger    *slog.Logger
	now       func() time.Time
	maxImages int32
	maxBytes  int32

	// processing lets one image at a time be decoded, so a few uploads at
	// once cannot take all the server's memory (package images bounds
	// what one image may use).
	processing chan struct{}
}

// The compiler checks that Service implements the handler.
var _ mapsv1connect.GalleryServiceHandler = (*Service)(nil)

// New returns a Service.
func New(cfg Config) (*Service, error) {
	if cfg.Pool == nil {
		return nil, errors.New("maps: a Pool is required")
	}
	s := &Service{
		pool:       cfg.Pool,
		queries:    mapsdb.New(cfg.Pool),
		blobs:      cfg.Blobs,
		logger:     cfg.Logger,
		now:        cfg.Now,
		maxImages:  cfg.MaxImages,
		maxBytes:   cfg.MaxBytes,
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

// Mount registers GalleryService and the image routes on a mux. handle is
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
