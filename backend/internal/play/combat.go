package play

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The combat (MR-013, Etapa 6): CombatService, in the same Service as
// PlayService because a combat lives inside the open game session and its
// events travel on the same stream. This file has the handlers; the helpers
// are in combat_write.go (the transaction every change shares),
// combat_view.go (what each viewer sees) and combat_rules.go (the pure rules).
//
// A combat is an encounters row with combatants rows (migration 00043 and
// 00044). Every handler starts with one explicit check, and every change
// takes the open session's row lock, checks the idempotency key, changes the
// rows, writes a session event, and only then, after the commit, publishes.

// StartEncounter implements playv1connect.CombatServiceHandler.
func (s *Service) StartEncounter(
	ctx context.Context,
	req *connect.Request[playv1.StartEncounterRequest],
) (*connect.Response[playv1.StartEncounterResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	name, err := names.Clean(req.Msg.GetName(), maxEncounterName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name %w", err))
	}
	pointID, err := optionalID(req.Msg.GetMapPointId(), "point not found")
	if err != nil {
		return nil, err
	}
	parts, err := s.participants(ctx, m.CampaignID, req.Msg.GetParticipants(), true)
	if err != nil {
		return nil, err
	}
	var point link.BattlePoint
	if pointID != nil {
		if point, err = s.maps.BattlePoint(ctx, m.CampaignID, *pointID); err != nil {
			return nil, s.dbError(ctx, "find the battle point", err)
		}
	}

	var newMap string // the map the point made current, if it changed
	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventEncounterStarted}, func(c *combatTx) (any, error) {
		newMap = ""
		_, err := c.q.GetOpenEncounter(ctx, c.session.ID)
		switch {
		case err == nil:
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ENCOUNTER_ALREADY_OPEN,
				"the session already has a combat; end it first")
		case !errors.Is(err, pgx.ErrNoRows):
			return nil, fmt.Errorf("find the open encounter: %w", err)
		}

		// The fight is on the session's current map, or on the map the battle
		// point leads to, which then becomes the current one.
		mapID := deref(c.session.CurrentMapID)
		if point.TargetMapID != "" {
			mapID = point.TargetMapID
		}
		if mapID == "" {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_CURRENT_MAP,
				"the session has no current map to fight on")
		}
		grid, err := s.maps.MapGrid(ctx, m.CampaignID, mapID)
		if err != nil {
			return nil, err
		}
		if !grid.OK() {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_MAP_HAS_NO_GRID,
				"the map has no grid", func(b *playv1.EncounterBlocked) { b.MapId = mapID })
		}
		if mapID != deref(c.session.CurrentMapID) {
			// As SetCurrentMap: the players see the current map, so it is revealed.
			if err := s.maps.RevealMap(ctx, c.tx, m.CampaignID, mapID, c.now); err != nil {
				return nil, err
			}
			if _, err := c.q.SetCurrentMap(ctx, playdb.SetCurrentMapParams{ID: c.session.ID, CurrentMapID: &mapID}); err != nil {
				return nil, fmt.Errorf("set the current map: %w", err)
			}
			newMap = mapID
		}

		enc, err := c.q.InsertEncounter(ctx, playdb.InsertEncounterParams{
			GameSessionID: c.session.ID, MapID: &mapID, MapPointID: pointID, Name: name,
			GridColumns: grid.Columns, GridRows: grid.Rows, CreatedAt: c.now,
		})
		if err != nil {
			return nil, fmt.Errorf("insert encounter: %w", err)
		}
		c.enc = enc
		_, added, err := s.addParticipants(ctx, c, grid, nil, parts)
		if err != nil {
			return nil, err
		}
		return map[string]any{"encounter_id": enc.ID, "combatants": len(added), "map_id": mapID}, nil
	})
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" && pgErr.ConstraintName == "encounters_one_open_per_session" {
		// Another start of the same session won the race.
		err = errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ENCOUNTER_ALREADY_OPEN,
			"the session already has a combat; end it first")
	}
	if err != nil {
		return nil, s.dbError(ctx, "start an encounter", err)
	}
	out, err := s.finish(ctx, m, res, func(d *encounterData) {
		if newMap != "" {
			s.Publish(m.CampaignID, true, &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_CurrentMapChanged_{
				CurrentMapChanged: &playv1.WatchGameSessionResponse_CurrentMapChanged{MapId: newMap},
			}})
		}
		s.publishEncounterChanged(m.CampaignID, d.enc)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.StartEncounterResponse{Encounter: out}), nil
}

