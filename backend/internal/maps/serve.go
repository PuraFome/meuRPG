package maps

import (
	"context"
	"errors"
	"net/http"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/blob"
	"github.com/PuraFome/meuRPG/backend/internal/platform/slowclient"
)

// playersSeeImage says whether the player sees the image now: it is the open
// session's shown image and they are in its audience, an image the master left
// with them (MR-028, RN-10), or the background of a map they see (revealed, or
// the session's current map), or the portrait of an NPC on the stage of the open
// scene when they are in the scene (MR-031). Reads, one per module: what the
// session shows and the stage come from play (LiveSession), the maps from this
// module's table.
//
// The background of a map with the fog of war on is never one of them: a player
// receives such a map only as the squares their character sees (MR-036), so the
// raw image is refused them whatever else shows it (RN-10). Turning the fog on
// copies an image that is also used another way (fogimage.go), so the fog map's
// image is its own.
func (s *Service) playersSeeImage(ctx context.Context, campaignID, userID, imageID string) (bool, error) {
	fog, err := s.queries.ImageIsOnAFogMap(ctx, mapsdb.ImageIsOnAFogMapParams{CampaignID: campaignID, ImageID: imageID})
	if err != nil || fog {
		return false, err
	}
	currentMap, _, err := s.live.OnScreen(ctx, campaignID)
	if err != nil {
		return false, err
	}
	shown, err := s.live.ImageShownTo(ctx, campaignID, userID, imageID)
	if err != nil || shown {
		return shown, err
	}
	left, err := s.queries.ImageIsLeftWith(ctx, mapsdb.ImageIsLeftWithParams{CampaignID: campaignID, ImageID: imageID, UserID: userID})
	if err != nil || left {
		return left, err
	}
	var current *string
	if currentMap != "" {
		current = &currentMap
	}
	onMap, err := s.queries.ImageIsOnAVisibleMap(ctx, mapsdb.ImageIsOnAVisibleMapParams{CampaignID: campaignID, ImageID: imageID, CurrentMapID: current})
	if err != nil || onMap {
		return onMap, err
	}
	// A portrait is fetchable only while its NPC is on the stage (RN-20): the
	// moment the master takes the NPC off, or closes the scene, it is 404.
	return s.live.ImageOnStage(ctx, campaignID, userID, imageID)
}

// playerSeesImage is playersSeeImage, or an image a line the player may read carries (RN-36).
func (s *Service) playerSeesImage(ctx context.Context, campaignID, userID, imageID string) (bool, error) {
	visible, err := s.playersSeeImage(ctx, campaignID, userID, imageID)
	if err != nil || visible {
		return visible, err
	}
	// The raw image of a fog map is refused whatever a line says (RN-10, MR-036).
	fog, err := s.queries.ImageIsOnAFogMap(ctx, mapsdb.ImageIsOnAFogMapParams{CampaignID: campaignID, ImageID: imageID})
	if err != nil || fog {
		return false, err
	}
	return s.live.ImageInConversation(ctx, campaignID, userID, imageID)
}

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

// serve sends an image, or its thumbnail, to a member of its campaign who
// may see it now (RN-10):
//   - the campaign's master, every image of the campaign;
//   - a player, only an image they see at this moment: the background of a
//     map they see (revealed, or the open session's current map), the
//     image the master shows in the open session when they are in its
//     audience, an image the master left with them (MR-028, RN-10), or the
//     portrait of an NPC that is on the stage of the open scene they are in
//     (MR-031).
//
// Knowing an ID is not enough for a player: they keep the IDs of maps that
// were hidden again and of images no longer shown, so the rule is checked
// on every request. Anyone else, and a player asking for an image they may
// not see now, gets 404, exactly as for an image that does not exist,
// never 403: the answer must not say whether an image exists.
//
// The order of the checks keeps that true. Without a session the answer
// is 401 before the image is even looked up. The membership and visibility
// checks come before the ETag's 304, so nobody learns that an image exists,
// or keeps an image that is no longer theirs to see, by sending
// If-None-Match.
//
// Caching follows the same rule. The master's copy may be kept for a year
// (an ID's bytes never change). A player's copy must be checked again on
// every use (no-cache): the browser asks with If-None-Match, gets a cheap
// 304 while the image is still visible, and 404 once it is not. Both carry
// Vary: Cookie, so a browser shared by the master and a player (one signs
// out, the other signs in) never answers one's request from the other's
// cached copy: a new session cookie is a new cache entry.
func (s *Service) serve(w http.ResponseWriter, r *http.Request, thumbnail bool) error {
	ctx := r.Context()
	if _, err := authz.RequireSignedIn(ctx); err != nil {
		return err
	}
	row, master, err := s.viewableImage(ctx, r.PathValue("id"))
	if err != nil {
		return err
	}
	key, etag := imageFileKey(row, thumbnail)
	obj, err := s.openImage(ctx, key)
	if err != nil {
		return err
	}
	defer func() { _ = obj.Close() }()

	setImageHeaders(w.Header(), obj.ContentType, etag, master)
	// ServeContent sets Content-Length, answers If-None-Match with 304
	// (from the ETag above), HEAD and range requests.
	defer slowclient.WriteBody(w, downloadWriteTimeout)()
	http.ServeContent(w, r, "", time.Time{}, obj.Content)
	return nil
}

