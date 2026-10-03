package maps

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
)

// MapService (MR-008, MR-009, MR-012). Every handler starts with one
// explicit check: authz.RequireCampaignMember for the reads, which any
// member may call and which filter what a player sees (visibility.go), and
// authz.RequireCampaignRole(master) for everything that changes a map. The
// check's error is already the right Connect error.
//
// A write reads the session's current map first (it decides whether the
// players see the map), changes the rows in one transaction, and only after
// the commit tells the watching members (visibility.go, "The live events").

// Limits that keep the unpaginated lists small (a proposal, like the
// gallery's quota). Tests set smaller ones through Config.
const (
	// DefaultMaxMaps is how many maps a campaign may have.
	DefaultMaxMaps = 200
	// DefaultMaxPointsPerMap is how many points of interest a map may have.
	DefaultMaxPointsPerMap = 200
)

// maxDescriptionLength is the longest point description, in characters
// (map_points_description_length).
const maxDescriptionLength = 2000

// maxHooksLength is the longest "Ganchos e anotações" of a scene point, in
// characters (map_points_hooks_length).
const maxHooksLength = 4000

// maxPosition is the largest position, in basis points of the image's width
// or height (map_points_position_valid, map_tokens_position_valid).
const maxPosition = 10000

// The database's point kinds (map_points_kind_valid) and the API's.
var (
	kindToDB = map[mapsv1.MapPointKind]string{
		mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE: "battle",
		mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP: "submap",
		mapsv1.MapPointKind_MAP_POINT_KIND_SCENE:  "scene",
	}
	kindFromDB = map[string]mapsv1.MapPointKind{
		"battle": mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE,
		"submap": mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP,
		"scene":  mapsv1.MapPointKind_MAP_POINT_KIND_SCENE,
	}
)

// ListMaps implements mapsv1connect.MapServiceHandler.
func (s *Service) ListMaps(
	ctx context.Context,
	req *connect.Request[mapsv1.ListMapsRequest],
) (*connect.Response[mapsv1.ListMapsResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	v, err := s.viewerOf(ctx, m)
	if err != nil {
		return nil, err
	}
	cm, err := s.loadMaps(ctx, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list maps", err)
	}
	res := &mapsv1.ListMapsResponse{}
	for _, r := range cm.rows {
		if v.seesMap(r.ID, r.RevealedAt) {
			res.Maps = append(res.Maps, cm.mapToProto(r, v))
		}
	}
	return connect.NewResponse(res), nil
}

// GetMap implements mapsv1connect.MapServiceHandler.
func (s *Service) GetMap(
	ctx context.Context,
	req *connect.Request[mapsv1.GetMapRequest],
) (*connect.Response[mapsv1.GetMapResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	mapID, ok := parseID(req.Msg.GetMapId())
	if !ok {
		return nil, errMapNotFound()
	}
	v, err := s.viewerOf(ctx, m)
	if err != nil {
		return nil, err
	}
	cm, err := s.loadMaps(ctx, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "get a map", err)
	}
	row, ok := cm.byID[mapID]
	if !ok || !v.seesMap(row.ID, row.RevealedAt) {
		// A hidden map is "not found" to a player, exactly as a map that
		// does not exist (RN-10).
		return nil, errMapNotFound()
	}
	res := &mapsv1.GetMapResponse{Map: cm.mapToProto(row, v)}

	points, err := s.queries.ListMapPoints(ctx, mapID)
	if err != nil {
		return nil, s.dbError(ctx, "list a map's points", err)
	}
	for _, p := range points {
		if v.seesPoint(p) {
			res.Points = append(res.Points, cm.pointToProto(p, v))
		}
	}
	if err := s.attachActions(ctx, mapID, res.Points, v.master); err != nil {
		return nil, s.dbError(ctx, "list a map's scene actions", err)
	}
	if err := s.attachClues(ctx, m.CampaignID, mapID, res.Points, v.master); err != nil {
		return nil, s.dbError(ctx, "list a map's scene clues", err)
	}

	tokens, err := s.queries.ListMapTokens(ctx, mapID)
	if err != nil {
		return nil, s.dbError(ctx, "list a map's tokens", err)
	}
	byCharacter := make(map[string]mapsdb.MapToken, len(tokens))
	ids := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if v.seesToken(t) {
			byCharacter[t.CharacterID] = t
			ids = append(ids, t.CharacterID)
		}
	}
	// The characters come back in their own order (players' first), and
	// only the living ones: a dead character's token stays on the map, but
	// is not listed.
	characters, err := s.characters.MapCharacters(ctx, m.CampaignID, ids)
	if err != nil {
		return nil, s.dbError(ctx, "read the characters on a map", err)
	}
	for _, c := range characters {
		if t, ok := byCharacter[c.GetId()]; ok {
			res.Tokens = append(res.Tokens, tokenToProto(t, c, v))
		}
	}
	return connect.NewResponse(res), nil
}

