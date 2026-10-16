package maps

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
)

// The phases of the master's writes to points and tokens (MapService's
// CreateMapPoint, UpdateMapPoint, DeleteMapPoint, SetMapPointRevealed,
// PlaceMapToken and RemoveMapToken): check the request, change the rows in one
// transaction, tell the watchers after the commit. Each handler keeps the
// order of its checks; who sees a change (RN-10) is decided by the publish
// helpers in fogwrites.go, which these call exactly as the handlers did.

// newPoint is a CreateMapPoint request after its checks.
type newPoint struct {
	mapID, kind, name, description, hooks string
	showDC                                bool
	x, y                                  int32
	target                                *string
	spec                                  pointSpec
}

// pointText is the cleaned text of a point.
type pointText struct {
	name, description, hooks string
}

// cleanPointText cleans a new point's name, description and hooks, in that order.
func cleanPointText(name, description, hooks string) (pointText, error) {
	var out pointText
	var err error
	if out.name, err = cleanName("name", name); err != nil {
		return pointText{}, err
	}
	if out.description, err = cleanDescription(description); err != nil {
		return pointText{}, err
	}
	if out.hooks, err = cleanHooks(hooks); err != nil {
		return pointText{}, err
	}
	return out, nil
}

// checkSceneOnlyData refuses the hooks and the DC switch on a point that is not
// a scene: the hooks first.
func checkSceneOnlyData(kind, hooks string, showDC bool) error {
	if kind == kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_SCENE] {
		return nil
	}
	if hooks != "" {
		return errOnlyScenesHaveHooks()
	}
	if showDC {
		return errOnlyScenesShowDC()
	}
	return nil
}

// checkNewPoint reads and checks a CreateMapPoint request, up to the data of
// its kind, before anything is read from the database.
func (s *Service) checkNewPoint(msg *mapsv1.CreateMapPointRequest) (newPoint, error) {
	mapID, ok := parseID(msg.GetMapId())
	if !ok {
		return newPoint{}, errMapNotFound()
	}
	kind, ok := kindToDB[msg.GetKind()]
	if !ok {
		return newPoint{}, connect.NewError(connect.CodeInvalidArgument, errors.New("kind is required"))
	}
	text, err := cleanPointText(msg.GetName(), msg.GetDescription(), msg.GetHooks())
	if err != nil {
		return newPoint{}, err
	}
	if err := checkSceneOnlyData(kind, text.hooks, msg.GetShowDc()); err != nil {
		return newPoint{}, err
	}
	if err := checkPosition(msg.GetXBp(), msg.GetYBp()); err != nil {
		return newPoint{}, err
	}
	target, err := parseTarget(msg.GetTargetMapId(), kind, mapID)
	if err != nil {
		return newPoint{}, err
	}
	spec, err := s.specFor(kind, msg.GetTrap(), msg.TreasureValuePo, msg.GetLight())
	if err != nil {
		return newPoint{}, err
	}
	return newPoint{
		mapID: mapID, kind: kind, name: text.name, description: text.description, hooks: text.hooks,
		showDC: msg.GetShowDc(), x: msg.GetXBp(), y: msg.GetYBp(), target: target, spec: spec,
	}, nil
}

// createPointTx inserts the point in one transaction, or finds the one a retry
// of the same request created (replayed).
func (s *Service) createPointTx(ctx context.Context, campaignID string, in newPoint, scopedKey, requestHash *string) (created mapsdb.MapPoint, mapRow mapsdb.Map, replayed bool, err error) {
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		if mapRow, err = s.campaignMap(ctx, q, campaignID, in.mapID); err != nil {
			return err
		}
		created, replayed, err = idem.Create(ctx, scopedKey, requestHash, q.GetMapPointByCreateKey,
			func(p mapsdb.MapPoint) *string { return p.CreateHash },
			func() (mapsdb.MapPoint, error) {
				return s.insertNewPoint(ctx, q, campaignID, in, scopedKey, requestHash)
			})
		return err
	})
	return created, mapRow, replayed, err
}

