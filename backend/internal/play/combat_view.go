package play

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Who sees what in a combat (RN-10, RN-20). The master sees everything. A
// player sees:
//
//	a combatant   when it is not hidden
//	its numbers   (initiative roll, bonus, speed, economy) only for their own
//	              character; an NPC's initiative total never
//	hit points    an NPC's as a word (CombatantState), never as numbers; a
//	              player's character only as the word "Caído" at 0
//	the turn      "Vez do mestre" when a hidden combatant is on turn
//
// Everything a player may not see is left out of the response, never
// blanked: a hidden combatant's ID, label and position never leave the
// server. The same rules decide what each audience gets on the live stream.

// combatViewer is who reads a combat: the caller.
type combatViewer struct {
	master bool
	userID string
}

func viewerOf(m authz.Membership) combatViewer {
	return combatViewer{master: m.Role == authz.RoleMaster, userID: m.UserID}
}

// sees says whether the viewer sees the combatant.
func (v combatViewer) sees(c playdb.Combatant) bool { return v.master || !c.Hidden }

// owns says whether the combatant is the viewer's own character.
func (v combatViewer) owns(c playdb.Combatant) bool {
	return !v.master && c.UserID != nil && v.userID != "" && *c.UserID == v.userID
}

// encounterData is a combat as stored: the row and its combatants in turn
// order. The filter for a viewer is view.
type encounterData struct {
	enc playdb.Encounter
	cs  []playdb.Combatant
}

// loadEncounter reads a combat's combatants, in turn order.
func loadEncounter(ctx context.Context, q *playdb.Queries, enc playdb.Encounter) (*encounterData, error) {
	cs, err := q.ListCombatants(ctx, enc.ID)
	if err != nil {
		return nil, fmt.Errorf("list the combatants: %w", err)
	}
	return &encounterData{enc: enc, cs: cs}, nil
}

// current returns the combatant on turn, if there is one.
func (d *encounterData) current() (playdb.Combatant, bool) {
	if d.enc.CurrentCombatantID == nil {
		return playdb.Combatant{}, false
	}
	i := slices.IndexFunc(d.cs, func(c playdb.Combatant) bool { return c.ID == *d.enc.CurrentCombatantID })
	if i < 0 {
		return playdb.Combatant{}, false
	}
	return d.cs[i], true
}

// turnFor says whose turn the viewer sees: the combatant on turn, or, when
// that one is hidden from them, "the master's".
func (d *encounterData) turnFor(v combatViewer) (id string, masterTurn bool) {
	c, ok := d.current()
	switch {
	case !ok:
		return "", false
	case v.sees(c):
		return c.ID, false
	}
	return "", true
}

// view builds the Encounter the viewer sees. vitals are the player
// characters' vitals by character ID; a player only learns from them that a
// character is down, and the master gets the hit points.
func (d *encounterData) view(v combatViewer, vitals map[string]*playv1.CharacterVitals) *playv1.Encounter {
	e := d.enc
	out := &playv1.Encounter{
		Id:          e.ID,
		Name:        e.Name,
		Status:      statusToProto[e.Status],
		Round:       e.Round,
		MapId:       deref(e.MapID),
		GridColumns: e.GridColumns,
		GridRows:    e.GridRows,
		Revision:    e.Revision,
		StartedAt:   timestampOrNil(e.StartedAt),
		EndedAt:     timestampOrNil(e.EndedAt),
	}
	if v.master {
		out.MapPointId = deref(e.MapPointID)
	}
	out.CurrentCombatantId, out.MasterTurn = d.turnFor(v)
	ties := unresolvedTies(d.cs)
	for _, c := range d.cs {
		if v.sees(c) {
			out.Combatants = append(out.Combatants, combatantToProto(c, v, ties[c.ID], vitals[c.CharacterID]))
		}
	}
	return out
}

// combatantToProto builds the Combatant the viewer sees. The caller checked
// that the viewer sees it.
func combatantToProto(c playdb.Combatant, v combatViewer, tieUnresolved bool, vitals *playv1.CharacterVitals) *playv1.Combatant {
	mine := v.owns(c)
	detail := v.master || mine // the numbers of the turn: the master's and the owner's
	out := &playv1.Combatant{
		Id:                 c.ID,
		Label:              c.Label,
		Kind:               kindToProto[c.Kind],
		Mine:               mine,
		Placed:             placed(c),
		State:              stateOf(c),
		Defeated:           c.Defeated,
		DeathSuccesses:     c.DeathSuccesses,
		DeathFailures:      c.DeathFailures,
		Conditions:         c.Conditions,
		ConcentrationSpell: deref(c.ConcentrationSpell),
	}
	if placed(c) {
		out.Col, out.Row = *c.GridCol, *c.GridRow
	}
	if c.Kind == kindPlayer || v.master {
		out.CharacterId = c.CharacterID // an NPC's character is the master's secret
	}
	// A player's roll is public among the players; an NPC's never reaches one
	// (RN-20).
	if c.Initiative != nil && (v.master || c.Kind == kindPlayer) {
		out.Initiative = c.Initiative
	}
	if detail {
		out.InitiativeBonus = ptr(c.InitiativeBonus)
		out.InitiativeFace = c.InitiativeFace
		out.SpeedFt = c.SpeedFt
		out.MovementUsedFt = c.MovementUsedFt
		out.MovementLeftFt = clamp32(movementLeftFt(c), 0, 1200) // twice the largest speed
		out.Dashed, out.ActionUsed, out.BonusActionUsed, out.ReactionUsed = c.Dashed, c.ActionUsed, c.BonusActionUsed, c.ReactionUsed
	}
	if v.master {
		out.Hidden = c.Hidden
		out.TieUnresolved = tieUnresolved
		out.HitPointsCurrent, out.HitPointsMax, out.HitPointsTemporary = c.HpCurrent, c.HpMax, c.HpTemp
		if vitals != nil {
			out.HitPointsCurrent = ptr(vitals.GetHitPointsCurrent())
			out.HitPointsMax = ptr(vitals.GetHitPointsMax())
			out.HitPointsTemporary = ptr(vitals.GetHitPointsTemporary())
		}
	}
	// A player's character at 0 hit points is down ("Caído"): everyone who
	// sees it gets the word, never the numbers (those are the master's, above).
	if c.Kind == kindPlayer && vitals.GetHitPointsMax() > 0 && vitals.GetHitPointsCurrent() == 0 {
		out.State = playv1.CombatantState_COMBATANT_STATE_DOWN
	}
	return out
}

