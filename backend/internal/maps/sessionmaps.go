package maps

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
)

// SessionMaps is what package play needs from this one for what the
// session shows (it implements play.MapKeeper):
//   - when the master makes a map the session's current one
//     (PlayService.SetCurrentMap), the map must be the campaign's, and it is
//     revealed, since the players see the current map (RN-10);
//   - when the master shows a gallery image (PlayService.SetShownImage,
//     MR-028), the image must be the campaign's, and the session needs its
//     name and size.
//
// It is apart from Service because the two modules need each other: this
// package needs play (LiveSession), and play needs this. SessionMaps needs
// nothing but the database, so cmd/api builds it first and hands it to
// play, then builds Service with play as its LiveSession.
type SessionMaps struct {
	queries *mapsdb.Queries
}

// NewSessionMaps returns the SessionMaps for play.
func NewSessionMaps(pool *pgxpool.Pool) *SessionMaps {
	return &SessionMaps{queries: mapsdb.New(pool)}
}

// RevealMap reveals the campaign's map inside tx; a revealed map stays as
// it is. It returns a `not_found` Connect error when mapID is not a map of
// the campaign. The caller checked that the caller is the campaign's
// master and that mapID is a UUID.
func (sm *SessionMaps) RevealMap(ctx context.Context, tx pgx.Tx, campaignID, mapID string, at time.Time) error {
	_, err := sm.queries.WithTx(tx).SetMapRevealed(ctx, mapsdb.SetMapRevealedParams{
		CampaignID: campaignID, ID: mapID, Revealed: true, Now: at,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return errMapNotFound()
	}
	if err != nil {
		return fmt.Errorf("reveal the current map: %w", err)
	}
	return nil
}

// ShownImage returns the campaign's gallery image as the session shows it
// to the players, or a `not_found` Connect error when imageID is not an
// image of the campaign's gallery. Its name goes along as the caption: the
// players see it while the image is shown.
func (sm *SessionMaps) ShownImage(ctx context.Context, campaignID, imageID string) (*playv1.ShownImage, error) {
	img, err := sm.queries.GetGalleryImageInCampaign(ctx, mapsdb.GetGalleryImageInCampaignParams{CampaignID: campaignID, ID: imageID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errImageNotFound()
	}
	if err != nil {
		return nil, fmt.Errorf("find the shown image: %w", err)
	}
	return &playv1.ShownImage{
		Id:           img.ID,
		Name:         img.Name,
		Width:        img.Width,
		Height:       img.Height,
		Url:          imageURL(img.ID),
		ThumbnailUrl: thumbnailURL(img.ID),
	}, nil
}