// insertNewPoint counts the map's points against the limit, checks the target
// and inserts the point. It returns pgx.ErrNoRows, as idem.Create wants, when the
// key's unique index already holds a row.
func (s *Service) insertNewPoint(ctx context.Context, q *mapsdb.Queries, campaignID string, in newPoint, scopedKey, requestHash *string) (mapsdb.MapPoint, error) {
	count, err := q.CountMapPoints(ctx, in.mapID)
	if err != nil {
		return mapsdb.MapPoint{}, fmt.Errorf("count points: %w", err)
	}
	if count >= s.maxPoints {
		return mapsdb.MapPoint{}, connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("the map already has %d points", s.maxPoints))
	}
	if err := checkTarget(ctx, q, campaignID, in.target); err != nil {
		return mapsdb.MapPoint{}, err
	}
	p, err := q.InsertMapPoint(ctx, mapsdb.InsertMapPointParams{
		MapID: in.mapID, Kind: in.kind, Name: in.name, Description: in.description, Hooks: in.hooks, ShowDc: in.showDC,
		XBp: in.x, YBp: in.y, TargetMapID: in.target, Now: s.now(),
		Trap: in.spec.trap, TrapState: in.spec.trapSt, TrapTriggeredAt: in.spec.triggeredAt(s.now), TreasureValuePo: in.spec.value,
		LightPreset: in.spec.light.preset, LightBrightFt: in.spec.lightBright(), LightDimFt: in.spec.lightDim(),
		CreateKey: scopedKey, CreateHash: requestHash,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return p, fmt.Errorf("insert point: %w", err)
	}
	return p, err
}

// findPointForUpdate reads and locks a point of the map.
func findPointForUpdate(ctx context.Context, q *mapsdb.Queries, mapID, pointID string) (mapsdb.MapPoint, error) {
	p, err := q.GetMapPointForUpdate(ctx, mapsdb.GetMapPointForUpdateParams{MapID: mapID, ID: pointID})
	if errors.Is(err, pgx.ErrNoRows) {
		return p, errPointNotFound()
	}
	if err != nil {
		return p, fmt.Errorf("find point: %w", err)
	}
	return p, nil
}

// isEmptyPointChange says that an UpdateMapPoint request names nothing to change.
func isEmptyPointChange(msg *mapsv1.UpdateMapPointRequest) bool {
	return msg.Kind == nil && msg.Name == nil && msg.Description == nil && msg.Hooks == nil && msg.ShowDc == nil && msg.XBp == nil && msg.YBp == nil &&
		msg.TargetMapId == nil && msg.Revealed == nil && msg.Trap == nil && msg.TreasureValuePo == nil && msg.Light == nil
}

// pointChangeOf checks an UpdateMapPoint request and reads what it changes.
func pointChangeOf(msg *mapsv1.UpdateMapPointRequest) (pointChange, error) {
	if isEmptyPointChange(msg) {
		return pointChange{}, connect.NewError(connect.CodeInvalidArgument, errors.New("nothing to change"))
	}
	change := pointChange{
		x: msg.XBp, y: msg.YBp, target: msg.TargetMapId, revealed: msg.Revealed, showDC: msg.ShowDc,
		trap: msg.Trap, treasure: msg.TreasureValuePo, light: msg.Light,
	}
	if err := change.readKind(msg); err != nil {
		return pointChange{}, err
	}
	if err := change.readText(msg); err != nil {
		return pointChange{}, err
	}
	if err := checkPosition(msg.GetXBp(), msg.GetYBp()); err != nil {
		return pointChange{}, err // unset positions read as 0, which is valid
	}
	return change, nil
}

// readKind reads the kind a request sets, if it sets one.
func (c *pointChange) readKind(msg *mapsv1.UpdateMapPointRequest) error {
	if msg.Kind == nil {
		return nil
	}
	kind, ok := kindToDB[msg.GetKind()]
	if !ok {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("kind must not be unspecified"))
	}
	c.kind = &kind
	return nil
}