// planned is a participant of a combat once checked: the character, how many
// copies fight, and whether they start hidden.
type planned struct {
	char   link.Character
	count  int
	hidden bool
}

// participants checks who joins a combat and reads their characters. With
// withParty, a request without any player character brings the whole party:
// a combat of nothing but NPCs makes no sense.
func (s *Service) participants(ctx context.Context, campaignID string, in []*playv1.Participant, withParty bool) ([]planned, error) {
	if len(in) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("participants must list who fights"))
	}
	if len(in) > maxCombatants {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("participants must have at most %d entries", maxCombatants))
	}
	ids := make([]string, 0, len(in))
	for i, p := range in {
		id, ok := parseID(p.GetCharacterId())
		if !ok {
			return nil, errCharacterNotFound()
		}
		if slices.Contains(ids, id) {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("participants[%d] repeats a character", i))
		}
		ids = append(ids, id)
	}
	found, err := s.roster.CombatCharacters(ctx, campaignID, ids)
	if err != nil {
		return nil, s.dbError(ctx, "read the characters of a combat", err)
	}
	byID := make(map[string]link.Character, len(found))
	for _, c := range found {
		byID[c.ID] = c
	}
	out := make([]planned, 0, len(in))
	for i, p := range in {
		c, ok := byID[ids[i]]
		if !ok {
			return nil, errCharacterNotFound()
		}
		count := int(p.GetCount())
		if count == 0 {
			count = 1
		}
		hidden := !c.Player // a new NPC starts hidden (question 31)
		if p.Hidden != nil {
			hidden = p.GetHidden()
		}
		switch {
		case c.Player && (count != 1 || hidden):
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("participants[%d]: a player's character joins once, and is never hidden", i))
		case count < 1 || count > maxNPCCopies:
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("participants[%d].count must be 1 to %d", i, maxNPCCopies))
		}
		out = append(out, planned{char: c, count: count, hidden: hidden})
	}
	if withParty && !slices.ContainsFunc(out, func(p planned) bool { return p.char.Player }) {
		party, err := s.roster.CombatParty(ctx, campaignID)
		if err != nil {
			return nil, s.dbError(ctx, "read the party", err)
		}
		for _, c := range party {
			out = append(out, planned{char: c, count: 1})
		}
	}
	// The party first, then the NPCs, each in the order given.
	slices.SortStableFunc(out, func(a, b planned) int {
		switch {
		case a.char.Player == b.char.Player:
			return 0
		case a.char.Player:
			return -1
		}
		return 1
	})
	total := 0
	for _, p := range out {
		total += p.count
	}
	if total > maxCombatants {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("a combat has at most %d combatants", maxCombatants))
	}
	return out, nil
}

func errCharacterNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("character not found"))
}

func parseID(raw string) (string, bool) {
	id, err := parseCombatID(raw, "")
	return id, err == nil
}