// CreateMap implements mapsv1connect.MapServiceHandler.
func (s *Service) CreateMap(
	ctx context.Context,
	req *connect.Request[mapsv1.CreateMapRequest],
) (*connect.Response[mapsv1.CreateMapResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	name, err := cleanName("name", req.Msg.GetName())
	if err != nil {
		return nil, err
	}
	imageID, ok := parseID(req.Msg.GetImageId())
	if !ok {
		return nil, errNotAGalleryImage()
	}

	var created mapsdb.Map
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		count, err := q.CountMaps(ctx, m.CampaignID)
		if err != nil {
			return fmt.Errorf("count maps: %w", err)
		}
		if count >= s.maxMaps {
			return connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("the campaign already has %d maps", s.maxMaps))
		}
		if err := checkImage(ctx, q, m.CampaignID, imageID); err != nil {
			return err
		}
		created, err = q.InsertMap(ctx, mapsdb.InsertMapParams{CampaignID: m.CampaignID, Name: name, ImageID: imageID, Now: s.now()})
		if err != nil {
			return fmt.Errorf("insert map: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "create a map", err)
	}
	// A new map is hidden: only the master hears about it.
	s.publishMapChanged(m.CampaignID, created.ID, false)
	out, err := s.masterMap(ctx, m, created.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&mapsv1.CreateMapResponse{Map: out}), nil
}

// UpdateMap implements mapsv1connect.MapServiceHandler.
func (s *Service) UpdateMap(
	ctx context.Context,
	req *connect.Request[mapsv1.UpdateMapRequest],
) (*connect.Response[mapsv1.UpdateMapResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	mapID, ok := parseID(req.Msg.GetMapId())
	if !ok {
		return nil, errMapNotFound()
	}
	if req.Msg.Name == nil && req.Msg.ImageId == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("set name, image_id or both"))
	}
	if req.Msg.GetRevision() < 1 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("revision must be at least 1"))
	}
	var name, imageID *string
	if req.Msg.Name != nil {
		clean, err := cleanName("name", req.Msg.GetName())
		if err != nil {
			return nil, err
		}
		name = &clean
	}
	if req.Msg.ImageId != nil {
		id, ok := parseID(req.Msg.GetImageId())
		if !ok {
			return nil, errNotAGalleryImage()
		}
		imageID = &id
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}

	var updated mapsdb.Map
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		row, err := q.GetMapForUpdate(ctx, mapsdb.GetMapForUpdateParams{CampaignID: m.CampaignID, ID: mapID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errMapNotFound()
		}
		if err != nil {
			return fmt.Errorf("find map: %w", err)
		}
		if row.Revision != req.Msg.GetRevision() {
			return errStaleMap()
		}
		params := mapsdb.UpdateMapParams{CampaignID: m.CampaignID, ID: mapID, Name: row.Name, ImageID: row.ImageID, Revision: row.Revision, Now: s.now()}
		if name != nil {
			params.Name = *name
		}
		if imageID != nil {
			if err := checkImage(ctx, q, m.CampaignID, *imageID); err != nil {
				return err
			}
			params.ImageID = *imageID
		}
		updated, err = q.UpdateMap(ctx, params)
		if errors.Is(err, pgx.ErrNoRows) {
			return errStaleMap() // never, under the lock above: a second guard
		}
		if err != nil {
			return fmt.Errorf("update map: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "update a map", err)
	}
	s.publishMapChanged(m.CampaignID, mapID, playersSee(mapID, updated.RevealedAt, current))
	out, err := s.masterMap(ctx, m, mapID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&mapsv1.UpdateMapResponse{Map: out}), nil
}

// DeleteMap implements mapsv1connect.MapServiceHandler.
func (s *Service) DeleteMap(
	ctx context.Context,
	req *connect.Request[mapsv1.DeleteMapRequest],
) (*connect.Response[mapsv1.DeleteMapResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	mapID, ok := parseID(req.Msg.GetMapId())
	if !ok {
		return nil, errMapNotFound()
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	// The links to this map, before they go, to tell the maps they leave.
	before, err := s.loadMaps(ctx, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "delete a map", err)
	}
	openScene, err := s.live.OpenScenePoint(ctx, m.CampaignID) // a point of this map may be the open scene
	if err != nil {
		return nil, s.dbError(ctx, "read the open scene", err)
	}

	var deleted mapsdb.Map
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		deleted, err = s.queries.WithTx(tx).DeleteMap(ctx, mapsdb.DeleteMapParams{CampaignID: m.CampaignID, ID: mapID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errMapNotFound()
		}
		if err != nil {
			return fmt.Errorf("delete map: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "delete a map", err)
	}
	seen := playersSee(mapID, deleted.RevealedAt, current)
	s.publishMapChanged(m.CampaignID, mapID, seen)
	s.publishParentsChanged(m.CampaignID, mapID, before, current, seen, false)
	if mapID == current {
		// The foreign key unset the session's current map.
		s.publishCurrentMapCleared(m.CampaignID)
	}
	if openScene != "" {
		// The map's points went with it: if the scene was one of them, the
		// foreign key closed it.
		if still, err := s.live.OpenScenePoint(ctx, m.CampaignID); err == nil && still == "" {
			s.publishSceneChanged(m.CampaignID)
		}
	}
	return connect.NewResponse(&mapsv1.DeleteMapResponse{}), nil
}

// SetMapRevealed implements mapsv1connect.MapServiceHandler.
func (s *Service) SetMapRevealed(
	ctx context.Context,
	req *connect.Request[mapsv1.SetMapRevealedRequest],
) (*connect.Response[mapsv1.SetMapRevealedResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	mapID, ok := parseID(req.Msg.GetMapId())
	if !ok {
		return nil, errMapNotFound()
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}

	var before, after mapsdb.Map
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		before, err = q.GetMapForUpdate(ctx, mapsdb.GetMapForUpdateParams{CampaignID: m.CampaignID, ID: mapID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errMapNotFound()
		}
		if err != nil {
			return fmt.Errorf("find map: %w", err)
		}
		after, err = q.SetMapRevealed(ctx, mapsdb.SetMapRevealedParams{CampaignID: m.CampaignID, ID: mapID, Revealed: req.Msg.GetRevealed(), Now: s.now()})
		if err != nil {
			return fmt.Errorf("set map revealed: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "reveal or hide a map", err)
	}
	seenBefore, seenAfter := playersSee(mapID, before.RevealedAt, current), playersSee(mapID, after.RevealedAt, current)
	s.publishMapChanged(m.CampaignID, mapID, seenBefore || seenAfter)
	if cm, err := s.loadMaps(ctx, m.CampaignID); err == nil {
		s.publishParentsChanged(m.CampaignID, mapID, cm, current, seenBefore, seenAfter)
	} else {
		// The change is made; only a hint to other maps is lost, and the
		// app reads the maps again after any reconnection.
		s.logger.ErrorContext(ctx, "maps: cannot tell the parent maps about a revealed map", "error", err)
	}
	out, err := s.masterMap(ctx, m, mapID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&mapsv1.SetMapRevealedResponse{Map: out}), nil
}

// The battle grid's limits (MR-013): how many squares of 1.5 m fit across
// the image's width. The maps_grid_columns_valid CHECK says the same.
const (
	minGridColumns = 4
	maxGridColumns = 200
)

// SetMapGrid implements mapsv1connect.MapServiceHandler.
func (s *Service) SetMapGrid(
	ctx context.Context,
	req *connect.Request[mapsv1.SetMapGridRequest],
) (*connect.Response[mapsv1.SetMapGridResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	mapID, ok := parseID(req.Msg.GetMapId())
	if !ok {
		return nil, errMapNotFound()
	}
	columns := req.Msg.GetColumns()
	if columns != 0 && (columns < minGridColumns || columns > maxGridColumns) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("columns must be 0 (no grid) or %d to %d", minGridColumns, maxGridColumns))
	}
	var gridColumns *int32 // NULL clears the grid
	if columns != 0 {
		gridColumns = &columns
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}

	var updated mapsdb.Map
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		updated, err = s.queries.WithTx(tx).SetMapGrid(ctx, mapsdb.SetMapGridParams{
			CampaignID: m.CampaignID, ID: mapID, GridColumns: gridColumns, Now: s.now(),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return errMapNotFound()
		}
		if err != nil {
			return fmt.Errorf("set map grid: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "set a map's grid", err)
	}
	s.publishMapChanged(m.CampaignID, mapID, playersSee(mapID, updated.RevealedAt, current))
	out, err := s.masterMap(ctx, m, mapID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&mapsv1.SetMapGridResponse{Map: out}), nil
}

// pointChange is what the master asks to change in a point. Nil fields
// stay as they are.
type pointChange struct {
	kind        *string
	name        *string
	description *string
	hooks       *string // a SCENE point's private text; "" clears it
	x, y        *int32
	target      *string // "" removes the target
	revealed    *bool
}

// CreateMapPoint implements mapsv1connect.MapServiceHandler.
func (s *Service) CreateMapPoint(
	ctx context.Context,
	req *connect.Request[mapsv1.CreateMapPointRequest],
) (*connect.Response[mapsv1.CreateMapPointResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	mapID, ok := parseID(req.Msg.GetMapId())
	if !ok {
		return nil, errMapNotFound()
	}
	kind, ok := kindToDB[req.Msg.GetKind()]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("kind is required"))
	}
	name, err := cleanName("name", req.Msg.GetName())
	if err != nil {
		return nil, err
	}
	description, err := cleanDescription(req.Msg.GetDescription())
	if err != nil {
		return nil, err
	}
	hooks, err := cleanHooks(req.Msg.GetHooks())
	if err != nil {
		return nil, err
	}
	if hooks != "" && kind != kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_SCENE] {
		return nil, errOnlyScenesHaveHooks()
	}
	if err := checkPosition(req.Msg.GetXBp(), req.Msg.GetYBp()); err != nil {
		return nil, err
	}
	target, err := parseTarget(req.Msg.GetTargetMapId(), kind, mapID)
	if err != nil {
		return nil, err
	}

	var created mapsdb.MapPoint
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		if _, err := s.campaignMap(ctx, q, m.CampaignID, mapID); err != nil {
			return err
		}
		count, err := q.CountMapPoints(ctx, mapID)
		if err != nil {
			return fmt.Errorf("count points: %w", err)
		}
		if count >= s.maxPoints {
			return connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("the map already has %d points", s.maxPoints))
		}
		if err := checkTarget(ctx, q, m.CampaignID, target); err != nil {
			return err
		}
		created, err = q.InsertMapPoint(ctx, mapsdb.InsertMapPointParams{
			MapID: mapID, Kind: kind, Name: name, Description: description, Hooks: hooks,
			XBp: req.Msg.GetXBp(), YBp: req.Msg.GetYBp(), TargetMapID: target, Now: s.now(),
		})
		if err != nil {
			return fmt.Errorf("insert point: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "create a point", err)
	}
	// A new point is hidden: only the master hears about it (MR-009).
	s.publishMapChanged(m.CampaignID, mapID, false)
	out, err := s.masterPoint(ctx, m, created)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&mapsv1.CreateMapPointResponse{Point: out}), nil
}

// UpdateMapPoint implements mapsv1connect.MapServiceHandler.
func (s *Service) UpdateMapPoint(
	ctx context.Context,
	req *connect.Request[mapsv1.UpdateMapPointRequest],
) (*connect.Response[mapsv1.UpdateMapPointResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	mapID, ok := parseID(req.Msg.GetMapId())
	if !ok {
		return nil, errMapNotFound()
	}
	pointID, ok := parseID(req.Msg.GetPointId())
	if !ok {
		return nil, errPointNotFound()
	}
	msg := req.Msg
	if msg.Kind == nil && msg.Name == nil && msg.Description == nil && msg.Hooks == nil && msg.XBp == nil && msg.YBp == nil && msg.TargetMapId == nil && msg.Revealed == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("nothing to change"))
	}
	change := pointChange{x: msg.XBp, y: msg.YBp, target: msg.TargetMapId, revealed: msg.Revealed}
	if msg.Kind != nil {
		kind, ok := kindToDB[msg.GetKind()]
		if !ok {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("kind must not be unspecified"))
		}
		change.kind = &kind
	}
	if msg.Name != nil {
		name, err := cleanName("name", msg.GetName())
		if err != nil {
			return nil, err
		}
		change.name = &name
	}
	if msg.Description != nil {
		description, err := cleanDescription(msg.GetDescription())
		if err != nil {
			return nil, err
		}
		change.description = &description
	}
	if msg.Hooks != nil {
		hooks, err := cleanHooks(msg.GetHooks())
		if err != nil {
			return nil, err
		}
		change.hooks = &hooks
	}
	if err := checkPosition(msg.GetXBp(), msg.GetYBp()); err != nil {
		return nil, err // unset positions read as 0, which is valid
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}

	var mapRow mapsdb.Map
	var before, after mapsdb.MapPoint
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		if mapRow, err = s.campaignMap(ctx, q, m.CampaignID, mapID); err != nil {
			return err
		}
		before, err = q.GetMapPointForUpdate(ctx, mapsdb.GetMapPointForUpdateParams{MapID: mapID, ID: pointID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errPointNotFound()
		}
		if err != nil {
			return fmt.Errorf("find point: %w", err)
		}
		after, err = s.applyPointChange(ctx, q, m.CampaignID, before, change)
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "update a point", err)
	}
	s.publishMapChanged(m.CampaignID, mapID,
		playersSee(mapID, mapRow.RevealedAt, current) && (before.RevealedAt != nil || after.RevealedAt != nil))
	// The open scene shows the point's name and description, and stops being
	// one when the point changes kind.
	s.publishSceneChangedIf(ctx, m.CampaignID, pointID)
	out, err := s.masterPoint(ctx, m, after)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&mapsv1.UpdateMapPointResponse{Point: out}), nil
}

// applyPointChange checks a change against the point as it is, inside the
// transaction, and saves it.
func (s *Service) applyPointChange(ctx context.Context, q *mapsdb.Queries, campaignID string, p mapsdb.MapPoint, c pointChange) (mapsdb.MapPoint, error) {
	params := mapsdb.UpdateMapPointParams{
		MapID: p.MapID, ID: p.ID, Kind: p.Kind, Name: p.Name, Description: p.Description,
		Hooks: p.Hooks, XBp: p.XBp, YBp: p.YBp, TargetMapID: p.TargetMapID, RevealedAt: p.RevealedAt, Now: s.now(),
	}
	if c.kind != nil {
		params.Kind = *c.kind
	}
	if c.name != nil {
		params.Name = *c.name
	}
	if c.description != nil {
		params.Description = *c.description
	}
	if c.hooks != nil {
		params.Hooks = *c.hooks
	}
	if c.x != nil {
		params.XBp = *c.x
	}
	if c.y != nil {
		params.YBp = *c.y
	}
	if c.target != nil {
		target, err := parseTarget(*c.target, params.Kind, p.MapID)
		if err != nil {
			return mapsdb.MapPoint{}, err
		}
		params.TargetMapID = target
	}
	if !leads(params.Kind) {
		params.TargetMapID = nil // only a submap or a battle leads somewhere
	}
	if err := checkTarget(ctx, q, campaignID, params.TargetMapID); err != nil {
		return mapsdb.MapPoint{}, err
	}
	if c.revealed != nil {
		params.RevealedAt = revealedAt(p.RevealedAt, *c.revealed, params.Now)
	}
	scene := kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_SCENE]
	if params.Kind != scene && c.hooks != nil && *c.hooks != "" {
		return mapsdb.MapPoint{}, errOnlyScenesHaveHooks()
	}
	if p.Kind == scene && params.Kind != p.Kind {
		// Only a scene has actions, clues and hooks (MR-015, MR-029). A scene
		// still open in a session then reads as closed: it is not a scene
		// point anymore. What players already received stays in their notes.
		if err := q.DeleteSceneActionsOfPoint(ctx, p.ID); err != nil {
			return mapsdb.MapPoint{}, fmt.Errorf("delete the scene's actions: %w", err)
		}
		if err := q.DeleteSceneCluesOfPoint(ctx, p.ID); err != nil {
			return mapsdb.MapPoint{}, fmt.Errorf("delete the scene's clues: %w", err)
		}
		params.Hooks = ""
	}
	if params.Kind == scene && params.RevealedAt != nil && (p.RevealedAt == nil || p.Kind != scene) {
		// Revealing a scene makes it "discovered" (MR-030, question 61): the
		// group may tag notes with it from now on, even if it is hidden again.
		if err := q.UpsertSceneDiscovery(ctx, mapsdb.UpsertSceneDiscoveryParams{CampaignID: campaignID, PointID: p.ID, DiscoveredAt: params.Now}); err != nil {
			return mapsdb.MapPoint{}, fmt.Errorf("record the scene's discovery: %w", err)
		}
	}
	out, err := q.UpdateMapPoint(ctx, params)
	if err != nil {
		return mapsdb.MapPoint{}, fmt.Errorf("update point: %w", err)
	}
	return out, nil
}