// viewableImage finds the image and checks that the caller may see it now: a
// member of its campaign, and for a player an image the players see. It says
// whether the caller is the master. Anyone else gets the same 404 as for an
// image that does not exist.
func (s *Service) viewableImage(ctx context.Context, rawID string) (mapsdb.GalleryImage, bool, error) {
	id, err := uuid.Parse(rawID)
	if err != nil {
		return mapsdb.GalleryImage{}, false, errImageNotFound()
	}
	row, err := s.queries.GetGalleryImage(ctx, id.String())
	if errors.Is(err, pgx.ErrNoRows) {
		return mapsdb.GalleryImage{}, false, errImageNotFound()
	}
	if err != nil {
		return mapsdb.GalleryImage{}, false, s.dbError(ctx, "find an image", err)
	}
	m, err := authz.RequireCampaignMember(ctx, row.CampaignID)
	if err != nil {
		if connect.CodeOf(err) == connect.CodeUnavailable {
			return mapsdb.GalleryImage{}, false, err
		}
		return mapsdb.GalleryImage{}, false, errImageNotFound()
	}
	master := m.Role == authz.RoleMaster
	if !master {
		visible, err := s.playerSeesImage(ctx, row.CampaignID, m.UserID, row.ID)
		if err != nil {
			return mapsdb.GalleryImage{}, false, s.dbError(ctx, "check whether a player sees an image", err)
		}
		if !visible {
			return mapsdb.GalleryImage{}, false, errImageNotFound()
		}
	}
	return row, master, nil
}

// imageFileKey is the blob key and the ETag of the image's file, or of its
// thumbnail.
func imageFileKey(row mapsdb.GalleryImage, thumbnail bool) (key, etag string) {
	imageKey, thumbnailKey := blobKeys(row.CampaignID, row.ID)
	if thumbnail {
		return thumbnailKey, `"` + row.ID + `.thumb"`
	}
	return imageKey, `"` + row.ID + `"`
}

// openImage opens an image's file. The row is written after its files and
// deleted before them, so a missing file means the store lost it.
func (s *Service) openImage(ctx context.Context, key string) (*blob.Object, error) {
	obj, err := s.blobs.Open(ctx, key)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			s.logger.ErrorContext(ctx, "maps: an image's file is missing")
			return nil, errImageNotFound()
		}
		s.logger.ErrorContext(ctx, "maps: cannot read an image file", "error", err)
		return nil, errStorage()
	}
	return obj, nil
}

// setImageHeaders sets how an image is sent and cached. The master's copy may be
// kept for a year (the bytes behind an ID never change), a player's must be
// checked again on every use; the browser must take the file for what
// Content-Type says and run nothing in it.
func setImageHeaders(header http.Header, contentType, etag string, master bool) {
	header.Set("Content-Type", contentType)
	if master {
		// The bytes behind an ID never change: a new upload gets a new ID.
		// So the master's browser may keep the image for a year without
		// asking again; private, because it is only for this member.
		header.Set("Cache-Control", "private, max-age=31536000, immutable")
	} else {
		// The player may lose sight of it any time: ask every time (the 304
		// keeps that cheap).
		header.Set("Cache-Control", "private, no-cache")
	}
	// The master's and a player's copies never stand in for each other.
	header.Set("Vary", "Cookie")
	header.Set("ETag", etag)
	// The browser must take the file for what Content-Type says, show it
	// in the page, and run nothing in it even when opened on its own.
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Content-Disposition", "inline")
	header.Set("Content-Security-Policy", "default-src 'none'")
	// Only this site's pages may embed it.
	header.Set("Cross-Origin-Resource-Policy", "same-origin")
}