// addParticipants inserts the combatants of the participants into the combat
// c.enc, rolls each NPC's own initiative (RN-19), puts them in the turn
// order, and returns the whole order and the combatants it added. A
// combatant starts on the square of its character's token, when it has one
// and nobody stands there (an NPC with several copies puts only the first
// one there). existing are
// the combatants already in the combat.
func (s *Service) addParticipants(ctx context.Context, c *combatTx, grid link.Grid, existing []playdb.Combatant, parts []planned) (order, added []playdb.Combatant, err error) {
	var tokens []link.TokenPosition
	if c.enc.MapID != nil {
		if tokens, err = s.maps.MapTokens(ctx, *c.enc.MapID); err != nil {
			return nil, nil, fmt.Errorf("read the tokens: %w", err)
		}
	}
	taken := make(map[string]bool, len(existing))
	for _, e := range existing {
		taken[e.Label] = true
	}
	all := slices.Clone(existing)
	for _, p := range parts {
		var labels []string
		if p.char.Player {
			labels = copyLabels(p.char.Name, 1, taken)
		} else {
			labels = copyLabels(p.char.Name, p.count, taken)
		}
		for i, label := range labels {
			row := playdb.InsertCombatantParams{
				EncounterID: c.enc.ID, CharacterID: p.char.ID, Label: label, Kind: kindNPC, Hidden: p.hidden,
				InitiativeBonus: clamp32(p.char.InitiativeBonus, -20, 40), OrderIndex: clamp32(len(all), 0, math.MaxInt32),
				SpeedFt: clamp32(p.char.SpeedFt, 0, 600), CreatedAt: c.now,
			}
			if p.char.Player {
				row.Kind = kindPlayer
				if p.char.PlayerUserID != "" {
					row.UserID = &p.char.PlayerUserID
				}
			} else {
				hp := clamp32(p.char.HitPointsMax, 1, math.MaxInt32)
				row.HpCurrent, row.HpMax, row.HpTemp = &hp, &hp, ptr(int32(0))
				// Each copy rolls for itself, in the app (RN-19).
				face, total, err := s.rollInitiative(int(row.InitiativeBonus))
				if err != nil {
					return nil, nil, err
				}
				row.Initiative, row.InitiativeFace = ptr(clamp32(total, math.MinInt32, math.MaxInt32)), ptr(clamp32(face, 1, 20))
			}
			// Never on a square someone already stands on (reinforcements of
			// an NPC already in the fight): the master places that one.
			if i == 0 {
				if t, ok := tokenOf(tokens, p.char.ID); ok {
					col, r := squareOf(grid, t.XBP, t.YBP)
					if !slices.ContainsFunc(all, func(o playdb.Combatant) bool {
						return placed(o) && *o.GridCol == col && *o.GridRow == r
					}) {
						row.GridCol, row.GridRow = &col, &r
					}
				}
			}
			inserted, err := c.q.InsertCombatant(ctx, row)
			if err != nil {
				return nil, nil, fmt.Errorf("insert combatant: %w", err)
			}
			all = append(all, inserted)
			added = append(added, inserted)
		}
	}
	order, err = saveOrder(ctx, c.q, all, orderCombatants(all))
	if err != nil {
		return nil, nil, err
	}
	// The added combatants as they are now in the order.
	for i, a := range added {
		added[i] = order[slices.IndexFunc(order, func(o playdb.Combatant) bool { return o.ID == a.ID })]
	}
	return order, added, nil
}

func tokenOf(tokens []link.TokenPosition, characterID string) (link.TokenPosition, bool) {
	i := slices.IndexFunc(tokens, func(t link.TokenPosition) bool { return t.CharacterID == characterID })
	if i < 0 {
		return link.TokenPosition{}, false
	}
	return tokens[i], true
}

// rollInitiative rolls a d20 in the app and adds the bonus (RN-19): the face
// and the total.
func (s *Service) rollInitiative(bonus int) (face, total int, err error) {
	res, err := dice.Roll(s.roller, dice.Expr{Count: 1, Sides: 20, Modifier: bonus})
	if err != nil {
		return 0, 0, fmt.Errorf("roll the initiative: %w", err)
	}
	return res.Faces[0], res.Total, nil
}

// clamp32 converts n to an int32 inside [lo, hi].
func clamp32(n, lo, hi int) int32 {
	return int32(min(max(n, lo), hi, math.MaxInt32)) //nolint:gosec // clamped to the int32 range just above
}

