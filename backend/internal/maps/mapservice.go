package maps

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
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
		mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE:   "battle",
		mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP:   "submap",
		mapsv1.MapPointKind_MAP_POINT_KIND_SCENE:    "scene",
		mapsv1.MapPointKind_MAP_POINT_KIND_TRAP:     "trap",
		mapsv1.MapPointKind_MAP_POINT_KIND_TREASURE: "treasure",
		mapsv1.MapPointKind_MAP_POINT_KIND_LIGHT:    "light",
	}
	kindFromDB = map[string]mapsv1.MapPointKind{
		"battle":   mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE,
		"submap":   mapsv1.MapPointKind_MAP_POINT_KIND_SUBMAP,
		"scene":    mapsv1.MapPointKind_MAP_POINT_KIND_SCENE,
		"trap":     mapsv1.MapPointKind_MAP_POINT_KIND_TRAP,
		"treasure": mapsv1.MapPointKind_MAP_POINT_KIND_TREASURE,
		"light":    mapsv1.MapPointKind_MAP_POINT_KIND_LIGHT,
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
	if !v.master {
		v.parentKnows = s.parentKnows(ctx, cm, v)
	}
	ctx = withPartyMemo(ctx) // the party's senses are read once for all the fog maps
	res := &mapsv1.ListMapsResponse{}
	for _, r := range cm.rows {
		if !v.seesMap(r.ID, r.RevealedAt) {
			continue
		}
		out := cm.mapToProto(r, v)
		if foggedFor(v, r) {
			// What a player counts on a fog map is what they receive of it (RN-10).
			pv, points, _, err := s.fogViewOf(ctx, r, v)
			if err != nil {
				return nil, s.dbError(ctx, "work out what a player sees", err)
			}
			key := countKey{mapID: r.ID, userID: v.userID, revision: pv.revision(), points: pointsSig(points), traps: trapsSig(v.traps)}
			n, ok := s.counts.get(key)
			if !ok {
				n = int32(len(s.visiblePoints(points, v, pv))) //nolint:gosec // G115: a map has at most 200 points
				s.counts.put(key, n)
			}
			out.PointCount = n
		}
		res.Maps = append(res.Maps, out)
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
	v, err := s.readerOf(ctx, m, req.Msg.GetAsCharacterId())
	if err != nil {
		return nil, err
	}
	snap, err := s.readMapSnapshot(ctx, m.CampaignID, mapID, v)
	if err != nil {
		return nil, s.dbError(ctx, "get a map", err)
	}
	if !v.master {
		v.parentKnows = s.parentKnows(ctx, snap.cm, v)
	}
	res := &mapsv1.GetMapResponse{Map: snap.cm.mapToProto(snap.row, v)}
	pv, err := s.mapFogView(ctx, snap, v)
	if err != nil {
		return nil, err
	}
	if err := s.fillMapPoints(ctx, m.CampaignID, mapID, res, snap, v, pv); err != nil {
		return nil, err
	}
	if res.Tokens, err = s.mapTokensFor(ctx, m.CampaignID, mapID, snap, v, pv); err != nil {
		return nil, err
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
	fogOn, err := s.fogOnNewMaps(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	reuse, err := s.prepareReuseCopy(ctx, m.CampaignID, imageID)
	if err != nil {
		return nil, s.dbError(ctx, "copy the image of a fog map", err)
	}
	// The key is unique in the campaign, and kept with a hash of the whole request: a retry
	// returns the first map only when it is the same request.
	scopedKey, requestHash, err := keyOf(m.CampaignID, req.Msg.GetIdempotencyKey(), req.Msg)
	if err != nil {
		return nil, err
	}
	var created mapsdb.Map
	var used, replayed bool
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		created, replayed, used, err = s.createMapTx(ctx, tx, newMap{
			campaignID: m.CampaignID, userID: m.UserID, name: name, imageID: imageID,
			fogOn: fogOn, reuse: reuse, key: scopedKey, hash: requestHash,
		})
		return err
	})
	s.discardFogCopy(ctx, m.CampaignID, reuse, err != nil || !used)
	if err != nil {
		return nil, s.dbError(ctx, "create a map", err)
	}
	// A new map is hidden: only the master hears about it. A retry announces nothing: the
	// first call did.
	if !replayed {
		s.publishMapChanged(m.CampaignID, created.ID, false)
	}
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
	update, err := checkMapUpdate(req.Msg)
	if err != nil {
		return nil, err
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	var imageCopy *fogCopy
	var fogSeen bool
	if update.imageID != nil {
		if imageCopy, fogSeen, err = s.imageCopyFor(ctx, m.CampaignID, mapID, *update.imageID); err != nil {
			return nil, s.dbError(ctx, "copy the map's image", err)
		}
	}
	var w mapUpdateWrite
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		w, err = s.writeMapUpdate(ctx, tx, mapUpdateTx{mapUpdate: update, campaignID: m.CampaignID, userID: m.UserID, mapID: mapID, imageCopy: imageCopy, fogSeen: fogSeen})
		return err
	})
	s.discardFogCopy(ctx, m.CampaignID, imageCopy, err != nil || !w.copied)
	if err != nil {
		return nil, s.dbError(ctx, "update a map", err)
	}
	s.announceMapUpdate(ctx, m, mapID, current, w)
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
	pre, err := s.readDeleteContext(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	var deleted mapsdb.Map
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		deleted, err = s.deleteMapRow(ctx, tx, m.CampaignID, mapID)
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "delete a map", err)
	}
	s.announceDeletedMap(ctx, m.CampaignID, mapID, pre, deleted)
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
		var err error
		before, after, err = s.revealMapRow(ctx, tx, m, mapID, req.Msg.GetRevealed())
		return err
	})
	if err != nil {
		return nil, s.dbError(ctx, "reveal or hide a map", err)
	}
	s.announceRevealedMap(ctx, m.CampaignID, mapID, current, before, after)
	out, err := s.masterMap(ctx, m, mapID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&mapsv1.SetMapRevealedResponse{Map: out}), nil
}