// readText reads the name, the description and the hooks a request sets, in that
// order.
func (c *pointChange) readText(msg *mapsv1.UpdateMapPointRequest) error {
	if msg.Name != nil {
		name, err := cleanName("name", msg.GetName())
		if err != nil {
			return err
		}
		c.name = &name
	}
	if msg.Description != nil {
		description, err := cleanDescription(msg.GetDescription())
		if err != nil {
			return err
		}
		c.description = &description
	}
	if msg.Hooks != nil {
		hooks, err := cleanHooks(msg.GetHooks())
		if err != nil {
			return err
		}
		c.hooks = &hooks
	}
	return nil
}

// pointChanged is a point as it was and is after a change, with the players who
// had learnt of a trap in it.
type pointChanged struct {
	mapRow        mapsdb.Map
	before, after mapsdb.MapPoint
	knowers       []string
}

// changePointTx locks the point, reads who knows of its trap and saves the
// change, in one transaction.
func (s *Service) changePointTx(ctx context.Context, campaignID, mapID, pointID string, change pointChange) (pointChanged, error) {
	var ch pointChanged
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		if ch.mapRow, err = s.campaignMap(ctx, q, campaignID, mapID); err != nil {
			return err
		}
		if ch.before, err = findPointForUpdate(ctx, q, mapID, pointID); err != nil {
			return err
		}
		if ch.knowers, err = s.trapKnowers(ctx, tx, q, campaignID, ch.before); err != nil {
			return err
		}
		ch.after, err = s.applyPointChange(ctx, q, campaignID, ch.before, change)
		return err
	})
	return ch, err
}

// announcePointChange tells the watchers of a changed point: the players who
// see the map and the point (before or after), and those who knew of its trap.
func (s *Service) announcePointChange(ctx context.Context, campaignID, mapID, current string, ch pointChanged) {
	s.publishPointsChanged(ctx, campaignID, ch.mapRow,
		playersSee(mapID, ch.mapRow.RevealedAt, current) && (everyoneSees(ch.before) || everyoneSees(ch.after)), ch.before, ch.after)
	s.tellTrapKnowers(campaignID, mapID, ch.mapRow, current, ch.knowers)
}

// baseUpdateParams is the update that saves the point as it is.
func (s *Service) baseUpdateParams(p mapsdb.MapPoint) mapsdb.UpdateMapPointParams {
	return mapsdb.UpdateMapPointParams{
		MapID: p.MapID, ID: p.ID, Kind: p.Kind, Name: p.Name, Description: p.Description,
		Hooks: p.Hooks, ShowDc: p.ShowDc, XBp: p.XBp, YBp: p.YBp, TargetMapID: p.TargetMapID, RevealedAt: p.RevealedAt, Now: s.now(),
		Trap: p.Trap, TrapState: p.TrapState, TrapTriggeredAt: p.TrapTriggeredAt, TreasureValuePo: p.TreasureValuePo, TreasureFoundAt: p.TreasureFoundAt,
		TreasureSessionID: p.TreasureSessionID, LightPreset: p.LightPreset, LightBrightFt: p.LightBrightFt, LightDimFt: p.LightDimFt,
	}
}

// overlay puts the plain fields a request sets over the update.
func (c *pointChange) overlay(params *mapsdb.UpdateMapPointParams) {
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
	if c.showDC != nil {
		params.ShowDc = *c.showDC
	}
	if c.x != nil {
		params.XBp = *c.x
	}
	if c.y != nil {
		params.YBp = *c.y
	}
}

// resolvePointTarget works out the point's target after the change, and checks
// that it is a map of the campaign.
func resolvePointTarget(ctx context.Context, q *mapsdb.Queries, campaignID string, p mapsdb.MapPoint, c pointChange, params *mapsdb.UpdateMapPointParams) error {
	if c.target != nil {
		target, err := parseTarget(*c.target, params.Kind, p.MapID)
		if err != nil {
			return err
		}
		params.TargetMapID = target
	}
	if !leads(params.Kind) {
		params.TargetMapID = nil // only a submap or a battle leads somewhere
	}
	return checkTarget(ctx, q, campaignID, params.TargetMapID)
}