// SubmitInitiative implements playv1connect.CombatServiceHandler.
func (s *Service) SubmitInitiative(
	ctx context.Context,
	req *connect.Request[playv1.SubmitInitiativeRequest],
) (*connect.Response[playv1.SubmitInitiativeResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	combID, err := parseCombatID(req.Msg.GetCombatantId(), "combatant")
	if err != nil {
		return nil, err
	}
	var inApp bool
	var face int
	switch roll := req.Msg.GetRoll().(type) {
	case *playv1.SubmitInitiativeRequest_RollInApp:
		if !roll.RollInApp {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("roll_in_app must be true"))
		}
		inApp = true
	case *playv1.SubmitInitiativeRequest_D20Face:
		face = int(roll.D20Face)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("set roll_in_app or d20_face"))
	}
	v := viewerOf(m)
	// RN-18: a player rolls the way the campaign and their preference say.
	// The master rolls either way, for anyone.
	var physical bool
	if !v.master {
		if physical, err = s.dice.RollsPhysical(ctx, m.CampaignID, m.UserID); err != nil {
			return nil, s.dbError(ctx, "read the dice setting", err)
		}
	}

	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventInitiativeSubmitted, encounterID: encID}, func(c *combatTx) (any, error) {
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		target, err := findCombatant(cs, combID, v)
		if err != nil {
			return nil, err
		}
		if err := v.mayAct(target); err != nil {
			return nil, err
		}
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		if c.enc.Status != statusSetup {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_IN_SETUP, "initiative is rolled before the combat begins")
		}
		if !v.master {
			if target.Initiative != nil {
				return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_INITIATIVE_ALREADY_SET, "your initiative is already set")
			}
			if physical == inApp {
				return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_WRONG_DICE_MODE, "this is not how the campaign has you roll your dice")
			}
		}
		expr := dice.Expr{Count: 1, Sides: 20, Modifier: int(target.InitiativeBonus)}
		var roll dice.Result
		if inApp {
			roll, err = dice.Roll(s.roller, expr)
			if err != nil {
				return nil, fmt.Errorf("roll the initiative: %w", err)
			}
			face = roll.Faces[0]
		} else if roll, err = dice.Physical(expr, face); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("d20_face must be 1 to 20"))
		}
		total, f := clamp32(roll.Total, math.MinInt32, math.MaxInt32), clamp32(face, 1, 20)
		if err := c.q.SetCombatantInitiative(ctx, playdb.SetCombatantInitiativeParams{ID: target.ID, Initiative: &total, InitiativeFace: &f}); err != nil {
			return nil, fmt.Errorf("save the initiative: %w", err)
		}
		for i := range cs {
			if cs[i].ID == target.ID {
				cs[i].Initiative, cs[i].InitiativeFace, cs[i].TieOrdered = &total, &f, false
			}
		}
		if _, err := saveOrder(ctx, c.q, cs, orderCombatants(cs)); err != nil {
			return nil, err
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		c.characterID = &target.CharacterID
		return map[string]any{"combatant_id": target.ID, "face": f, "total": total, "physical": !inApp}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "submit an initiative", err)
	}
	out, err := s.finish(ctx, m, res, s.changed(m.CampaignID))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.SubmitInitiativeResponse{Encounter: out}), nil
}

// SetInitiativeOrder implements playv1connect.CombatServiceHandler.
func (s *Service) SetInitiativeOrder(
	ctx context.Context,
	req *connect.Request[playv1.SetInitiativeOrderRequest],
) (*connect.Response[playv1.SetInitiativeOrderResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(req.Msg.GetCombatantIds()))
	for _, raw := range req.Msg.GetCombatantIds() {
		id, err := parseCombatID(raw, "combatant")
		if err != nil {
			return nil, err
		}
		if slices.Contains(ids, id) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("combatant_ids repeats a combatant"))
		}
		ids = append(ids, id)
	}
	if len(ids) < 2 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("combatant_ids must name at least two combatants"))
	}

	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventInitiativeOrderSet, encounterID: encID}, func(c *combatTx) (any, error) {
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		var first playdb.Combatant
		for i, id := range ids {
			found, err := findCombatant(cs, id, viewerOf(m))
			if err != nil {
				return nil, err
			}
			if i == 0 {
				first = found
			}
			if !sameTie(first, found) {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("combatant_ids must be tied: the same initiative total and bonus"))
			}
		}
		if _, err := saveOrder(ctx, c.q, cs, withOrder(orderCombatants(cs), ids)); err != nil {
			return nil, err
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		return map[string]any{"combatant_ids": ids}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "set the initiative order", err)
	}
	out, err := s.finish(ctx, m, res, s.changed(m.CampaignID))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.SetInitiativeOrderResponse{Encounter: out}), nil
}

