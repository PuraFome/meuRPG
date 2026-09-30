package maps

import (
	"errors"
	"net/http"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/blob"
)

// handleImage serves GET /images/{id}, the image.
func (s *Service) handleImage(w http.ResponseWriter, r *http.Request) {
	if err := s.serve(w, r, false); err != nil {
		s.writeError(w, r, err)
	}
}

// handleThumbnail serves GET /images/{id}/thumb, its thumbnail.
func (s *Service) handleThumbnail(w http.ResponseWriter, r *http.Request) {
	if err := s.serve(w, r, true); err != nil {
		s.writeError(w, r, err)
	}
}

// serve sends an image, or its thumbnail, to an active member of its
// campaign.
//
// Players may fetch images too, because the maps they see are images. That
// does not reveal a hidden map (RN-10): a player only learns an image's ID
// from a response they may see, and no such response carries the ID of a
// hidden map's image or the gallery's list. Anyone else gets 404, exactly
// as for an image that does not exist, never 403: the answer must not say
// whether an image exists.
//
// The order of the checks keeps that true. Without a session the answer
// is 401 before the image is even looked up. The membership check comes
// before the ETag's 304, so a stranger cannot learn that an image exists by
// sending If-None-Match either.
func (s *Service) serve(w http.ResponseWriter, r *http.Request, thumbnail bool) error {
	ctx := r.Context()
	if _, err := authz.RequireSignedIn(ctx); err != nil {
		return err
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return errImageNotFound()
	}
	row, err := s.queries.GetGalleryImage(ctx, id.String())
	if errors.Is(err, pgx.ErrNoRows) {
		return errImageNotFound()
	}
	if err != nil {
		return s.dbError(ctx, "find an image", err)
	}
	if _, err := authz.RequireCampaignMember(ctx, row.CampaignID); err != nil {
		if connect.CodeOf(err) == connect.CodeUnavailable {
			return err
		}
		return errImageNotFound()
	}

	imageKey, thumbnailKey := blobKeys(row.CampaignID, row.ID)
	key, etag := imageKey, `"`+row.ID+`"`
	if thumbnail {
		key, etag = thumbnailKey, `"`+row.ID+`.thumb"`
	}
	obj, err := s.blobs.Open(ctx, key)
	if err != nil {
		// The row is written after its files and deleted before them, so a
		// missing file means the store lost it.
		if errors.Is(err, blob.ErrNotFound) {
			s.logger.ErrorContext(ctx, "maps: an image's file is missing")
			return errImageNotFound()
		}
		s.logger.ErrorContext(ctx, "maps: cannot read an image file", "error", err)
		return errStorage()
	}
	defer func() { _ = obj.Close() }()

	header := w.Header()
	header.Set("Content-Type", obj.ContentType)
	// The bytes behind an ID never change: a new upload gets a new ID. So
	// the browser may keep the image for a year without asking again;
	// private, because it is only for this member.
	header.Set("Cache-Control", "private, max-age=31536000, immutable")
	header.Set("ETag", etag)
	// The browser must take the file for what Content-Type says, show it
	// in the page, and run nothing in it even when opened on its own.
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Content-Disposition", "inline")
	header.Set("Content-Security-Policy", "default-src 'none'")
	// Only this site's pages may embed it.
	header.Set("Cross-Origin-Resource-Policy", "same-origin")
	// ServeContent sets Content-Length, answers If-None-Match with 304
	// (from the ETag above), HEAD and range requests.
	http.ServeContent(w, r, "", time.Time{}, obj.Content)
	return nil
}
