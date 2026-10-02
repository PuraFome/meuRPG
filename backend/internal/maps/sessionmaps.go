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
//     name and size;
//   - when the master leaves the image with the players ("Deixar com os
//     jogadores", MR-028), the image goes to the campaign's left list, in
//     the session's transaction, and the master takes it back later. The
//     list is a table of this module (campaign_left_images) because the
//     image route reads it for the players (RN-10), next to the maps.
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
	return shownImage(img), nil
}

func shownImage(img mapsdb.GalleryImage) *playv1.ShownImage {
	return &playv1.ShownImage{
		Id:           img.ID,
		Name:         img.Name,
		Width:        img.Width,
		Height:       img.Height,
		Url:          imageURL(img.ID),
		ThumbnailUrl: thumbnailURL(img.ID),
	}
}

// LeaveImage leaves the campaign's gallery image with the players inside
// tx; an image already left stays as it is, and one that is not the
// campaign's anymore (deleted meanwhile) is skipped, since a left image
// that is gone has nothing to leave. The caller checked that the caller is
// the campaign's master.
func (sm *SessionMaps) LeaveImage(ctx context.Context, tx pgx.Tx, campaignID, imageID string, at time.Time) error {
	_, err := sm.queries.WithTx(tx).LeaveImage(ctx, mapsdb.LeaveImageParams{CampaignID: campaignID, ImageID: imageID, Now: at})
	if err != nil {
		return fmt.Errorf("leave the image with the players: %w", err)
	}
	return nil
}

// ListLeftImages returns the images left with the players, in the order
// they were left, with their names as captions.
func (sm *SessionMaps) ListLeftImages(ctx context.Context, campaignID string) ([]*playv1.ShownImage, error) {
	rows, err := sm.queries.ListLeftImages(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("list the left images: %w", err)
	}
	out := make([]*playv1.ShownImage, 0, len(rows))
	for _, r := range rows {
		out = append(out, shownImage(r))
	}
	return out, nil
}

// TakeBackImage takes the image off the left list, or returns a
// `not_found` Connect error when it is not on it. The caller checked that
// the caller is the campaign's master.
func (sm *SessionMaps) TakeBackImage(ctx context.Context, campaignID, imageID string) error {
	n, err := sm.queries.TakeBackLeftImage(ctx, mapsdb.TakeBackLeftImageParams{CampaignID: campaignID, ImageID: imageID})
	if err != nil {
		return fmt.Errorf("take the left image back: %w", err)
	}
	if n == 0 {
		return errImageNotFound()
	}
	return nil
}