// The battle grid's limits (MR-013): how many squares of the drawing fit across
// the image's width. The maps_grid_columns_valid CHECK says the same of the
// rules' columns, which are at most 200 as well (the calibration multiplies the
// drawn columns, and rules/grid.EngineGrid refuses what passes the limits).
const (
	minGridColumns = 4
	maxGridColumns = grid.MaxColumns // 200, the rules' limit too
)

// SetMapGrid implements mapsv1connect.MapServiceHandler. calibration.go says
// what it does to the painted layers.
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
	drawn := req.Msg.GetColumns()
	factor, err := checkGridRequest(drawn, req.Msg.GetSquareFactor())
	if err != nil {
		return nil, err
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	imageCopy, err := s.gridFogCopy(ctx, m.CampaignID, mapID, drawn)
	if err != nil {
		return nil, err
	}
	var w gridWrite
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		w, err = s.writeGrid(ctx, tx, gridChange{campaignID: m.CampaignID, userID: m.UserID, mapID: mapID, drawn: drawn, factor: factor, imageCopy: imageCopy})
		return err
	})
	s.discardFogCopy(ctx, m.CampaignID, imageCopy, err != nil || !w.copied)
	if err != nil {
		return nil, s.dbError(ctx, "set a map's grid", err)
	}
	s.announceGrid(ctx, m, mapID, current, w)
	out, err := s.masterMap(ctx, m, mapID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&mapsv1.SetMapGridResponse{Map: out}), nil
}

// int32PtrValue is the number a nullable column holds, 0 for NULL.
func int32PtrValue(n *int32) int32 {
	if n == nil {
		return 0
	}
	return *n
}

