package maps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
)

// Traps, treasure and who knows about them (MR-035, MR-041; Etapa 9, D5, D8;
// RN-10).
//
// A trap is hidden from a player until it was revealed to one of their
// characters (a row in map_point_reveals), revealed to everyone (the point's
// revealed_at), or triggered. Slice 9.8 writes the first kind of reveal when a
// character notices or finds the trap in play; here the master writes it
// (RevealTrap). The player of the character is told, and nobody else: not the
// master's other players, and no event or response ever names the trap to them.
//
// A treasure follows the existing reveal rule and, once the master marks it
// found (MarkTreasureFound), everyone who sees its map sees it, with what is
// inside, what it is worth and who found it. Slice 9.11 turns found treasure
// into XP and sets treasure_converted_award_id, after which its value and its
// found mark are frozen.

const (
	eventTrapRevealed    = "trap_revealed"
	eventTreasureFound   = "treasure_found"
	eventTreasureUnfound = "treasure_unfound"
)

// The session events' payloads: IDs and numbers only, never a name or a text
// (docs/privacidade.md).
type trapRevealedEvent struct {
	PointID      string   `json:"point_id"`
	CharacterIDs []string `json:"character_ids,omitempty"`
	All          bool     `json:"all,omitempty"`
}

type treasureEvent struct {
	PointID      string   `json:"point_id"`
	CharacterIDs []string `json:"character_ids,omitempty"`
	ValuePO      int32    `json:"value_po"`
}

// knownTraps reads which traps the player's characters know (RN-10): by point
// ID, with the map each is on. A character that no longer lives, or whose player
// is someone else, tells the player nothing.
func (s *Service) knownTraps(ctx context.Context, campaignID, userID string) (map[string]knownTrap, error) {
	rows, err := s.queries.ListPointRevealsOfCampaign(ctx, campaignID)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	var ids []string
	for _, r := range rows {
		if !slices.Contains(ids, r.CharacterID) {
			ids = append(ids, r.CharacterID)
		}
	}
	chars, err := s.characters.MapCharacters(ctx, nil, campaignID, ids)
	if err != nil {
		return nil, err
	}
	mine := map[string]bool{}
	for _, c := range chars {
		if c.GetPlayerUserId() != "" && c.GetPlayerUserId() == userID {
			mine[c.GetId()] = true
		}
	}
	known := map[string]knownTrap{}
	for _, r := range rows {
		if mine[r.CharacterID] {
			known[r.PointID] = knownTrap{mapID: r.MapID, public: r.Public}
		}
	}
	return known, nil
}

// attachPointDetails fills what a point's read needs a second query for: who
// found each found treasure (everyone who sees the treasure gets it) and, for
// the master, who knows each trap.
func (s *Service) attachPointDetails(ctx context.Context, campaignID, mapID string, points []*mapsv1.MapPoint, master bool) error {
	wantFinders := slices.ContainsFunc(points, func(p *mapsv1.MapPoint) bool { return p.GetTreasureFoundAt() != nil })
	wantReveals := master && slices.ContainsFunc(points, func(p *mapsv1.MapPoint) bool { return p.GetKind() == mapsv1.MapPointKind_MAP_POINT_KIND_TRAP })
	if !wantFinders && !wantReveals {
		return nil
	}
	var finders []mapsdb.MapTreasureFinder
	var reveals []mapsdb.MapPointReveal
	var err error
	if wantFinders {
		if finders, err = s.queries.ListTreasureFindersOfMap(ctx, mapID); err != nil {
			return err
		}
	}
	if wantReveals {
		if reveals, err = s.queries.ListPointRevealsOfMap(ctx, mapID); err != nil {
			return err
		}
	}
	var ids []string
	for _, f := range finders {
		ids = append(ids, f.CharacterID)
	}
	for _, r := range reveals {
		ids = append(ids, r.CharacterID)
	}
	names, order, err := s.characterNames(ctx, campaignID, ids)
	if err != nil {
		return err
	}
	for _, p := range points {
		for _, id := range order { // players' characters first, oldest first
			for _, f := range finders {
				if f.PointID == p.GetId() && f.CharacterID == id && p.GetTreasureFoundAt() != nil {
					p.TreasureFoundBy = append(p.TreasureFoundBy, &mapsv1.TreasureFinder{CharacterId: id, CharacterName: names[id]})
				}
			}
		}
		for _, r := range reveals {
			if r.PointID == p.GetId() {
				p.TrapRevealedTo = append(p.TrapRevealedTo, &mapsv1.TrapReveal{
					CharacterId: r.CharacterID, CharacterName: names[r.CharacterID], How: revealHowFromDB[r.How], At: timestamppb.New(r.At),
				})
			}
		}
	}
	return nil
}

