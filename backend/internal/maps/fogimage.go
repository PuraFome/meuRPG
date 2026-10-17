package maps

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
)

// The image of a map with the fog of war on (MR-036, RN-10, D6). A player never
// receives it raw: the image route refuses it (serve.go, ImageIsOnAFogMap) and
// the map does not carry its id. A fog map's image is its own, and every other
// use gets a copy of its own, a new gallery image with the same bytes:
//
//   - when the fog goes on, an image that is also used another way (the background
//     of another map, an image left with the players, the one shown in the session,
//     a portrait) is copied for the fog map, and the other uses keep the old one;
//   - when the master shows an image, sets it as an NPC's portrait, or makes a map
//     without fog with it, and it is a fog map's background, the use gets the copy
//     (ImageToShow, PortraitImage, CreateMap, UpdateMap).
//
// The copy takes a place in the campaign's gallery: a full gallery refuses with
// `resource_exhausted`.
//
// The files are copied before the transaction (the way an upload stores them
// before its row), and the row, the quota check and the map's new image are one
// transaction; the copy's files are deleted when it does not happen.

// fogCopy is a copy of an image made for a fog map, not yet in the gallery.
type fogCopy struct {
	id     string
	source mapsdb.GalleryImage
	// remember marks the row as the copy of source (copy_of_image_id), so the next
	// use of the same fog image finds it (prepareUseCopy).
	remember bool
}

// copySuffix is how the copy's name says what it is for.
const copySuffix = " (névoa)"

// prepareFogCopy copies the image's files when the image is also used another
// way than as the background of this map, and returns nil when it is not (or
// there is no blob store). The caller adds the gallery row inside its transaction
// (insert) or deletes the files (discard).
func (s *Service) prepareFogCopy(ctx context.Context, campaignID, mapID, imageID string) (*fogCopy, error) {
	if s.blobs == nil {
		return nil, nil
	}
	used, err := s.imageUsedElsewhere(ctx, campaignID, mapID, imageID)
	if err != nil {
		return nil, err
	}
	if !used {
		return nil, nil
	}
	return s.copyFiles(ctx, campaignID, imageID)
}

// prepareReuseCopy copies the image's files when it is the background of a map
// with the fog on and something else is about to use it; nil when it is not.
func (s *Service) prepareReuseCopy(ctx context.Context, campaignID, imageID string) (*fogCopy, error) {
	if s.blobs == nil {
		return nil, nil
	}
	fog, err := s.queries.ImageIsOnAFogMap(ctx, mapsdb.ImageIsOnAFogMapParams{CampaignID: campaignID, ImageID: imageID})
	if err != nil || !fog {
		return nil, err
	}
	return s.copyFiles(ctx, campaignID, imageID)
}

// mapBeforeTx reads a map outside the transaction, to know which copy of an image to
// prepare. A map that is not there is ok=false: the transaction refuses it. Any other
// error is the call's, so a failed read never goes on as "no copy needed".
func (s *Service) mapBeforeTx(ctx context.Context, campaignID, mapID string) (m mapsdb.Map, ok bool, err error) {
	m, err = s.queries.GetMap(ctx, mapsdb.GetMapParams{CampaignID: campaignID, ID: mapID})
	if errors.Is(err, pgx.ErrNoRows) {
		return m, false, nil
	}
	if err != nil {
		return m, false, fmt.Errorf("read the map before copying its image: %w", err)
	}
	return m, true, nil
}

// imageCopyFor prepares the copy that giving the map imageID needs: a fog map's image
// is its own, and a map without fog never takes a fog map's. It is nil when the map
// has the image already, nothing else uses it, or the map is not there. fogSeen is the
// fog the copy was prepared for: the transaction refuses the call when the fog changed
// meanwhile (the revision does not move with it), because the copy would be the wrong one.
func (s *Service) imageCopyFor(ctx context.Context, campaignID, mapID, imageID string) (c *fogCopy, fogSeen bool, err error) {
	before, ok, err := s.mapBeforeTx(ctx, campaignID, mapID)
	if err != nil || !ok || before.ImageID == imageID {
		return nil, before.FogEnabled, err
	}
	if before.FogEnabled {
		c, err = s.prepareFogCopy(ctx, campaignID, mapID, imageID)
	} else {
		c, err = s.prepareReuseCopy(ctx, campaignID, imageID)
	}
	return c, before.FogEnabled, err
}