// checkChangeSceneOnlyData refuses the hooks and the DC switch a request sets
// on a point that ends up not being a scene.
func checkChangeSceneOnlyData(kind string, c pointChange) error {
	var hooks string
	if c.hooks != nil {
		hooks = *c.hooks
	}
	return checkSceneOnlyData(kind, hooks, c.showDC != nil && *c.showDC)
}

// leaveScene cleans up what a scene leaves behind when the point changes kind.
func leaveScene(ctx context.Context, q *mapsdb.Queries, p mapsdb.MapPoint, params *mapsdb.UpdateMapPointParams) error {
	scene := kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_SCENE]
	if p.Kind != scene || params.Kind == p.Kind {
		return nil
	}
	// Only a scene has actions, clues, hooks and the DC switch (MR-015,
	// MR-029). A scene
	// still open in a session then reads as closed: it is not a scene
	// point anymore. What players already received stays in their notes.
	if err := q.DeleteSceneActionsOfPoint(ctx, p.ID); err != nil {
		return fmt.Errorf("delete the scene's actions: %w", err)
	}
	if err := q.DeleteSceneCluesOfPoint(ctx, p.ID); err != nil {
		return fmt.Errorf("delete the scene's clues: %w", err)
	}
	if err := q.DeletePointImages(ctx, p.ID); err != nil {
		return fmt.Errorf("delete the scene's images: %w", err)
	}
	params.Hooks = ""
	params.ShowDc = false
	return nil
}

// discoverScene records a scene that the change reveals for the first time.
func discoverScene(ctx context.Context, q *mapsdb.Queries, campaignID string, p mapsdb.MapPoint, params mapsdb.UpdateMapPointParams) error {
	scene := kindToDB[mapsv1.MapPointKind_MAP_POINT_KIND_SCENE]
	if params.Kind != scene || params.RevealedAt == nil || (p.RevealedAt != nil && p.Kind == scene) {
		return nil
	}
	// Revealing a scene makes it "discovered" (MR-030, question 61): the
	// group may tag notes with it from now on, even if it is hidden again.
	if err := q.UpsertSceneDiscovery(ctx, mapsdb.UpsertSceneDiscoveryParams{CampaignID: campaignID, PointID: p.ID, DiscoveredAt: params.Now}); err != nil {
		return fmt.Errorf("record the scene's discovery: %w", err)
	}
	return nil
}

// pointDeleted is what a deletion leaves to announce.
type pointDeleted struct {
	mapRow  mapsdb.Map
	deleted mapsdb.MapPoint
	knowers []string
}

// checkDeletable refuses to delete a treasure that was found or turned into XP:
// it is part of the session's record, unmarked before it goes, never lost
// silently.
func checkDeletable(p mapsdb.MapPoint) error {
	if p.TreasureConvertedAwardID != nil {
		return errTreasureConverted()
	}
	if p.TreasureFoundAt != nil {
		return errTreasureFound()
	}
	return nil
}

// deletePointTx deletes the point in one transaction.
func (s *Service) deletePointTx(ctx context.Context, campaignID, mapID, pointID string) (pointDeleted, error) {
	var out pointDeleted
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		if out.mapRow, err = s.campaignMap(ctx, q, campaignID, mapID); err != nil {
			return err
		}
		point, err := findPointForUpdate(ctx, q, mapID, pointID)
		if err != nil {
			return err
		}
		if err := checkDeletable(point); err != nil {
			return err
		}
		// Read before the delete cascades the reveal rows away.
		if out.knowers, err = s.trapKnowers(ctx, tx, q, campaignID, point); err != nil {
			return err
		}
		out.deleted, err = q.DeleteMapPoint(ctx, mapsdb.DeleteMapPointParams{MapID: mapID, ID: pointID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errPointNotFound()
		}
		if err != nil {
			return fmt.Errorf("delete point: %w", err)
		}
		return nil
	})
	return out, err
}

// tokenPlacement is a token after PlaceMapToken: the row, the row it replaced
// and whether it is new.
type tokenPlacement struct {
	mapRow     mapsdb.Map
	token, was mapsdb.MapToken
	placed     bool // a new token, not a move
}