// BeginCombat implements playv1connect.CombatServiceHandler.
func (s *Service) BeginCombat(
	ctx context.Context,
	req *connect.Request[playv1.BeginCombatRequest],
) (*connect.Response[playv1.BeginCombatResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}

	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventCombatBegun, encounterID: encID}, func(c *combatTx) (any, error) {
		if c.enc.Status != statusSetup {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_IN_SETUP, "the combat already began or ended")
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		if len(cs) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("the combat has no combatants"))
		}
		var missing []string
		var labels []string
		for _, c := range cs {
			if c.Initiative == nil {
				missing, labels = append(missing, c.ID), append(labels, c.Label)
			}
		}
		if len(missing) > 0 {
			// Only the master calls this, so the message may name who is missing.
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_INITIATIVE_MISSING,
				fmt.Sprintf("missing the initiative of: %v", labels), func(b *playv1.EncounterBlocked) { b.CombatantIds = missing })
		}
		next, _, ok := nextTurn(cs, "", "")
		if !ok {
			next = cs[0].ID // everybody defeated: the first of the order starts
		}
		if err := c.q.ResetCombatantTurn(ctx, next); err != nil {
			return nil, fmt.Errorf("reset the turn: %w", err)
		}
		started := c.now
		if c.enc, err = c.q.SetEncounterState(ctx, playdb.SetEncounterStateParams{
			ID: c.enc.ID, Status: statusActive, Round: 1, CurrentCombatantID: &next, StartedAt: &started,
		}); err != nil {
			return nil, fmt.Errorf("begin the combat: %w", err)
		}
		return map[string]any{"round": 1, "current_combatant_id": next}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "begin a combat", err)
	}
	out, err := s.finish(ctx, m, res, func(d *encounterData) {
		s.publishEncounterChanged(m.CampaignID, d.enc)
		s.publishTurnChanged(m.CampaignID, d)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.BeginCombatResponse{Encounter: out}), nil
}

// EndTurn implements playv1connect.CombatServiceHandler.
func (s *Service) EndTurn(
	ctx context.Context,
	req *connect.Request[playv1.EndTurnRequest],
) (*connect.Response[playv1.EndTurnResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	// Empty only when nobody is on turn (see the orphaned turn below).
	expected := ""
	if raw := req.Msg.GetExpectedCombatantId(); raw != "" {
		if expected, err = parseCombatID(raw, "combatant"); err != nil {
			return nil, err
		}
	}
	v := viewerOf(m)
	stale := connect.NewError(connect.CodeAborted, errors.New("another combatant is on turn now"))

	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventTurnEnded, encounterID: encID}, func(c *combatTx) (any, error) {
		if c.enc.Status != statusActive {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_ACTIVE, "the combat is not running")
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		// An orphaned turn: the combatant on turn left the fight (the last one
		// who could act was removed, or its character was deleted), so nobody
		// is on turn. The master starts the turns again from the top of the
		// order, in the same round; without this the combat could never move.
		onTurn := c.enc.CurrentCombatantID != nil &&
			slices.ContainsFunc(cs, func(o playdb.Combatant) bool { return o.ID == *c.enc.CurrentCombatantID })
		if !onTurn {
			if !v.master {
				return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only the master can start the turns again"))
			}
			if expected != "" && (c.enc.CurrentCombatantID == nil || *c.enc.CurrentCombatantID != expected) {
				return nil, stale
			}
			next, _, ok := nextTurn(cs, "", "")
			if !ok {
				return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_ACTIVE, "no combatant can take a turn")
			}
			if err := c.q.ResetCombatantTurn(ctx, next); err != nil {
				return nil, fmt.Errorf("reset the turn: %w", err)
			}
			if c.enc, err = c.q.SetEncounterState(ctx, playdb.SetEncounterStateParams{
				ID: c.enc.ID, Status: statusActive, Round: c.enc.Round, CurrentCombatantID: &next, StartedAt: c.enc.StartedAt,
			}); err != nil {
				return nil, fmt.Errorf("pass the turn: %w", err)
			}
			return map[string]any{"to": next, "round": c.enc.Round}, nil
		}
		// A double tap, or a stale screen: someone else is on turn now, and
		// nothing changes, so one tap never skips two turns.
		if *c.enc.CurrentCombatantID != expected {
			return nil, stale
		}
		current, err := findCombatant(cs, expected, v)
		if err != nil {
			return nil, err
		}
		if err := v.mayAct(current); err != nil {
			return nil, err
		}
		next, newRound, ok := nextTurn(cs, current.ID, "")
		if !ok {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_ACTIVE, "no combatant can take a turn")
		}
		round := c.enc.Round
		if newRound {
			round++
		}
		// The next one's turn starts: movement, action, bonus action, dash and
		// reaction come back.
		if err := c.q.ResetCombatantTurn(ctx, next); err != nil {
			return nil, fmt.Errorf("reset the turn: %w", err)
		}
		if c.enc, err = c.q.SetEncounterState(ctx, playdb.SetEncounterStateParams{
			ID: c.enc.ID, Status: statusActive, Round: round, CurrentCombatantID: &next, StartedAt: c.enc.StartedAt,
		}); err != nil {
			return nil, fmt.Errorf("pass the turn: %w", err)
		}
		c.characterID = &current.CharacterID
		return map[string]any{"from": current.ID, "to": next, "round": round}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "end a turn", err)
	}
	out, err := s.finish(ctx, m, res, func(d *encounterData) {
		s.publishTurnChanged(m.CampaignID, d)
		s.publishEncounterChanged(m.CampaignID, d.enc)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.EndTurnResponse{Encounter: out}), nil
}