// fogCopyOf prepares the copy of the map's own image that turning its fog on needs,
// when needed says that the map as it is now is about to get the fog.
func (s *Service) fogCopyOf(ctx context.Context, campaignID, mapID string, needed func(mapsdb.Map) bool) (*fogCopy, error) {
	before, ok, err := s.mapBeforeTx(ctx, campaignID, mapID)
	if err != nil || !ok || !needed(before) {
		return nil, err
	}
	return s.prepareFogCopy(ctx, campaignID, mapID, before.ImageID)
}

// discardFogCopy deletes the files of a copy made for nothing: the transaction failed,
// or the map did not take it.
func (s *Service) discardFogCopy(ctx context.Context, campaignID string, c *fogCopy, unused bool) {
	if c != nil && unused {
		s.deleteFiles(ctx, campaignID, c.id)
	}
}

// giveMapItsCopy adds the copy to the gallery and makes it the map's image, inside the
// transaction.
func (c *fogCopy) giveMapItsCopy(ctx context.Context, q *mapsdb.Queries, s *Service, campaignID, userID, mapID string) error {
	if err := c.insert(ctx, q, s, campaignID, userID); err != nil {
		return err
	}
	if _, err := q.SetMapImageOnly(ctx, mapsdb.SetMapImageOnlyParams{CampaignID: campaignID, ID: mapID, ImageID: c.id, Now: s.now()}); err != nil {
		return fmt.Errorf("give the map its copy of the image: %w", err)
	}
	return nil
}

// copyFiles copies the files of a gallery image to a new image ID.
func (s *Service) copyFiles(ctx context.Context, campaignID, imageID string) (*fogCopy, error) {
	src, err := s.queries.GetGalleryImageInCampaign(ctx, mapsdb.GetGalleryImageInCampaignParams{CampaignID: campaignID, ID: imageID})
	if err != nil {
		return nil, fmt.Errorf("find the image: %w", err)
	}
	c := &fogCopy{id: uuid.New().String(), source: src}
	from, fromThumb := blobKeys(campaignID, src.ID)
	to, toThumb := blobKeys(campaignID, c.id)
	for _, f := range [][2]string{{from, to}, {fromThumb, toThumb}} {
		if err := s.copyBlob(ctx, f[0], f[1]); err != nil {
			s.deleteFiles(ctx, campaignID, c.id)
			return nil, err
		}
	}
	return c, nil
}

// copyBlob copies one file of the blob store, streaming it: nothing of the
// image (at most 10 MiB) is held in memory.
func (s *Service) copyBlob(ctx context.Context, from, to string) error {
	obj, err := s.blobs.Open(ctx, from)
	if err != nil {
		return fmt.Errorf("open an image file to copy it: %w", err)
	}
	defer func() { _ = obj.Close() }()
	if err := s.blobs.Put(ctx, to, obj.ContentType, obj.Content); err != nil {
		return fmt.Errorf("store the copy of an image file: %w", err)
	}
	return nil
}

// insert adds the copy to the gallery inside the transaction, checking the quota
// as an upload does.
func (c *fogCopy) insert(ctx context.Context, q *mapsdb.Queries, s *Service, campaignID, userID string) error {
	_, err := c.insertRow(ctx, q, s, campaignID, &userID)
	return err
}