// pointChange is what the master asks to change in a point. Nil fields
// stay as they are.
type pointChange struct {
	kind        *string
	name        *string
	description *string
	hooks       *string // a SCENE point's private text; "" clears it
	showDC      *bool   // a SCENE point's "Mostrar a CD aos jogadores"
	x, y        *int32
	target      *string // "" removes the target
	revealed    *bool
	trap        *mapsv1.TrapSpec
	treasure    *int32
	light       *mapsv1.LightSpec
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
	in, err := s.checkNewPoint(req.Msg)
	if err != nil {
		return nil, err
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	// The key is unique in the campaign (it shares the column of "Pôr no mapa"), and kept with a
	// hash of the whole request: a retry returns the first point only when it is the same request.
	scopedKey, requestHash, err := keyOf(m.CampaignID, req.Msg.GetIdempotencyKey(), req.Msg)
	if err != nil {
		return nil, err
	}
	created, mapRow, replayed, err := s.createPointTx(ctx, m.CampaignID, in, scopedKey, requestHash)
	if err != nil {
		return nil, s.dbError(ctx, "create a point", err)
	}
	// A new point is hidden: only the master hears about it (MR-009), unless it
	// is born visible to everyone (a trap created already triggered). A retry announces
	// nothing: the first call did.
	if !replayed {
		s.publishPointsChanged(ctx, m.CampaignID, mapRow, playersSee(in.mapID, mapRow.RevealedAt, current) && everyoneSees(created), created)
	}
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
	change, err := pointChangeOf(req.Msg)
	if err != nil {
		return nil, err
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	ch, err := s.changePointTx(ctx, m.CampaignID, mapID, pointID, change)
	if err != nil {
		return nil, s.dbError(ctx, "update a point", err)
	}
	s.announcePointChange(ctx, m.CampaignID, mapID, current, ch)
	// The open scene shows the point's name and description, and stops being
	// one when the point changes kind.
	s.publishSceneChangedIf(ctx, m.CampaignID, pointID)
	out, err := s.masterPoint(ctx, m, ch.after)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&mapsv1.UpdateMapPointResponse{Point: out}), nil
}

// applyPointChange checks a change against the point as it is, inside the
// transaction, and saves it.
func (s *Service) applyPointChange(ctx context.Context, q *mapsdb.Queries, campaignID string, p mapsdb.MapPoint, c pointChange) (mapsdb.MapPoint, error) {
	params := s.baseUpdateParams(p)
	c.overlay(&params)
	if err := resolvePointTarget(ctx, q, campaignID, p, c, &params); err != nil {
		return mapsdb.MapPoint{}, err
	}
	if c.revealed != nil {
		params.RevealedAt = revealedAt(p.RevealedAt, *c.revealed, params.Now)
	}
	if err := s.applySpecChange(ctx, q, p, c, &params); err != nil {
		return mapsdb.MapPoint{}, err
	}
	if err := checkChangeSceneOnlyData(params.Kind, c); err != nil {
		return mapsdb.MapPoint{}, err
	}
	if err := leaveScene(ctx, q, p, &params); err != nil {
		return mapsdb.MapPoint{}, err
	}
	if err := discoverScene(ctx, q, campaignID, p, params); err != nil {
		return mapsdb.MapPoint{}, err
	}
	out, err := q.UpdateMapPoint(ctx, params)
	if err != nil {
		return mapsdb.MapPoint{}, fmt.Errorf("update point: %w", err)
	}
	return out, nil
}

// applySpecChange works out the data of the new kinds (a trap, a treasure's
// value, a light) for the point after the change, and cleans up what a point
// that changes kind leaves behind. Each kind needs its own data, and refuses
// another's; a point keeps what it has unless the request replaces it.
func (s *Service) applySpecChange(ctx context.Context, q *mapsdb.Queries, p mapsdb.MapPoint, c pointChange, params *mapsdb.UpdateMapPointParams) error {
	if err := s.applyTrapChange(ctx, q, p, c, params); err != nil {
		return err
	}
	if err := applyTreasureChange(ctx, q, p, c, params); err != nil {
		return err
	}
	return s.applyLightChange(p, c, params)
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
	gone, err := s.deletePointTx(ctx, m.CampaignID, mapID, pointID)
	if err != nil {
		return nil, s.dbError(ctx, "delete a point", err)
	}
	s.publishPointsChanged(ctx, m.CampaignID, gone.mapRow, playersSee(mapID, gone.mapRow.RevealedAt, current) && everyoneSees(gone.deleted), gone.deleted)
	s.tellTrapKnowers(m.CampaignID, mapID, gone.mapRow, current, gone.knowers)
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
	ch, err := s.changePointTx(ctx, m.CampaignID, mapID, pointID, pointChange{revealed: &revealed})
	if err != nil {
		return nil, s.dbError(ctx, "reveal or hide a point", err)
	}
	s.announcePointChange(ctx, m.CampaignID, mapID, current, ch)
	out, err := s.masterPoint(ctx, m, ch.after)
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
	if err := tokenSubject(req.Msg.GetCharacterId(), req.Msg.GetCreatureId()); err != nil {
		return nil, err
	}
	if req.Msg.GetCreatureId() != "" {
		res, err := s.placeCreatureToken(ctx, m, req.Msg, mapID)
		if err != nil {
			return nil, err
		}
		return connect.NewResponse(res), nil
	}
	character, err := s.livingCharacter(ctx, m.CampaignID, req.Msg.GetCharacterId())
	if err != nil {
		return nil, err
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	pl, err := s.placeTokenTx(ctx, m.CampaignID, mapID, req.Msg.GetXBp(), req.Msg.GetYBp(), character)
	if err != nil {
		return nil, s.dbError(ctx, "place a token", err)
	}
	s.announceTokenPlaced(ctx, m, mapID, current, pl, character)
	return connect.NewResponse(&mapsv1.PlaceMapTokenResponse{Token: tokenToProto(pl.token, character, newViewer(m, current))}), nil
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
	s.publishTokenWritten(ctx, m.CampaignID, mapRow, current, &before, &after, isPlayerCharacter(character), false)
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
	if err := tokenSubject(req.Msg.GetCharacterId(), req.Msg.GetCreatureId()); err != nil {
		return nil, err
	}
	if req.Msg.GetCreatureId() != "" {
		if err := s.removeCreatureToken(ctx, m, mapID, req.Msg.GetCreatureId()); err != nil {
			return nil, err
		}
		return connect.NewResponse(&mapsv1.RemoveMapTokenResponse{}), nil
	}
	characterID, ok := parseID(req.Msg.GetCharacterId())
	if !ok {
		return nil, errTokenNotFound()
	}
	if err := s.removeCharacterToken(ctx, m, mapID, characterID); err != nil {
		return nil, err
	}
	return connect.NewResponse(&mapsv1.RemoveMapTokenResponse{}), nil
}

// Helpers.

// viewerOf reads the session's current map, which a player sees too.
func (s *Service) viewerOf(ctx context.Context, m authz.Membership) (viewer, error) {
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return viewer{}, err
	}
	v := newViewer(m, current)
	if !v.master {
		if v.traps, err = s.knownTraps(ctx, m.CampaignID, m.UserID); err != nil {
			return viewer{}, s.dbError(ctx, "read the traps a player knows", err)
		}
	}
	return v, nil
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
	out := s.pointToProto(cm, p, viewer{master: true, userID: m.UserID})
	details, err := readPointDetails(ctx, s.queries, p.MapID, true)
	if err != nil {
		return nil, s.dbError(ctx, "list a point's traps and treasures", err)
	}
	if err := s.attachPointDetails(ctx, m.CampaignID, []*mapsv1.MapPoint{out}, details); err != nil {
		return nil, s.dbError(ctx, "list a point's traps and treasures", err)
	}
	if err := s.attachActions(ctx, p.MapID, []*mapsv1.MapPoint{out}, true); err != nil {
		return nil, s.dbError(ctx, "list a point's scene actions", err)
	}
	if err := s.attachImages(ctx, p.MapID, []*mapsv1.MapPoint{out}, true); err != nil {
		return nil, s.dbError(ctx, "list a point's scene images", err)
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
	found, err := s.characters.MapCharacters(ctx, nil, campaignID, []string{id})
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

func errOnlyScenesShowDC() error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New("only a SCENE point shows a DC to the players"))
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

// keyOf checks a create's idempotency key and returns it as it is stored (the campaign's ID and
// the key; nil for no key) with the hash of the whole request, for idem.Create.
func keyOf(campaignID, key string, msg proto.Message) (scoped, hash *string, err error) {
	key, err = idem.Clean(key)
	if err != nil {
		return nil, nil, err
	}
	return idem.Scope(campaignID, key), idem.Hash(msg), nil
}
