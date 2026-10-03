package maps

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
)

// Who sees what on a map (RN-10, MR-009). The master sees everything. A
// player sees:
//
//	a map      when it is revealed, or it is the open session's current map
//	a point    when it is revealed, on a map they see
//	a token    when it is not hidden, on a map they see
//	a target   (where a submap point leads) when they see the target map
//
// Everything a player may not see is left out of the response, never
// blanked: its ID, name and description never leave the server. The same
// rules decide who hears about a change on the live stream: a player hears
// about it only when it touches something they see, before or after it
// (publish, below).

// viewer is who reads the maps: the caller, and the open session's current
// map, which a player sees even when it is hidden.
type viewer struct {
	master     bool
	userID     string
	currentMap string // "" when there is no open session or no current map
}

func newViewer(m authz.Membership, currentMap string) viewer {
	return viewer{master: m.Role == authz.RoleMaster, userID: m.UserID, currentMap: currentMap}
}

// playersSee says whether the players see a map: revealed, or the current
// one.
func playersSee(id string, revealedAt *time.Time, currentMap string) bool {
	return revealedAt != nil || (currentMap != "" && id == currentMap)
}

// seesMap says whether the viewer sees a map.
func (v viewer) seesMap(id string, revealedAt *time.Time) bool {
	return v.master || playersSee(id, revealedAt, v.currentMap)
}

// seesPoint says whether the viewer sees a point of a map they see.
func (v viewer) seesPoint(p mapsdb.MapPoint) bool {
	return v.master || p.RevealedAt != nil
}

// seesToken says whether the viewer sees a token of a map they see.
func (v viewer) seesToken(t mapsdb.MapToken) bool {
	return v.master || !t.Hidden
}

// campaignMaps is what the responses need to know about a campaign's maps,
// read at once: each map with its image and point counts, the submap links
// between them, and the current map.
type campaignMaps struct {
	rows  []mapsdb.ListMapDetailsRow // oldest first
	byID  map[string]mapsdb.ListMapDetailsRow
	links []mapsdb.ListSubmapLinksRow
}

// loadMaps reads the campaign's maps. A campaign has at most 200, so
// reading them all is cheap, and it gives every response the names and
// states it needs (parents, submap targets) in two queries.
func (s *Service) loadMaps(ctx context.Context, campaignID string) (campaignMaps, error) {
	rows, err := s.queries.ListMapDetails(ctx, campaignID)
	if err != nil {
		return campaignMaps{}, err
	}
	links, err := s.queries.ListSubmapLinks(ctx, campaignID)
	if err != nil {
		return campaignMaps{}, err
	}
	cm := campaignMaps{rows: rows, byID: make(map[string]mapsdb.ListMapDetailsRow, len(rows)), links: links}
	for _, r := range rows {
		cm.byID[r.ID] = r
	}
	return cm, nil
}

// mapToProto builds the Map the viewer sees. The caller checked that the
// viewer sees it.
func (cm campaignMaps) mapToProto(r mapsdb.ListMapDetailsRow, v viewer) *mapsv1.Map {
	out := &mapsv1.Map{
		Id:         r.ID,
		CampaignId: r.CampaignID,
		Name:       r.Name,
		Image: &mapsv1.MapImage{
			Id:           r.ImageID,
			Url:          imageURL(r.ImageID),
			ThumbnailUrl: thumbnailURL(r.ImageID),
			Width:        r.ImageWidth,
			Height:       r.ImageHeight,
		},
		Revealed:   r.RevealedAt != nil,
		Current:    v.currentMap != "" && r.ID == v.currentMap,
		PointCount: r.RevealedPointCount,
		Revision:   r.Revision,
		CreatedAt:  timestamppb.New(r.CreatedAt),
		UpdatedAt:  timestamppb.New(r.UpdatedAt),
	}
	if r.GridColumns != nil {
		out.GridColumns = *r.GridColumns
		out.GridRows = gridRows(*r.GridColumns, r.ImageWidth, r.ImageHeight)
	}
	if v.master {
		// The gallery is the master's preparation: its names stay with them.
		out.Image.Name = r.ImageName
		out.PointCount = r.PointCount
	}
	// The parents, in the order of the maps list (oldest first), each once.
	// A player sees a parent only through a revealed point on a map they
	// see.
	parents := map[string]bool{}
	for _, l := range cm.links {
		parent, ok := cm.byID[l.MapID]
		if l.TargetMapID == r.ID && ok && (v.master || (l.Revealed && v.seesMap(parent.ID, parent.RevealedAt))) {
			parents[parent.ID] = true
		}
	}
	for _, p := range cm.rows {
		if parents[p.ID] {
			out.ParentMaps = append(out.ParentMaps, &mapsv1.MapRef{Id: p.ID, Name: p.Name})
		}
	}
	return out
}