// characterNames reads the names of living characters, and the order the
// characters module lists them in (players first, oldest first).
func (s *Service) characterNames(ctx context.Context, campaignID string, ids []string) (map[string]string, []string, error) {
	names := map[string]string{}
	if len(ids) == 0 {
		return names, nil, nil
	}
	found, err := s.characters.MapCharacters(ctx, nil, campaignID, slices.Compact(slices.Sorted(slices.Values(ids))))
	if err != nil {
		return nil, nil, err
	}
	order := make([]string, 0, len(found))
	for _, c := range found {
		names[c.GetId()] = c.GetName()
		order = append(order, c.GetId())
	}
	return names, order, nil
}

var revealHowFromDB = map[string]mapsv1.TrapRevealHow{
	"noticed":  mapsv1.TrapRevealHow_TRAP_REVEAL_HOW_NOTICED,
	"searched": mapsv1.TrapRevealHow_TRAP_REVEAL_HOW_SEARCHED,
	"master":   mapsv1.TrapRevealHow_TRAP_REVEAL_HOW_MASTER,
}

// playerCharacters checks the characters a master named: each must be a living
// player character of the campaign, and, when needUser, have a player. It
// returns their IDs without repeats, and the user of each. Anything else is
// "not found", as for a token's character.
func (s *Service) playerCharacters(ctx context.Context, campaignID string, raw []string, needUser bool) ([]string, map[string]string, error) {
	if len(raw) == 0 || len(raw) > maxRevealCharacters {
		return nil, nil, badSpec("character_ids must have 1 to %d characters", maxRevealCharacters)
	}
	ids := make([]string, 0, len(raw))
	for _, r := range raw {
		id, ok := parseID(r)
		if !ok {
			return nil, nil, errCharacterNotFound()
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	found, err := s.characters.MapCharacters(ctx, nil, campaignID, ids)
	if err != nil {
		return nil, nil, s.dbError(ctx, "find the characters", err)
	}
	userOf := make(map[string]string, len(found))
	for _, c := range found {
		if c.GetKind() == charactersv1.CharacterKind_CHARACTER_KIND_PLAYER && (!needUser || c.GetPlayerUserId() != "") {
			userOf[c.GetId()] = c.GetPlayerUserId()
		}
	}
	for _, id := range ids {
		if _, ok := userOf[id]; !ok {
			return nil, nil, errCharacterNotFound()
		}
	}
	return ids, userOf, nil
}

// pointCall is the start of every call on one point of a map by the master: the
// membership, the IDs, and a map of the campaign.
func (s *Service) pointCall(ctx context.Context, campaignID, mapID, pointID string) (authz.Membership, string, string, error) {
	m, err := authz.RequireCampaignRole(ctx, campaignID, authz.RoleMaster)
	if err != nil {
		return authz.Membership{}, "", "", err
	}
	mid, ok := parseID(mapID)
	if !ok {
		return authz.Membership{}, "", "", errMapNotFound()
	}
	pid, ok := parseID(pointID)
	if !ok {
		return authz.Membership{}, "", "", errPointNotFound()
	}
	return m, mid, pid, nil
}

// lockPoint reads and locks a point of a map of the campaign, which must be of
// the kind.
func (s *Service) lockPoint(ctx context.Context, q *mapsdb.Queries, campaignID, mapID, pointID string, kind mapsv1.MapPointKind) (mapsdb.Map, mapsdb.MapPoint, error) {
	mapRow, err := s.campaignMap(ctx, q, campaignID, mapID)
	if err != nil {
		return mapsdb.Map{}, mapsdb.MapPoint{}, err
	}
	p, err := q.GetMapPointForUpdate(ctx, mapsdb.GetMapPointForUpdateParams{MapID: mapID, ID: pointID})
	if errors.Is(err, pgx.ErrNoRows) {
		return mapsdb.Map{}, mapsdb.MapPoint{}, errPointNotFound()
	}
	if err != nil {
		return mapsdb.Map{}, mapsdb.MapPoint{}, fmt.Errorf("find point: %w", err)
	}
	if p.Kind != kindToDB[kind] {
		return mapsdb.Map{}, mapsdb.MapPoint{}, badSpec("the point is not a %s point", kind.String()[len("MAP_POINT_KIND_"):])
	}
	return mapRow, p, nil
}

// RevealTrap implements mapsv1connect.MapServiceHandler.
func (s *Service) RevealTrap(
	ctx context.Context,
	req *connect.Request[mapsv1.RevealTrapRequest],
) (*connect.Response[mapsv1.RevealTrapResponse], error) {
	m, mapID, pointID, err := s.pointCall(ctx, req.Msg.GetCampaignId(), req.Msg.GetMapId(), req.Msg.GetPointId())
	if err != nil {
		return nil, err
	}
	all := req.Msg.GetAll()
	if all == (len(req.Msg.GetCharacterIds()) > 0) {
		return nil, badSpec("send character_ids or all, not both and not neither")
	}
	var ids []string
	var userOf map[string]string
	if !all {
		if ids, userOf, err = s.playerCharacters(ctx, m.CampaignID, req.Msg.GetCharacterIds(), true); err != nil {
			return nil, err
		}
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}

	var mapRow mapsdb.Map
	var after mapsdb.MapPoint
	var newUsers, gave []string
	var changed bool
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		newUsers, gave, changed = nil, nil, false
		var before mapsdb.MapPoint
		var err error
		if mapRow, before, err = s.lockPoint(ctx, q, m.CampaignID, mapID, pointID, mapsv1.MapPointKind_MAP_POINT_KIND_TRAP); err != nil {
			return err
		}
		now := s.now()
		after = before
		if all {
			reveal := true
			if after, err = s.applyPointChange(ctx, q, m.CampaignID, before, pointChange{revealed: &reveal}); err != nil {
				return err
			}
			changed = before.RevealedAt == nil
		}
		for _, id := range ids {
			n, err := q.InsertPointReveal(ctx, mapsdb.InsertPointRevealParams{PointID: pointID, CharacterID: id, How: "master", At: now})
			if err != nil {
				return fmt.Errorf("reveal the trap: %w", err)
			}
			if n == 1 {
				gave = append(gave, id)
				if !slices.Contains(newUsers, userOf[id]) {
					newUsers = append(newUsers, userOf[id])
				}
			}
		}
		if !changed && len(gave) == 0 {
			return nil // everyone it names already knew: nothing changes, nothing is recorded
		}
		changed = true
		payload, err := json.Marshal(trapRevealedEvent{PointID: pointID, CharacterIDs: gave, All: all})
		if err != nil {
			return fmt.Errorf("encode the event payload: %w", err)
		}
		// With no open session nothing is written here: the reveal is still made.
		if _, err := s.live.AppendEvent(ctx, tx, m.CampaignID, eventTrapRevealed, m.UserID, payload, now); err != nil {
			return fmt.Errorf("record the reveal: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "reveal a trap", err)
	}
	if changed {
		seen := playersSee(mapID, mapRow.RevealedAt, current)
		// Everyone who sees the map hears of a trap revealed to all. A trap
		// revealed to some characters reaches only their players (and the master,
		// who made the change), never the others.
		s.publishPointsChanged(ctx, m.CampaignID, mapRow, all && seen, after)
		if !all && seen && len(newUsers) > 0 {
			s.live.PublishToUsers(m.CampaignID, newUsers, mapChangedEvent(mapID))
		}
	}
	out, err := s.masterPoint(ctx, m, after)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&mapsv1.RevealTrapResponse{Point: out}), nil
}

// mapChangedEvent is the stream event that makes a watcher read the map again.
func mapChangedEvent(mapID string) *playv1.WatchGameSessionResponse {
	return &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_MapChanged_{
		MapChanged: &playv1.WatchGameSessionResponse_MapChanged{MapId: mapID},
	}}
}

// MarkTreasureFound implements mapsv1connect.MapServiceHandler.
func (s *Service) MarkTreasureFound(
	ctx context.Context,
	req *connect.Request[mapsv1.MarkTreasureFoundRequest],
) (*connect.Response[mapsv1.MarkTreasureFoundResponse], error) {
	m, mapID, pointID, err := s.pointCall(ctx, req.Msg.GetCampaignId(), req.Msg.GetMapId(), req.Msg.GetPointId())
	if err != nil {
		return nil, err
	}
	ids, _, err := s.playerCharacters(ctx, m.CampaignID, req.Msg.GetCharacterIds(), false)
	if err != nil {
		return nil, err
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}

	var mapRow mapsdb.Map
	var after mapsdb.MapPoint
	var firstFind bool // this call is the one that found it (a retried transaction starts false again)
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		firstFind = false
		q := s.queries.WithTx(tx)
		var before mapsdb.MapPoint
		var err error
		if mapRow, before, err = s.lockPoint(ctx, q, m.CampaignID, mapID, pointID, mapsv1.MapPointKind_MAP_POINT_KIND_TREASURE); err != nil {
			return err
		}
		if before.TreasureConvertedAwardID != nil {
			return errTreasureConverted()
		}
		// The session open now, if any, is the one whose summary counts it.
		session, err := s.live.OpenSessionID(ctx, tx, m.CampaignID)
		if err != nil {
			return fmt.Errorf("read the open session: %w", err)
		}
		now := s.now()
		params := mapsdb.SetTreasureFoundParams{MapID: mapID, ID: pointID, FoundAt: now, Now: now}
		if session != "" {
			params.SessionID = &session
		}
		if after, err = q.SetTreasureFound(ctx, params); err != nil {
			return fmt.Errorf("mark the treasure found: %w", err)
		}
		// Marking a found treasure again changes who found it, and keeps when.
		if err := q.DeleteTreasureFinders(ctx, pointID); err != nil {
			return fmt.Errorf("replace the finders: %w", err)
		}
		for _, id := range ids {
			if err := q.InsertTreasureFinder(ctx, mapsdb.InsertTreasureFinderParams{PointID: pointID, CharacterID: id}); err != nil {
				return fmt.Errorf("record a finder: %w", err)
			}
		}
		if before.TreasureFoundAt != nil {
			return nil // already found: only the finders changed, and no new event
		}
		payload, err := json.Marshal(treasureEvent{PointID: pointID, CharacterIDs: ids, ValuePO: int32PtrValue(after.TreasureValuePo)})
		if err != nil {
			return fmt.Errorf("encode the event payload: %w", err)
		}
		if _, err := s.live.AppendEvent(ctx, tx, m.CampaignID, eventTreasureFound, m.UserID, payload, now); err != nil {
			return fmt.Errorf("record the find: %w", err)
		}
		firstFind = true
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "mark a treasure found", err)
	}
	if firstFind {
		logging.Event(ctx, s.logger, "treasure.found", slog.String("map_id", mapID), slog.String("point_id", pointID), slog.Int("finders", len(ids)))
	}
	// A found treasure is visible to everyone who sees the map.
	s.publishPointsChanged(ctx, m.CampaignID, mapRow, playersSee(mapID, mapRow.RevealedAt, current), after)
	out, err := s.masterPoint(ctx, m, after)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&mapsv1.MarkTreasureFoundResponse{Point: out}), nil
}

