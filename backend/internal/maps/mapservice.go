package maps

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
	"github.com/PuraFome/meuRPG/backend/internal/platform/uid"
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

// int32PtrValue is the number a nullable column holds, 0 for NULL.
func int32PtrValue(n *int32) int32 {
	if n == nil {
		return 0
	}
	return *n
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

// lockMap locks the campaign's map row for the rest of the transaction, or answers
// `not_found`.
func lockMap(ctx context.Context, q *mapsdb.Queries, campaignID, mapID string) (mapsdb.Map, error) {
	row, err := q.GetMapForUpdate(ctx, mapsdb.GetMapForUpdateParams{CampaignID: campaignID, ID: mapID})
	if errors.Is(err, pgx.ErrNoRows) {
		return row, errMapNotFound()
	}
	if err != nil {
		return row, fmt.Errorf("find map: %w", err)
	}
	return row, nil
}

// livingCharacter returns a living character of the campaign, or
// `not_found`.
func (s *Service) livingCharacter(ctx context.Context, campaignID, characterID string) (*charactersv1.CharacterSummary, error) {
	id, ok := uid.Canonical(characterID)
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
	id, ok := uid.Canonical(raw)
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