// MoveCombatant implements playv1connect.CombatServiceHandler.
func (s *Service) MoveCombatant(
	ctx context.Context,
	req *connect.Request[playv1.MoveCombatantRequest],
) (*connect.Response[playv1.MoveCombatantResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	combID, err := parseCombatID(req.Msg.GetCombatantId(), "combatant")
	if err != nil {
		return nil, err
	}
	col, row := req.Msg.GetCol(), req.Msg.GetRow()
	v := viewerOf(m)

	var moved playdb.Combatant
	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventCombatantMoved, encounterID: encID}, func(c *combatTx) (any, error) {
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		target, err := findCombatant(cs, combID, v)
		if err != nil {
			return nil, err
		}
		if err := v.mayAct(target); err != nil {
			return nil, err
		}
		if !inGrid(c.enc, col, row) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("col and row must be a square of the grid"))
		}
		used, cost := target.MovementUsedFt, 0
		if !v.master {
			// RN-21: a player walks only on their own turn, as far as the
			// movement left, and not onto a square somebody they see stands on.
			switch {
			case c.enc.Status != statusActive:
				return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_ACTIVE, "the combat is not running")
			case c.enc.CurrentCombatantID == nil || *c.enc.CurrentCombatantID != target.ID:
				return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_YOUR_TURN, "it is not your turn")
			case !placed(target):
				return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_PLACED, "you are not on the map yet; ask the master")
			}
			cost = moveCostFt(target, col, row)
			if left := movementLeftFt(target); cost > left {
				return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TOO_FAR, "that square is beyond your movement",
					func(b *playv1.EncounterBlocked) { b.MissingFt = clamp32(cost-left, 0, math.MaxInt32) })
			}
			for _, other := range cs {
				if other.ID != target.ID && !other.Defeated && v.sees(other) && placed(other) && *other.GridCol == col && *other.GridRow == row {
					return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_SQUARE_OCCUPIED, "another combatant is on that square")
				}
			}
			used += clamp32(cost, 0, math.MaxInt32)
		}
		if err := c.q.SetCombatantSquare(ctx, playdb.SetCombatantSquareParams{ID: target.ID, GridCol: &col, GridRow: &row, MovementUsedFt: used}); err != nil {
			return nil, fmt.Errorf("move the combatant: %w", err)
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		moved = target
		moved.GridCol, moved.GridRow, moved.MovementUsedFt = &col, &row, used
		c.characterID = &target.CharacterID
		return map[string]any{"combatant_id": target.ID, "col": col, "row": row, "cost_ft": cost}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "move a combatant", err)
	}
	out, err := s.finish(ctx, m, res, func(d *encounterData) { s.publishCombatantMoved(m.CampaignID, d.enc, moved) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.MoveCombatantResponse{Encounter: out}), nil
}

// markDashed records the Dash action: the combatant's speed counts twice for
// the rest of the turn (RN-21). The actions of the next slice call it, inside
// their own transaction, when the action is spent; it is here because it is
// movement's rule.
func markDashed(ctx context.Context, q *playdb.Queries, combatantID string) error { //nolint:unused // called by the actions of the next slice
	if err := q.MarkCombatantDashed(ctx, combatantID); err != nil {
		return fmt.Errorf("mark the dash: %w", err)
	}
	return nil
}