// UnmarkTreasureFound implements mapsv1connect.MapServiceHandler.
func (s *Service) UnmarkTreasureFound(
	ctx context.Context,
	req *connect.Request[mapsv1.UnmarkTreasureFoundRequest],
) (*connect.Response[mapsv1.UnmarkTreasureFoundResponse], error) {
	m, mapID, pointID, err := s.pointCall(ctx, req.Msg.GetCampaignId(), req.Msg.GetMapId(), req.Msg.GetPointId())
	if err != nil {
		return nil, err
	}
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}

	var mapRow mapsdb.Map
	var after mapsdb.MapPoint
	var changed bool
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		changed = false
		var before mapsdb.MapPoint
		var err error
		if mapRow, before, err = s.lockPoint(ctx, q, m.CampaignID, mapID, pointID, mapsv1.MapPointKind_MAP_POINT_KIND_TREASURE); err != nil {
			return err
		}
		after = before
		if before.TreasureFoundAt == nil {
			return nil // not found: nothing to unmark
		}
		if before.TreasureConvertedAwardID != nil {
			return errTreasureConverted()
		}
		now := s.now()
		if after, err = q.ClearTreasureFound(ctx, mapsdb.ClearTreasureFoundParams{MapID: mapID, ID: pointID, Now: now}); err != nil {
			return fmt.Errorf("unmark the treasure: %w", err)
		}
		if err := q.DeleteTreasureFinders(ctx, pointID); err != nil {
			return fmt.Errorf("forget the finders: %w", err)
		}
		changed = true
		payload, err := json.Marshal(treasureEvent{PointID: pointID, ValuePO: int32PtrValue(after.TreasureValuePo)})
		if err != nil {
			return fmt.Errorf("encode the event payload: %w", err)
		}
		if _, err := s.live.AppendEvent(ctx, tx, m.CampaignID, eventTreasureUnfound, m.UserID, payload, now); err != nil {
			return fmt.Errorf("record the unmark: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "unmark a treasure", err)
	}
	if changed {
		// A found treasure was visible to everyone; so was the change.
		s.publishPointsChanged(ctx, m.CampaignID, mapRow, playersSee(mapID, mapRow.RevealedAt, current), after)
	}
	out, err := s.masterPoint(ctx, m, after)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&mapsv1.UnmarkTreasureFoundResponse{Point: out}), nil
}

