package characters

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
)

// The NPC's portrait (MR-031, D7, question 62): an image of the campaign's
// gallery, kept on the NPC's sheet as `portrait_image_id`. The sheet is JSON,
// so the field needs no migration.
//
//   - Only an NPC has one: a player's character with a portrait is an error.
//   - The image must be in the campaign's gallery, which package maps owns, so
//     the check goes through a small interface (Gallery), connected by
//     SetGallery. An image of another campaign, or one that does not exist,
//     is the same error: nobody learns that an image exists elsewhere.
//   - Deleting the gallery image clears the portrait (ClearPortraits, which
//     package maps calls inside the delete's transaction), where a map's
//     image refuses the delete: a portrait is a convenience that falls back to
//     the initials, a map without its image is not a map.
//   - A portrait set in the instant the image is deleted may outlive it: the
//     URL then answers 404, and the app draws the initials, as for any NPC with
//     none. The next save of the sheet is refused until the portrait is fixed.

// Gallery says which images are in a campaign's gallery. The maps module
// implements it (maps.SessionMaps), because the gallery is its own.
type Gallery interface {
	// ImageInCampaign reports whether imageID is an image of the campaign's
	// gallery.
	ImageInCampaign(ctx context.Context, campaignID, imageID string) (bool, error)
}

// SetGallery connects the gallery, which package maps owns (the same
// arrangement as SetLevelUps). Without it no portrait is accepted.
func (s *Service) SetGallery(g Gallery) { s.gallery = g }

// portraitOf is the portrait image ID on a sheet, "" for none.
func portraitOf(sheet *charactersv1.CharacterSheet) string {
	if sheet.GetFull() != nil {
		return sheet.GetFull().GetPortraitImageId()
	}
	return sheet.GetBasic().GetPortraitImageId()
}

// checkPortrait checks the portrait of a sheet being written to a character of
// kind in the campaign. It returns a fieldError, for the handler to turn into
// invalid_argument, or an error of the database.
func (s *Service) checkPortrait(ctx context.Context, campaignID, kind string, sheet *charactersv1.CharacterSheet) error {
	id := portraitOf(sheet)
	if id == "" {
		return nil
	}
	field := "sheet.basic.portrait_image_id"
	if sheet.GetFull() != nil {
		field = "sheet.full.portrait_image_id"
	}
	if kind == kindPlayer {
		return fieldErr(field, "must be empty for a player's character")
	}
	id, ok := parseUUID(id)
	if !ok {
		return fieldErr(field, "must be an image of the campaign's gallery")
	}
	if s.gallery == nil {
		return fieldErr(field, "cannot be set: the gallery is off")
	}
	found, err := s.gallery.ImageInCampaign(ctx, campaignID, id)
	if err != nil {
		return s.dbError(ctx, "check a portrait", err)
	}
	if !found {
		return fieldErr(field, "must be an image of the campaign's gallery")
	}
	return nil
}

// ClearPortraits takes the image off the portrait of every NPC of the campaign
// that has it, inside tx, and returns how many it cleared. Package maps calls it
// when the master deletes the image from the gallery. Each NPC's revision goes
// up, so an editor still holding the old sheet is told it is stale instead of
// saving the dead portrait again.
func (s *Service) ClearPortraits(ctx context.Context, tx pgx.Tx, campaignID, imageID string) (int64, error) {
	n, err := s.queries.WithTx(tx).ClearPortraits(ctx, charactersdb.ClearPortraitsParams{CampaignID: campaignID, ImageID: imageID, Now: s.now()})
	if err != nil {
		return 0, fmt.Errorf("clear the portraits of an image: %w", err)
	}
	return n, nil
}