// SetCombatantHidden implements playv1connect.CombatServiceHandler.
func (s *Service) SetCombatantHidden(
	ctx context.Context,
	req *connect.Request[playv1.SetCombatantHiddenRequest],
) (*connect.Response[playv1.SetCombatantHiddenResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	combID, err := parseCombatID(req.Msg.GetCombatantId(), "combatant")
	if err != nil {
		return nil, err
	}
	hidden := req.Msg.GetHidden()

	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventCombatantHiddenSet, encounterID: encID}, func(c *combatTx) (any, error) {
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		target, err := findCombatant(cs, combID, viewerOf(m))
		if err != nil {
			return nil, err
		}
		if target.Kind != kindNPC {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a player's combatant is never hidden"))
		}
		if err := c.q.SetCombatantHidden(ctx, playdb.SetCombatantHiddenParams{ID: target.ID, Hidden: hidden}); err != nil {
			return nil, fmt.Errorf("hide the combatant: %w", err)
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		c.characterID = &target.CharacterID
		return map[string]any{"combatant_id": target.ID, "hidden": hidden}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "hide or show a combatant", err)
	}
	out, err := s.finish(ctx, m, res, func(d *encounterData) {
		s.publishEncounterChanged(m.CampaignID, d.enc)
		// If it is on turn, the players' copy of the turn changes.
		if d.enc.CurrentCombatantID != nil && *d.enc.CurrentCombatantID == combID {
			s.publishTurnChanged(m.CampaignID, d)
		}
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.SetCombatantHiddenResponse{Encounter: out}), nil
}

// AddCombatants implements playv1connect.CombatServiceHandler.
func (s *Service) AddCombatants(
	ctx context.Context,
	req *connect.Request[playv1.AddCombatantsRequest],
) (*connect.Response[playv1.AddCombatantsResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	parts, err := s.participants(ctx, m.CampaignID, req.Msg.GetParticipants(), false)
	if err != nil {
		return nil, err
	}

	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventCombatantsAdded, encounterID: encID}, func(c *combatTx) (any, error) {
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		count := len(cs)
		for _, p := range parts {
			count += p.count
			if !p.char.Player {
				continue
			}
			// A player's character cannot be rolled for later in the order.
			if c.enc.Status != statusSetup || slices.ContainsFunc(cs, func(o playdb.Combatant) bool { return o.CharacterID == p.char.ID }) {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a player's character joins only before the combat begins, and once"))
			}
		}
		if count > maxCombatants {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("a combat has at most %d combatants", maxCombatants))
		}
		_, added, err := s.addParticipants(ctx, c, link.Grid{Columns: c.enc.GridColumns, Rows: c.enc.GridRows}, cs, parts)
		if err != nil {
			return nil, err
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		return map[string]any{"added": len(added)}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "add combatants", err)
	}
	out, err := s.finish(ctx, m, res, s.changed(m.CampaignID))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.AddCombatantsResponse{Encounter: out}), nil
}

// RemoveCombatant implements playv1connect.CombatServiceHandler.
func (s *Service) RemoveCombatant(
	ctx context.Context,
	req *connect.Request[playv1.RemoveCombatantRequest],
) (*connect.Response[playv1.RemoveCombatantResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	combID, err := parseCombatID(req.Msg.GetCombatantId(), "combatant")
	if err != nil {
		return nil, err
	}

	var turnPassed bool
	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventCombatantRemoved, encounterID: encID}, func(c *combatTx) (any, error) {
		turnPassed = false
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		target, err := findCombatant(cs, combID, viewerOf(m))
		if err != nil {
			return nil, err
		}
		if target.Kind == kindPlayer && c.enc.Status == statusActive {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_PLAYER_IN_COMBAT, "a player's combatant cannot leave a combat that is running")
		}
		// The turn passes first when the combatant on turn leaves.
		onTurn := c.enc.Status == statusActive && c.enc.CurrentCombatantID != nil && *c.enc.CurrentCombatantID == target.ID
		if onTurn {
			next, newRound, ok := nextTurn(cs, target.ID, target.ID)
			round, current := c.enc.Round, (*string)(nil)
			if ok {
				current = &next
				if newRound {
					round++
				}
				if err := c.q.ResetCombatantTurn(ctx, next); err != nil {
					return nil, fmt.Errorf("reset the turn: %w", err)
				}
			}
			if c.enc, err = c.q.SetEncounterState(ctx, playdb.SetEncounterStateParams{
				ID: c.enc.ID, Status: c.enc.Status, Round: round, CurrentCombatantID: current, StartedAt: c.enc.StartedAt,
			}); err != nil {
				return nil, fmt.Errorf("pass the turn: %w", err)
			}
			turnPassed = true
		} else if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		if err := c.q.DeleteCombatant(ctx, target.ID); err != nil {
			return nil, fmt.Errorf("delete the combatant: %w", err)
		}
		rest := slices.DeleteFunc(slices.Clone(cs), func(o playdb.Combatant) bool { return o.ID == target.ID })
		if _, err := saveOrder(ctx, c.q, cs, rest); err != nil {
			return nil, err
		}
		c.characterID = &target.CharacterID
		return map[string]any{"combatant_id": target.ID}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "remove a combatant", err)
	}
	out, err := s.finish(ctx, m, res, func(d *encounterData) {
		s.publishEncounterChanged(m.CampaignID, d.enc)
		if turnPassed {
			s.publishTurnChanged(m.CampaignID, d)
		}
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.RemoveCombatantResponse{Encounter: out}), nil
}