// trapKnowers returns the players whose characters know a trap that not everyone
// sees: a change to it, or its deletion, must reach them and nobody else. It
// reads inside the transaction, before a delete cascades the reveal rows away.
func (s *Service) trapKnowers(ctx context.Context, tx pgx.Tx, q *mapsdb.Queries, campaignID string, p mapsdb.MapPoint) ([]string, error) {
	if p.Kind != kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_TRAP] || everyoneSees(p) {
		return nil, nil
	}
	rows, err := q.ListPointRevealsOfPoint(ctx, p.ID)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.CharacterID)
	}
	chars, err := s.characters.MapCharacters(ctx, tx, campaignID, ids)
	if err != nil {
		return nil, fmt.Errorf("find who knows the trap: %w", err)
	}
	var users []string
	for _, c := range chars {
		if u := c.GetPlayerUserId(); u != "" && !slices.Contains(users, u) {
			users = append(users, u)
		}
	}
	return users, nil
}

// tellTrapKnowers tells those players that the map changed, after the commit,
// when the map is one they see.
func (s *Service) tellTrapKnowers(campaignID, mapID string, mapRow mapsdb.Map, currentMap string, users []string) {
	if len(users) > 0 && playersSee(mapID, mapRow.RevealedAt, currentMap) {
		s.live.PublishToUsers(campaignID, users, mapChangedEvent(mapID))
	}
}
