package maps

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
)

// SessionMaps is what package play needs from this one for what the
// session shows (it implements play.MapKeeper):
//   - when the master makes a map the session's current one
//     (PlayService.SetCurrentMap), the map must be the campaign's, and it is
//     revealed, since the players see the current map (RN-10);
//   - when the master shows a gallery image (PlayService.SetShownImage,
//     MR-028), the image must be the campaign's, and the session needs its
//     name and size;
//   - when a combat starts (CombatService.StartEncounter, MR-013), the
//     session needs the map's grid, the battle point's map and where the
//     tokens stand, and when it ends it moves the player characters'
//     tokens to where they ended.
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

// gridRows is how many rows of squares a grid of columns across has on an
// image of this size (MR-013): the squares are square, so the rows follow
// the image's proportions, rounded, and kept between 1 and 400 (the
// encounters_grid_valid CHECK) for an extremely long image.
func gridRows(columns, width, height int32) int32 {
	if columns <= 0 || width <= 0 {
		return 0
	}
	rows := (int64(columns)*int64(height) + int64(width)/2) / int64(width)
	return int32(min(max(rows, 1), 400))
}

// MapGrid returns the battle grid of the campaign's map (MR-013), or the
// zero Grid when it has none. It returns a `not_found` Connect error when
// mapID is not a map of the campaign.
func (sm *SessionMaps) MapGrid(ctx context.Context, campaignID, mapID string) (link.Grid, error) {
	row, err := sm.queries.GetMapGrid(ctx, mapsdb.GetMapGridParams{CampaignID: campaignID, ID: mapID})
	if errors.Is(err, pgx.ErrNoRows) {
		return link.Grid{}, errMapNotFound()
	}
	if err != nil {
		return link.Grid{}, fmt.Errorf("read the map's grid: %w", err)
	}
	if row.GridColumns == nil {
		return link.Grid{}, nil
	}
	return link.Grid{Columns: *row.GridColumns, Rows: gridRows(*row.GridColumns, row.ImageWidth, row.ImageHeight)}, nil
}

// BattlePoint returns a battle point of the campaign: its map and the map of
// its fight, if the master chose one. It returns a `not_found` Connect error
// when pointID is not a battle point of the campaign.
func (sm *SessionMaps) BattlePoint(ctx context.Context, campaignID, pointID string) (link.BattlePoint, error) {
	p, err := sm.queries.GetMapPointInCampaign(ctx, mapsdb.GetMapPointInCampaignParams{CampaignID: campaignID, ID: pointID})
	if err == nil && p.Kind != kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE] {
		err = pgx.ErrNoRows // a point of another kind starts no combat
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return link.BattlePoint{}, errPointNotFound()
	}
	if err != nil {
		return link.BattlePoint{}, fmt.Errorf("find the battle point: %w", err)
	}
	out := link.BattlePoint{MapID: p.MapID}
	if p.TargetMapID != nil {
		out.TargetMapID = *p.TargetMapID
	}
	return out, nil
}

// MapTokens returns where every token of the map stands, hidden ones
// included: the combat places the combatants on the squares where their
// characters' tokens are. The caller is the play module, after its own
// authorization check, and sends nothing of it to a player.
func (sm *SessionMaps) MapTokens(ctx context.Context, mapID string) ([]link.TokenPosition, error) {
	rows, err := sm.queries.ListMapTokens(ctx, mapID)
	if err != nil {
		return nil, fmt.Errorf("list the map's tokens: %w", err)
	}
	out := make([]link.TokenPosition, 0, len(rows))
	for _, t := range rows {
		out = append(out, link.TokenPosition{CharacterID: t.CharacterID, XBP: t.XBp, YBP: t.YBp})
	}
	return out, nil
}

// SetTokenPositions moves the characters' tokens on the map inside tx, or
// puts them there when they have none: where a combat left its player
// characters. A token that exists keeps its hidden flag; a new one starts
// visible. The map must exist: its foreign key says so.
func (sm *SessionMaps) SetTokenPositions(ctx context.Context, tx pgx.Tx, mapID string, positions []link.TokenPosition, at time.Time) error {
	q := sm.queries.WithTx(tx)
	for _, p := range positions {
		if err := q.UpsertMapTokenPosition(ctx, mapsdb.UpsertMapTokenPositionParams{
			MapID: mapID, CharacterID: p.CharacterID, XBp: p.XBP, YBp: p.YBP, UpdatedAt: at,
		}); err != nil {
			return fmt.Errorf("move a token: %w", err)
		}
	}
	return nil
}