// insertRow is insert, with who made it (nil for a copy made on a player's behalf
// by the session) and the new gallery row returned.
func (c *fogCopy) insertRow(ctx context.Context, q *mapsdb.Queries, s *Service, campaignID string, userID *string) (mapsdb.GalleryImage, error) {
	usage, err := q.GetGalleryUsage(ctx, campaignID)
	if err != nil {
		return mapsdb.GalleryImage{}, fmt.Errorf("read the gallery usage: %w", err)
	}
	if usage.ImageCount >= s.maxImages || usage.ByteCount+int64(c.source.ByteSize) > int64(s.maxBytes) {
		return mapsdb.GalleryImage{}, connect.NewError(connect.CodeResourceExhausted, errors.New("the campaign's gallery is full: a fog map's image needs a copy of its own"))
	}
	row, err := q.InsertGalleryImage(ctx, mapsdb.InsertGalleryImageParams{
		ID: c.id, CampaignID: campaignID, UploadedBy: userID, Name: copyName(c.source.Name),
		ContentType: c.source.ContentType, Width: c.source.Width, Height: c.source.Height, ByteSize: c.source.ByteSize, CreatedAt: s.now(), CopyOfImageID: c.copyOf(),
	})
	if err != nil {
		return mapsdb.GalleryImage{}, fmt.Errorf("insert the image's copy: %w", err)
	}
	return row, nil
}

func (c *fogCopy) copyOf() *string {
	if !c.remember {
		return nil
	}
	return &c.source.ID
}

// prepareUseCopy is what a use of the image other than the fog map's own gets: the
// image itself (nil copy) when it is not a fog map's background; the copy already
// made of it, when there is one; else the files of a new copy, whose row the
// caller adds inside its transaction (insertRow), or whose files it deletes. The
// returned image is the one to use, with the ID and name the new copy will have.
func (s *Service) prepareUseCopy(ctx context.Context, campaignID string, img mapsdb.GalleryImage) (mapsdb.GalleryImage, *fogCopy, error) {
	if s.blobs == nil {
		return img, nil, nil
	}
	fog, err := s.queries.ImageIsOnAFogMap(ctx, mapsdb.ImageIsOnAFogMapParams{CampaignID: campaignID, ImageID: img.ID})
	if err != nil {
		return mapsdb.GalleryImage{}, nil, fmt.Errorf("check the image's map: %w", err)
	}
	if !fog {
		return img, nil, nil
	}
	existing, err := s.queries.FindImageCopy(ctx, mapsdb.FindImageCopyParams{CampaignID: campaignID, CopyOfImageID: &img.ID})
	if err == nil {
		return existing, nil, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return mapsdb.GalleryImage{}, nil, fmt.Errorf("find the image's copy: %w", err)
	}
	c, err := s.copyFiles(ctx, campaignID, img.ID)
	if err != nil {
		return mapsdb.GalleryImage{}, nil, err
	}
	c.remember = true
	img.ID, img.Name = c.id, copyName(img.Name)
	return img, c, nil
}

// ownImage returns the gallery image to use for something other than the fog
// map's own background: the image itself, or, when it is a fog map's background, a
// copy of it, the one made before or a new one made now (its own transaction and
// quota check). An NPC's portrait uses it.
func (s *Service) ownImage(ctx context.Context, campaignID string, img mapsdb.GalleryImage) (mapsdb.GalleryImage, error) {
	use, c, err := s.prepareUseCopy(ctx, campaignID, img)
	if err != nil || c == nil {
		return use, err
	}
	var row mapsdb.GalleryImage
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		row, err = c.insertRow(ctx, s.queries.WithTx(tx), s, campaignID, nil)
		return err
	})
	if err != nil {
		s.deleteFiles(ctx, campaignID, c.id)
		return mapsdb.GalleryImage{}, err
	}
	return row, nil
}

// copyName is the name of the copy: the image's, then the suffix, within 80
// characters.
func copyName(name string) string { return copyNameWith(name, copySuffix) }

// copyNameWith is name and then suffix, within 80 characters.
func copyNameWith(name, suffix string) string {
	room := maxNameLength - utf8.RuneCountInString(suffix)
	if utf8.RuneCountInString(name) > room {
		name = strings.TrimSpace(string([]rune(name)[:room]))
	}
	return name + suffix
}
