package maps

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// UpdateMap's phases (MR-008, AIP-154, RN-10, MR-025): the request is checked
// before anything is read, the fog's image copy is made before the transaction,
// and the transaction locks the map, compares the revision, picks the image and
// writes. The checks run in this order: something to change, the revision, the
// name, the image's id; inside the transaction: the map, the revision, the image.

// mapUpdate is what an UpdateMap request asks for, checked. A nil field stays as
// it is.
type mapUpdate struct {
	name, imageID *string
	revision      int32
}

// checkMapUpdate refuses a request that changes nothing, carries no revision, or
// whose name or image's id cannot be a map's.
func checkMapUpdate(msg *mapsv1.UpdateMapRequest) (mapUpdate, error) {
	if msg.Name == nil && msg.ImageId == nil {
		return mapUpdate{}, connect.NewError(connect.CodeInvalidArgument, errors.New("set name, image_id or both"))
	}
	if msg.GetRevision() < 1 {
		return mapUpdate{}, connect.NewError(connect.CodeInvalidArgument, errors.New("revision must be at least 1"))
	}
	u := mapUpdate{revision: msg.GetRevision()}
	if msg.Name != nil {
		clean, err := cleanName("name", msg.GetName())
		if err != nil {
			return mapUpdate{}, err
		}
		u.name = &clean
	}
	if msg.ImageId != nil {
		id, ok := parseID(msg.GetImageId())
		if !ok {
			return mapUpdate{}, errNotAGalleryImage()
		}
		u.imageID = &id
	}
	return u, nil
}

// mapUpdateTx is one UpdateMap call as its transaction sees it.
type mapUpdateTx struct {
	mapUpdate
	campaignID, userID, mapID string
	imageCopy                 *fogCopy
	fogSeen                   bool // the fog the copy was prepared for (imageCopyFor)
}

// mapUpdateWrite is what the transaction did.
type mapUpdateWrite struct {
	updated mapsdb.Map
	cleared bool // the layers were cleared by a new image
	copied  bool // the fog's image copy was used
}

// writeMapUpdate is UpdateMap's transaction. Whether the layers are cleared is
// decided from the row locked here, so a combat that starts meanwhile is seen.
func (s *Service) writeMapUpdate(ctx context.Context, tx pgx.Tx, c mapUpdateTx) (mapUpdateWrite, error) {
	q := s.queries.WithTx(tx)
	row, err := lockMapToUpdate(ctx, q, c.campaignID, c.mapID)
	if err != nil {
		return mapUpdateWrite{}, err
	}
	if row.Revision != c.revision {
		return mapUpdateWrite{}, errStaleMap()
	}
	params := mapsdb.UpdateMapParams{CampaignID: c.campaignID, ID: c.mapID, Name: row.Name, ImageID: row.ImageID, Revision: row.Revision, Now: s.now()}
	if c.name != nil {
		params.Name = *c.name
	}
	var w mapUpdateWrite
	if c.imageID != nil {
		if w.copied, err = s.pickMapImage(ctx, q, c, row, &params); err != nil {
			return mapUpdateWrite{}, err
		}
	}
	// A new image clears the painted layers (D2), which a fight standing on
	// them cannot lose; the same image again clears nothing.
	imageChanged := params.ImageID != row.ImageID
	if imageChanged {
		if err := s.refuseWhileCombat(ctx, tx, c.campaignID, c.mapID); err != nil {
			return mapUpdateWrite{}, err
		}
	}
	if w.updated, err = q.UpdateMap(ctx, params); errors.Is(err, pgx.ErrNoRows) {
		return mapUpdateWrite{}, errStaleMap() // never, under the lock above: a second guard
	} else if err != nil {
		return mapUpdateWrite{}, fmt.Errorf("update map: %w", err)
	}
	if imageChanged {
		w.cleared = true
		return w, clearLayers(ctx, q, c.mapID)
	}
	return w, nil
}

// lockMapToUpdate locks the map's row.
func lockMapToUpdate(ctx context.Context, q *mapsdb.Queries, campaignID, mapID string) (mapsdb.Map, error) {
	row, err := q.GetMapForUpdate(ctx, mapsdb.GetMapForUpdateParams{CampaignID: campaignID, ID: mapID})
	if errors.Is(err, pgx.ErrNoRows) {
		return row, errMapNotFound()
	}
	if err != nil {
		return row, fmt.Errorf("find map: %w", err)
	}
	return row, nil
}

// pickMapImage checks that the new image is in the campaign's gallery and fits
// the map's grid, and puts it (or the copy made for it) into the update. It tells
// whether the copy was used.
func (s *Service) pickMapImage(ctx context.Context, q *mapsdb.Queries, c mapUpdateTx, row mapsdb.Map, params *mapsdb.UpdateMapParams) (bool, error) {
	img, err := q.GetGalleryImageInCampaign(ctx, mapsdb.GetGalleryImageInCampaignParams{CampaignID: c.campaignID, ID: *c.imageID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, errNotAGalleryImage()
	}
	if err != nil {
		return false, fmt.Errorf("find image: %w", err)
	}
	if err := checkImageFitsGrid(row, *c.imageID, img); err != nil {
		return false, err
	}
	if *c.imageID != row.ImageID && c.fogSeen != row.FogEnabled {
		return false, errStaleMap() // the fog changed after the copy was prepared, and the revision did not
	}
	params.ImageID = *c.imageID
	if c.imageCopy == nil || *c.imageID != c.imageCopy.source.ID {
		return false, nil
	}
	if err := c.imageCopy.insert(ctx, q, s, c.campaignID, c.userID); err != nil {
		return false, err
	}
	params.ImageID = c.imageCopy.id
	return true, nil
}

// checkImageFitsGrid refuses an image that would make a calibrated map's grid
// pass the limits. The rows follow the image's proportions: a taller image on a
// calibrated map could pass the rules' 400 rows (MR-025), which no layer, combat
// or CHECK accepts. The master picks another image or recalibrates first.
func checkImageFitsGrid(row mapsdb.Map, imageID string, img mapsdb.GalleryImage) error {
	if row.GridColumns == nil || imageID == row.ImageID {
		return nil
	}
	if _, err := grid.EngineGrid(int(*row.GridColumns/row.GridFactor), int(row.GridFactor), int(img.Width), int(img.Height)); err != nil {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(
			"this image would make the map's grid pass the limit of %d x %d squares: lower the calibration first", grid.MaxColumns, grid.MaxRows))
	}
	return nil
}

// announceMapUpdate tells the watchers of an updated map, and forgets what
// depended on the layers when a new image cleared them.
func (s *Service) announceMapUpdate(ctx context.Context, m authz.Membership, mapID, current string, w mapUpdateWrite) {
	s.publishMapChanged(m.CampaignID, mapID, playersSee(mapID, w.updated.RevealedAt, current))
	if w.cleared {
		s.tiles.forget(mapID)
		s.refreshVision(ctx, m.CampaignID, mapID) // the players' memory went with the layers
	}
}