// The database's statuses and kinds, and the API's.
var (
	statusToProto = map[string]playv1.EncounterStatus{
		statusSetup:  playv1.EncounterStatus_ENCOUNTER_STATUS_SETUP,
		statusActive: playv1.EncounterStatus_ENCOUNTER_STATUS_ACTIVE,
		statusEnded:  playv1.EncounterStatus_ENCOUNTER_STATUS_ENDED,
	}
	kindToProto = map[string]playv1.CombatantKind{
		kindPlayer: playv1.CombatantKind_COMBATANT_KIND_PLAYER,
		kindNPC:    playv1.CombatantKind_COMBATANT_KIND_NPC,
	}
)

func ptr[T any](v T) *T { return &v }

func timestampOrNil(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// viewFor builds the combat for a caller. It reads the player characters'
// vitals: the master's copy carries their hit points, and every copy says
// which of them are down.
func (s *Service) viewFor(ctx context.Context, m authz.Membership, d *encounterData) (*playv1.Encounter, error) {
	v := viewerOf(m)
	var byCharacter map[string]*playv1.CharacterVitals
	if slices.ContainsFunc(d.cs, func(c playdb.Combatant) bool { return c.Kind == kindPlayer }) {
		all, err := s.vitals.ListVitals(ctx, m.CampaignID)
		if err != nil {
			return nil, s.dbError(ctx, "list vitals", err)
		}
		byCharacter = make(map[string]*playv1.CharacterVitals, len(all))
		for _, vit := range all {
			byCharacter[vit.GetCharacterId()] = vit
		}
	}
	return d.view(v, byCharacter), nil
}

// GetEncounter implements playv1connect.CombatServiceHandler.
func (s *Service) GetEncounter(
	ctx context.Context,
	req *connect.Request[playv1.GetEncounterRequest],
) (*connect.Response[playv1.GetEncounterResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	session, err := s.openSession(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	enc, err := s.queries.GetLatestEncounter(ctx, session.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return connect.NewResponse(&playv1.GetEncounterResponse{}), nil
	}
	if err != nil {
		return nil, s.dbError(ctx, "find the encounter", err)
	}
	d, err := loadEncounter(ctx, s.queries, enc)
	if err != nil {
		return nil, s.dbError(ctx, "read the encounter", err)
	}
	out, err := s.viewFor(ctx, m, d)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.GetEncounterResponse{Encounter: out}), nil
}

// The live events of a combat (play.proto, WatchGameSessionResponse). They
// go out after the commit. encounter_changed is a hint without content, so
// everyone gets it; turn_changed and combatant_moved carry names of squares
// and turns, so each audience gets only what it may see.

// publishEncounterChanged tells everyone to read the combat again.
func (s *Service) publishEncounterChanged(campaignID string, e playdb.Encounter) {
	s.hub.Publish(campaignID, live.Event{
		Audience: live.Audience{Everyone: true},
		Message: &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_EncounterChanged_{
			EncounterChanged: &playv1.WatchGameSessionResponse_EncounterChanged{EncounterId: e.ID, Revision: e.Revision},
		}},
	})
}

// publishTurnChanged tells who is on turn: the master the real combatant,
// the players what they may see (a hidden one's turn is "the master's").
func (s *Service) publishTurnChanged(campaignID string, d *encounterData) {
	for _, v := range []combatViewer{{master: true}, {}} {
		id, masterTurn := d.turnFor(v)
		s.hub.Publish(campaignID, live.Event{
			Audience: live.Audience{Master: v.master, Players: !v.master},
			Message: &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_TurnChanged_{
				TurnChanged: &playv1.WatchGameSessionResponse_TurnChanged{
					EncounterId: d.enc.ID, Round: d.enc.Round, CurrentCombatantId: id, MasterTurn: masterTurn,
				},
			}},
		})
	}
}

// publishCombatantMoved tells the master always, and the players only when the
// combatant is not hidden from them.
func (s *Service) publishCombatantMoved(campaignID string, e playdb.Encounter, c playdb.Combatant) {
	if !placed(c) {
		return
	}
	s.hub.Publish(campaignID, live.Event{
		Audience: live.Audience{Master: true, Players: !c.Hidden},
		Message: &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_CombatantMoved_{
			CombatantMoved: &playv1.WatchGameSessionResponse_CombatantMoved{
				EncounterId: e.ID, CombatantId: c.ID, Col: *c.GridCol, Row: *c.GridRow,
			},
		}},
	})
}