// DeleteMapPoint implements mapsv1connect.MapServiceHandler.
func (s *Service) DeleteMapPoint(
	ctx context.Context,
	req *connect.Request[mapsv1.DeleteMapPointRequest],
) (*connect.Response[mapsv1.DeleteMapPointResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	mapID, ok := parseID(req.Msg.GetMapId())
	if !ok {
		return nil, errMapNotFound()
	}
	pointID, ok := parseID(req.Msg.GetPointId())
	if !ok {
		return nil, errPointNotFound()
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	openScene, err := s.live.OpenScenePoint(ctx, m.CampaignID) // the foreign key closes it with the point
	if err != nil {
		return nil, s.dbError(ctx, "read the open scene", err)
	}

	var mapRow mapsdb.Map
	var deleted mapsdb.MapPoint
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		if mapRow, err = s.campaignMap(ctx, q, m.CampaignID, mapID); err != nil {
			return err
		}
		deleted, err = q.DeleteMapPoint(ctx, mapsdb.DeleteMapPointParams{MapID: mapID, ID: pointID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errPointNotFound()
		}
		if err != nil {
			return fmt.Errorf("delete point: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "delete a point", err)
	}
	s.publishMapChanged(m.CampaignID, mapID, playersSee(mapID, mapRow.RevealedAt, current) && deleted.RevealedAt != nil)
	if openScene == pointID {
		s.publishSceneChanged(m.CampaignID)
	}
	return connect.NewResponse(&mapsv1.DeleteMapPointResponse{}), nil
}

// SetMapPointRevealed implements mapsv1connect.MapServiceHandler.
func (s *Service) SetMapPointRevealed(
	ctx context.Context,
	req *connect.Request[mapsv1.SetMapPointRevealedRequest],
) (*connect.Response[mapsv1.SetMapPointRevealedResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	mapID, ok := parseID(req.Msg.GetMapId())
	if !ok {
		return nil, errMapNotFound()
	}
	pointID, ok := parseID(req.Msg.GetPointId())
	if !ok {
		return nil, errPointNotFound()
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	revealed := req.Msg.GetRevealed()

	var mapRow mapsdb.Map
	var before, after mapsdb.MapPoint
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		if mapRow, err = s.campaignMap(ctx, q, m.CampaignID, mapID); err != nil {
			return err
		}
		before, err = q.GetMapPointForUpdate(ctx, mapsdb.GetMapPointForUpdateParams{MapID: mapID, ID: pointID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errPointNotFound()
		}
		if err != nil {
			return fmt.Errorf("find point: %w", err)
		}
		after, err = s.applyPointChange(ctx, q, m.CampaignID, before, pointChange{revealed: &revealed})
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "reveal or hide a point", err)
	}
	s.publishMapChanged(m.CampaignID, mapID,
		playersSee(mapID, mapRow.RevealedAt, current) && (before.RevealedAt != nil || after.RevealedAt != nil))
	out, err := s.masterPoint(ctx, m, after)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&mapsv1.SetMapPointRevealedResponse{Point: out}), nil
}

// PlaceMapToken implements mapsv1connect.MapServiceHandler.
func (s *Service) PlaceMapToken(
	ctx context.Context,
	req *connect.Request[mapsv1.PlaceMapTokenRequest],
) (*connect.Response[mapsv1.PlaceMapTokenResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	mapID, ok := parseID(req.Msg.GetMapId())
	if !ok {
		return nil, errMapNotFound()
	}
	if err := checkPosition(req.Msg.GetXBp(), req.Msg.GetYBp()); err != nil {
		return nil, err
	}
	character, err := s.livingCharacter(ctx, m.CampaignID, req.Msg.GetCharacterId())
	if err != nil {
		return nil, err
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}

	var mapRow mapsdb.Map
	var token mapsdb.MapToken
	var placed bool // a new token, not a move
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		placed = false
		var err error
		if mapRow, err = s.campaignMap(ctx, q, m.CampaignID, mapID); err != nil {
			return err
		}
		key := mapsdb.GetMapTokenForUpdateParams{MapID: mapID, CharacterID: character.GetId()}
		_, err = q.GetMapTokenForUpdate(ctx, key)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// A player's character starts visible, an NPC hidden (question
			// 31 for Samuel): the master reveals an NPC when the group meets
			// it.
			token, err = q.InsertMapToken(ctx, mapsdb.InsertMapTokenParams{
				MapID: mapID, CharacterID: character.GetId(), XBp: req.Msg.GetXBp(), YBp: req.Msg.GetYBp(),
				Hidden:    character.GetKind() != charactersv1.CharacterKind_CHARACTER_KIND_PLAYER,
				UpdatedAt: s.now(),
			})
			placed = true
		case err == nil:
			token, err = q.MoveMapToken(ctx, mapsdb.MoveMapTokenParams{
				MapID: mapID, CharacterID: character.GetId(), XBp: req.Msg.GetXBp(), YBp: req.Msg.GetYBp(), UpdatedAt: s.now(),
			})
		}
		if err != nil {
			return fmt.Errorf("place token: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "place a token", err)
	}
	players := playersSee(mapID, mapRow.RevealedAt, current) && !token.Hidden
	if placed {
		// A new token needs its name: the app reads the map again.
		s.publishMapChanged(m.CampaignID, mapID, players)
	} else {
		s.publishTokenMoved(m.CampaignID, token, players)
	}
	return connect.NewResponse(&mapsv1.PlaceMapTokenResponse{Token: tokenToProto(token, character, newViewer(m, current))}), nil
}

// SetMapTokenHidden implements mapsv1connect.MapServiceHandler.
func (s *Service) SetMapTokenHidden(
	ctx context.Context,
	req *connect.Request[mapsv1.SetMapTokenHiddenRequest],
) (*connect.Response[mapsv1.SetMapTokenHiddenResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	mapID, ok := parseID(req.Msg.GetMapId())
	if !ok {
		return nil, errMapNotFound()
	}
	character, err := s.livingCharacter(ctx, m.CampaignID, req.Msg.GetCharacterId())
	if err != nil {
		return nil, err // a dead character's token is not listed either
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}

	var mapRow mapsdb.Map
	var before, after mapsdb.MapToken
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		if mapRow, err = s.campaignMap(ctx, q, m.CampaignID, mapID); err != nil {
			return err
		}
		before, err = q.GetMapTokenForUpdate(ctx, mapsdb.GetMapTokenForUpdateParams{MapID: mapID, CharacterID: character.GetId()})
		if errors.Is(err, pgx.ErrNoRows) {
			return errTokenNotFound()
		}
		if err != nil {
			return fmt.Errorf("find token: %w", err)
		}
		after, err = q.SetMapTokenHidden(ctx, mapsdb.SetMapTokenHiddenParams{
			MapID: mapID, CharacterID: character.GetId(), Hidden: req.Msg.GetHidden(), Now: s.now(),
		})
		if err != nil {
			return fmt.Errorf("hide or show token: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "hide or show a token", err)
	}
	s.publishMapChanged(m.CampaignID, mapID, playersSee(mapID, mapRow.RevealedAt, current) && (!before.Hidden || !after.Hidden))
	return connect.NewResponse(&mapsv1.SetMapTokenHiddenResponse{Token: tokenToProto(after, character, newViewer(m, current))}), nil
}

// RemoveMapToken implements mapsv1connect.MapServiceHandler. It does not
// ask whether the character still lives: a dead character's token can be
// taken off too.
func (s *Service) RemoveMapToken(
	ctx context.Context,
	req *connect.Request[mapsv1.RemoveMapTokenRequest],
) (*connect.Response[mapsv1.RemoveMapTokenResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	mapID, ok := parseID(req.Msg.GetMapId())
	if !ok {
		return nil, errMapNotFound()
	}
	characterID, ok := parseID(req.Msg.GetCharacterId())
	if !ok {
		return nil, errTokenNotFound()
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}

	var mapRow mapsdb.Map
	var deleted mapsdb.MapToken
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		if mapRow, err = s.campaignMap(ctx, q, m.CampaignID, mapID); err != nil {
			return err
		}
		deleted, err = q.DeleteMapToken(ctx, mapsdb.DeleteMapTokenParams{MapID: mapID, CharacterID: characterID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errTokenNotFound()
		}
		if err != nil {
			return fmt.Errorf("remove token: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "remove a token", err)
	}
	s.publishMapChanged(m.CampaignID, mapID, playersSee(mapID, mapRow.RevealedAt, current) && !deleted.Hidden)
	return connect.NewResponse(&mapsv1.RemoveMapTokenResponse{}), nil
}

// Helpers.

// viewerOf reads the session's current map, which a player sees too.
func (s *Service) viewerOf(ctx context.Context, m authz.Membership) (viewer, error) {
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return viewer{}, err
	}
	return newViewer(m, current), nil
}

// currentMap asks the live session for the current map.
func (s *Service) currentMap(ctx context.Context, campaignID string) (string, error) {
	current, _, err := s.live.OnScreen(ctx, campaignID)
	if err != nil {
		return "", s.dbError(ctx, "read the current map", err)
	}
	return current, nil
}

// masterMap reads a map again, as the master sees it, for a write's
// answer.
func (s *Service) masterMap(ctx context.Context, m authz.Membership, mapID string) (*mapsv1.Map, error) {
	v, err := s.viewerOf(ctx, m)
	if err != nil {
		return nil, err
	}
	cm, err := s.loadMaps(ctx, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read a map", err)
	}
	row, ok := cm.byID[mapID]
	if !ok {
		return nil, errMapNotFound() // deleted right after the change
	}
	return cm.mapToProto(row, v), nil
}

// masterPoint builds a write's answer: the point as the master sees it,
// with its target's name.
func (s *Service) masterPoint(ctx context.Context, m authz.Membership, p mapsdb.MapPoint) (*mapsv1.MapPoint, error) {
	cm, err := s.loadMaps(ctx, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read the maps", err)
	}
	out := cm.pointToProto(p, viewer{master: true, userID: m.UserID})
	if err := s.attachActions(ctx, p.MapID, []*mapsv1.MapPoint{out}, true); err != nil {
		return nil, s.dbError(ctx, "list a point's scene actions", err)
	}
	if err := s.attachClues(ctx, m.CampaignID, p.MapID, []*mapsv1.MapPoint{out}, true); err != nil {
		return nil, s.dbError(ctx, "list a point's scene clues", err)
	}
	return out, nil
}

// campaignMap returns the map, or `not_found` when it is not the
// campaign's.
func (s *Service) campaignMap(ctx context.Context, q *mapsdb.Queries, campaignID, mapID string) (mapsdb.Map, error) {
	row, err := q.GetMap(ctx, mapsdb.GetMapParams{CampaignID: campaignID, ID: mapID})
	if errors.Is(err, pgx.ErrNoRows) {
		return mapsdb.Map{}, errMapNotFound()
	}
	if err != nil {
		return mapsdb.Map{}, fmt.Errorf("find map: %w", err)
	}
	return row, nil
}

// livingCharacter returns a living character of the campaign, or
// `not_found`.
func (s *Service) livingCharacter(ctx context.Context, campaignID, characterID string) (*charactersv1.CharacterSummary, error) {
	id, ok := parseID(characterID)
	if !ok {
		return nil, errCharacterNotFound()
	}
	found, err := s.characters.MapCharacters(ctx, campaignID, []string{id})
	if err != nil {
		return nil, s.dbError(ctx, "find a character", err)
	}
	if len(found) != 1 {
		return nil, errCharacterNotFound()
	}
	return found[0], nil
}

// checkImage checks that an image is in the campaign's gallery.
func checkImage(ctx context.Context, q *mapsdb.Queries, campaignID, imageID string) error {
	_, err := q.GetGalleryImageInCampaign(ctx, mapsdb.GetGalleryImageInCampaignParams{CampaignID: campaignID, ID: imageID})
	if errors.Is(err, pgx.ErrNoRows) {
		return errNotAGalleryImage()
	}
	if err != nil {
		return fmt.Errorf("find image: %w", err)
	}
	return nil
}

// leads says whether a point of this kind (as the database stores it) may
// lead to another map: a submap to the map it opens, a battle to the map
// of its fight (MR-013).
func leads(kind string) bool {
	return kind == kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP] || kind == kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE]
}

// parseTarget reads a point's target from a request: nil for none. Only a
// submap or a battle leads somewhere, and never to its own map.
func parseTarget(raw, kind, mapID string) (*string, error) {
	if raw == "" {
		return nil, nil
	}
	if !leads(kind) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target_map_id is only for a SUBMAP or a BATTLE point"))
	}
	id, ok := parseID(raw)
	if !ok {
		return nil, errNotACampaignMap()
	}
	if id == mapID {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target_map_id must be another map, not the point's own"))
	}
	return &id, nil
}

// checkTarget checks that a point's target (submap or battle) is a map of
// the campaign.
func checkTarget(ctx context.Context, q *mapsdb.Queries, campaignID string, target *string) error {
	if target == nil {
		return nil
	}
	_, err := q.GetMap(ctx, mapsdb.GetMapParams{CampaignID: campaignID, ID: *target})
	if errors.Is(err, pgx.ErrNoRows) {
		return errNotACampaignMap()
	}
	if err != nil {
		return fmt.Errorf("find the target map: %w", err)
	}
	return nil
}

// revealedAt is a point's revealed_at after a reveal or hide: revealing
// keeps the first time.
func revealedAt(was *time.Time, revealed bool, now time.Time) *time.Time {
	switch {
	case !revealed:
		return nil
	case was != nil:
		return was
	default:
		return &now
	}
}

// cleanName checks a map or point name. The message says the rule, never
// the name, which is free text.
func cleanName(field, raw string) (string, error) {
	name, err := names.Clean(raw, maxNameLength)
	if err != nil {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s %w", field, err))
	}
	return name, nil
}

// cleanDescription checks a point's description: several lines allowed.
func cleanDescription(raw string) (string, error) {
	description, err := names.CleanText(raw, maxDescriptionLength)
	if err != nil {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("description %w", err))
	}
	return description, nil
}

// cleanHooks checks a point's hooks (MR-029): the master's private Markdown,
// several lines allowed, empty for none.
func cleanHooks(raw string) (string, error) {
	hooks, err := names.CleanText(raw, maxHooksLength)
	if err != nil {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("hooks %w", err))
	}
	return hooks, nil
}

func errOnlyScenesHaveHooks() error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New("only a SCENE point has hooks"))
}

// checkPosition checks a position in basis points.
func checkPosition(x, y int32) error {
	if x < 0 || x > maxPosition || y < 0 || y > maxPosition {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("x_bp and y_bp must be 0 to %d", maxPosition))
	}
	return nil
}

// parseID returns an ID in canonical form, or false when it is not a UUID:
// such an ID names nothing.
func parseID(raw string) (string, bool) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", false
	}
	return id.String(), true
}

// The answers for what is not in the campaign. Their texts are the same
// whether the thing exists or not.
func errMapNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("map not found"))
}

func errPointNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("point not found"))
}

func errTokenNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("token not found"))
}

func errCharacterNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("character not found"))
}

func errNotAGalleryImage() error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New("image_id is not an image of the campaign's gallery"))
}

func errNotACampaignMap() error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New("target_map_id is not a map of the campaign"))
}

// errStaleMap is the answer when the map changed since the client read it
// (AIP-154).
func errStaleMap() error {
	return connect.NewError(connect.CodeAborted, errors.New("the map changed since you opened it; reload it and try again"))
}