// putToken inserts the character's token, or moves the one it has.
func (s *Service) putToken(ctx context.Context, q *mapsdb.Queries, mapID string, x, y int32, character *charactersv1.CharacterSummary, pl *tokenPlacement) error {
	var err error
	key := mapsdb.GetMapTokenForUpdateParams{MapID: mapID, CharacterID: character.GetId()}
	pl.was, err = q.GetMapTokenForUpdate(ctx, key)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// A player's character starts visible, an NPC hidden (question
		// 31 for Samuel): the master reveals an NPC when the group meets
		// it.
		pl.token, err = q.InsertMapToken(ctx, mapsdb.InsertMapTokenParams{
			MapID: mapID, CharacterID: character.GetId(), XBp: x, YBp: y,
			Hidden:    character.GetKind() != charactersv1.CharacterKind_CHARACTER_KIND_PLAYER,
			UpdatedAt: s.now(),
		})
		pl.placed = true
	case err == nil:
		pl.token, err = q.MoveMapToken(ctx, mapsdb.MoveMapTokenParams{
			MapID: mapID, CharacterID: character.GetId(), XBp: x, YBp: y, UpdatedAt: s.now(),
		})
	}
	if err != nil {
		return fmt.Errorf("place token: %w", err)
	}
	return nil
}

// placeTokenTx places or moves the character's token in one transaction.
func (s *Service) placeTokenTx(ctx context.Context, campaignID, mapID string, x, y int32, character *charactersv1.CharacterSummary) (tokenPlacement, error) {
	var pl tokenPlacement
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		pl.placed = false
		var err error
		if pl.mapRow, err = s.campaignMap(ctx, q, campaignID, mapID); err != nil {
			return err
		}
		return s.putToken(ctx, q, mapID, x, y, character, &pl)
	})
	return pl, err
}

// announceTokenPlaced tells the watchers of a placed or moved token, and lets a
// trap react to it.
func (s *Service) announceTokenPlaced(ctx context.Context, m authz.Membership, mapID, current string, pl tokenPlacement, character *charactersv1.CharacterSummary) {
	// A new token needs its name: the app reads the map again. A move is a
	// `token_moved`, except for an NPC on a fog map, which reaches a player only as
	// what they see (publishTokenWritten).
	var before *mapsdb.MapToken
	if !pl.placed {
		before = &pl.was
	}
	s.publishTokenWritten(ctx, m.CampaignID, pl.mapRow, current, before, &pl.token, isPlayerCharacter(character), !pl.placed)
	s.tokenLanded(ctx, m.CampaignID, m.UserID, mapID, pl.token, character) // a trap may fire, or be noticed (MR-035)
}

// removeCharacterToken takes a character's token off the map and tells the
// watchers.
func (s *Service) removeCharacterToken(ctx context.Context, m authz.Membership, mapID, characterID string) error {
	current, err := s.currentMap(ctx, m.CampaignID)
	if err != nil {
		return err
	}
	mapRow, deleted, err := s.deleteTokenTx(ctx, m.CampaignID, mapID, characterID)
	if err != nil {
		return s.dbError(ctx, "remove a token", err)
	}
	// Whose token it was: a dead character's is told as an NPC's, which reaches no
	// player on a fog map but through what they see.
	player := false
	if found, err := s.characters.MapCharacters(ctx, nil, m.CampaignID, []string{characterID}); err == nil && len(found) == 1 {
		player = isPlayerCharacter(found[0])
	}
	s.publishTokenWritten(ctx, m.CampaignID, mapRow, current, &deleted, nil, player, false)
	return nil
}

// deleteTokenTx deletes the character's token in one transaction.
func (s *Service) deleteTokenTx(ctx context.Context, campaignID, mapID, characterID string) (mapsdb.Map, mapsdb.MapToken, error) {
	var mapRow mapsdb.Map
	var deleted mapsdb.MapToken
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		if mapRow, err = s.campaignMap(ctx, q, campaignID, mapID); err != nil {
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
	return mapRow, deleted, err
}