// EndEncounter implements playv1connect.CombatServiceHandler.
func (s *Service) EndEncounter(
	ctx context.Context,
	req *connect.Request[playv1.EndEncounterRequest],
) (*connect.Response[playv1.EndEncounterResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}

	var ended bool
	res, err := s.write(ctx, combatWrite{m: m, key: key, kind: eventEncounterEnded, encounterID: encID}, func(c *combatTx) (any, error) {
		ended = false
		if c.enc.Status == statusEnded {
			return nil, nil // ended before: nothing changes, and no event
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		if err := s.endEncounter(ctx, c, cs); err != nil {
			return nil, err
		}
		ended = true
		return map[string]any{"round": c.enc.Round}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "end an encounter", err)
	}
	out, err := s.finish(ctx, m, res, func(d *encounterData) {
		if !ended {
			return
		}
		s.publishEncounterChanged(m.CampaignID, d.enc)
		if d.enc.MapID != nil {
			// The player characters' tokens moved with the combat's end.
			s.Publish(m.CampaignID, true, &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_MapChanged_{
				MapChanged: &playv1.WatchGameSessionResponse_MapChanged{MapId: *d.enc.MapID},
			}})
		}
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.EndEncounterResponse{Encounter: out}), nil
}

// endEncounter ends the combat c.enc inside the transaction: its status, and
// the player characters' tokens, which move to the squares where they ended
// (the map shows them where they stand). cs are its combatants.
func (s *Service) endEncounter(ctx context.Context, c *combatTx, cs []playdb.Combatant) error {
	ended := c.now
	enc, err := c.q.SetEncounterState(ctx, playdb.SetEncounterStateParams{
		ID: c.enc.ID, Status: statusEnded, Round: c.enc.Round, StartedAt: c.enc.StartedAt, EndedAt: &ended,
	})
	if err != nil {
		return fmt.Errorf("end the encounter: %w", err)
	}
	c.enc = enc
	if enc.MapID == nil {
		return nil // the map was deleted: nothing to write back to
	}
	grid := link.Grid{Columns: enc.GridColumns, Rows: enc.GridRows}
	var positions []link.TokenPosition
	for _, cb := range cs {
		if cb.Kind == kindPlayer && placed(cb) {
			x, y := centerOf(grid, *cb.GridCol, *cb.GridRow)
			positions = append(positions, link.TokenPosition{CharacterID: cb.CharacterID, XBP: x, YBP: y})
		}
	}
	if err := s.maps.SetTokenPositions(ctx, c.tx, *enc.MapID, positions, c.now); err != nil {
		return fmt.Errorf("move the tokens: %w", err)
	}
	return nil
}

// endOpenEncounter ends the session's combat, if it has one, when the master
// ends the session (EndGameSession), inside its transaction, with a session
// event.
func (s *Service) endOpenEncounter(ctx context.Context, tx pgx.Tx, q *playdb.Queries, session playdb.GameSession, actorUserID string) error {
	enc, err := q.GetOpenEncounter(ctx, session.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find the open encounter: %w", err)
	}
	cs, err := q.ListCombatants(ctx, enc.ID)
	if err != nil {
		return fmt.Errorf("list the combatants: %w", err)
	}
	c := &combatTx{tx: tx, q: q, session: session, enc: enc, now: s.now()}
	if err := s.endEncounter(ctx, c, cs); err != nil {
		return err
	}
	return insertEvent(ctx, c, eventEncounterEnded, &actorUserID, nil, map[string]any{"round": c.enc.Round, "reason": "session_ended"})
}