// pointToProto builds the MapPoint the viewer sees. The caller checked
// that the viewer sees the point. Its target is left out when the viewer
// does not see the target map.
func (cm campaignMaps) pointToProto(p mapsdb.MapPoint, v viewer) *mapsv1.MapPoint {
	out := &mapsv1.MapPoint{
		Id:          p.ID,
		MapId:       p.MapID,
		Kind:        kindFromDB[p.Kind],
		Name:        p.Name,
		Description: p.Description,
		XBp:         p.XBp,
		YBp:         p.YBp,
		Revealed:    p.RevealedAt != nil,
		CreatedAt:   timestamppb.New(p.CreatedAt),
		UpdatedAt:   timestamppb.New(p.UpdatedAt),
	}
	if v.master {
		out.Hooks = p.Hooks // the master's private text (RN-20): never a player's
	}
	if p.TargetMapID != nil {
		if target, ok := cm.byID[*p.TargetMapID]; ok && v.seesMap(target.ID, target.RevealedAt) {
			out.TargetMap = &mapsv1.MapRef{Id: target.ID, Name: target.Name}
		}
	}
	return out
}

// tokenToProto builds the MapToken the viewer sees, with its character.
func tokenToProto(t mapsdb.MapToken, c *charactersv1.CharacterSummary, v viewer) *mapsv1.MapToken {
	return &mapsv1.MapToken{
		MapId:       t.MapID,
		CharacterId: t.CharacterID,
		Name:        c.GetName(),
		Kind:        c.GetKind(),
		Mine:        c.GetPlayerUserId() != "" && c.GetPlayerUserId() == v.userID,
		XBp:         t.XBp,
		YBp:         t.YBp,
		Hidden:      t.Hidden,
		UpdatedAt:   timestamppb.New(t.UpdatedAt),
	}
}

// The live events (play.proto, WatchGameSessionResponse). Each one goes to
// the master, and to the players only when the change touches something
// they see before or after it: then their copy of the map is stale. A
// change to hidden things only reaches the master, so a player's stream
// never mentions them, not even by a map_changed on a map they see.

// publishMapChanged tells the watching members that a map changed: the
// master, and the players when players is true.
func (s *Service) publishMapChanged(campaignID, mapID string, players bool) {
	s.live.Publish(campaignID, players, &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_MapChanged_{
		MapChanged: &playv1.WatchGameSessionResponse_MapChanged{MapId: mapID},
	}})
}

// publishTokenMoved tells the watching members that a token moved.
func (s *Service) publishTokenMoved(campaignID string, t mapsdb.MapToken, players bool) {
	s.live.Publish(campaignID, players, &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_TokenMoved_{
		TokenMoved: &playv1.WatchGameSessionResponse_TokenMoved{MapId: t.MapID, CharacterId: t.CharacterID, XBp: t.XBp, YBp: t.YBp},
	}})
}

// publishCurrentMapCleared tells everyone that the session has no current
// map anymore: its map was deleted.
func (s *Service) publishCurrentMapCleared(campaignID string) {
	s.live.Publish(campaignID, true, &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_CurrentMapChanged_{
		CurrentMapChanged: &playv1.WatchGameSessionResponse_CurrentMapChanged{},
	}})
}

// publishParentsChanged tells the watching members about the maps whose
// submap points lead to target, after target was deleted, revealed or
// hidden: those points gained or lost their target. A player hears about a
// parent they see, with a revealed point, when the target was visible to
// them before or after (targetBefore, targetAfter) and that changed.
func (s *Service) publishParentsChanged(campaignID, target string, cm campaignMaps, currentMap string, targetBefore, targetAfter bool) {
	players := map[string]bool{}
	var order []string
	for _, l := range cm.links {
		if l.TargetMapID != target {
			continue
		}
		parent, ok := cm.byID[l.MapID]
		if !ok {
			continue
		}
		if _, seen := players[parent.ID]; !seen {
			order = append(order, parent.ID)
		}
		players[parent.ID] = players[parent.ID] ||
			(l.Revealed && playersSee(parent.ID, parent.RevealedAt, currentMap) && targetBefore != targetAfter)
	}
	for _, id := range order {
		s.publishMapChanged(campaignID, id, players[id])
	}
}
