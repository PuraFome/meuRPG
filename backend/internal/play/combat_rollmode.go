package play

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// Changing the mode of a roll (PM-06a, decisions 11 and 12). The server suggests
// the mode its sources make (combat_advantage.go). A player rolls the suggestion, or
// disadvantage with no one's leave; a mode better for them than the suggestion
// (advantage the server did not suggest, or normal where it suggested disadvantage)
// needs the master, who decides what circumstances do to a roll (SRD 5.1, "Advantage
// and Disadvantage"): the player asks with RequestRollMode and rolls the master's
// answer. The master sets any mode. Every change carries a reason of 1 to 120
// characters, kept in combat_reasons or roll_mode_requests (free text never goes in
// the event, docs/privacy.md) and written in the log.

// maxReasonLength is the longest reason of a change of mode or of a removed extra.
const maxReasonLength = 120

// cleanReason checks a reason: one line of 1 to 120 characters.
func cleanReason(raw string) (string, error) {
	reason, err := names.Clean(raw, maxReasonLength)
	if err != nil {
		return "", connect.NewError(connect.CodeInvalidArgument, errors.New("the reason must be 1 to 120 characters on one line"))
	}
	return reason, nil
}

// modeAsk is what the caller of a roll asks for besides the suggestion.
type modeAsk struct {
	want      playv1.RollMode
	reason    string
	requestID string
	// dry works the mode out without writing (the roll waits for a Bardic Inspiration die and
	// is settled later); trusted takes the mode a held roll was made with, which was checked
	// when it was held, even if the circumstances changed meanwhile.
	dry, trusted bool
}

// modeChoice is the mode a roll has once the caller's ask is settled.
type modeChoice struct {
	Mode, Suggested combat.RollMode
	// ReasonID is the combat_reasons row of a change nobody had to approve;
	// RequestID the answered request the roll used. Both empty for the suggestion.
	ReasonID, RequestID string
	// ByMaster says the master set the mode (or answered the request).
	ByMaster bool
	// Reason is the text, for the answer to the master and to the roller's player.
	Reason string
}

// changed says the mode is not the suggestion.
func (m modeChoice) changed() bool { return m.Mode != m.Suggested }

// chooseMode settles the mode of a roll inside its transaction. truth is the mode
// the sources make. It writes the reason of a free change and closes the combatant's
// open requests.
func (s *Service) chooseMode(ctx context.Context, c *combatTx, v combatViewer, attacker, target playdb.Combatant, attackKey string, truth combat.RollMode, ask modeAsk) (modeChoice, error) {
	out := modeChoice{Mode: truth, Suggested: truth, ByMaster: v.master}
	if ask.requestID != "" {
		req, err := c.q.GetRollModeRequest(ctx, playdb.GetRollModeRequestParams{EncounterID: c.enc.ID, ID: ask.requestID})
		if errors.Is(err, pgx.ErrNoRows) {
			return out, connect.NewError(connect.CodeNotFound, errors.New("roll mode request not found"))
		}
		if err != nil {
			return out, fmt.Errorf("find the roll mode request: %w", err)
		}
		if req.CombatantID != attacker.ID || req.TargetID != target.ID || req.AttackKey != attackKey {
			return out, connect.NewError(connect.CodeInvalidArgument, errors.New("roll_mode_request_id is for another attack"))
		}
		switch req.Status {
		case "pending":
			return out, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ROLL_MODE_REQUEST_PENDING, "the master has not answered yet")
		case "closed":
			return out, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ROLL_MODE_REQUEST_CLOSED, "the request is closed")
		}
		out.Mode, out.Reason, out.RequestID, out.ByMaster = modeOfKey(deref(req.DecidedMode)), req.Reason, req.ID, true
		// The master decided: the mode may differ from today's suggestion (a condition
		// changed meanwhile), and the player rolls what he answered.
		return out, nil
	}
	want, asked := modeFromProto(ask.want)
	if !asked || want == truth {
		return out, nil
	}
	var reason string
	if !ask.trusted || ask.reason != "" {
		var err error
		if reason, err = cleanReason(ask.reason); err != nil {
			return out, err
		}
	}
	if !ask.trusted && !v.master && want.Better(truth) {
		return out, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ROLL_MODE_NEEDS_APPROVAL, "a better mode needs the master's approval")
	}
	out.Mode, out.Reason = want, reason
	if ask.dry || reason == "" {
		return out, nil
	}
	row, err := c.q.InsertCombatReason(ctx, playdb.InsertCombatReasonParams{EncounterID: c.enc.ID, Kind: "roll_mode", Reason: reason, CreatedAt: c.now})
	if err != nil {
		return out, fmt.Errorf("keep the reason of the mode: %w", err)
	}
	out.ReasonID = row.ID
	return out, nil
}

// attackModeInputs gathers what the mode of an attack needs inside a change.
type attackModeInputs struct {
	cs     []playdb.Combatant
	states map[string][]playdb.CombatantState
	hiding []playdb.CombatHiding
	helps  []playdb.CombatHelp
}

// readModeInputs reads the combat's states in the transaction.
func (s *Service) readModeInputs(ctx context.Context, c *combatTx, cs []playdb.Combatant) (attackModeInputs, error) {
	rows, err := c.q.ListCombatantStates(ctx, c.enc.ID)
	if err != nil {
		return attackModeInputs{}, fmt.Errorf("list the states: %w", err)
	}
	hiding, helps, err := s.contestFacts(ctx, c.q, c.enc, cs)
	if err != nil {
		return attackModeInputs{}, err
	}
	return attackModeInputs{cs: cs, states: statesOf(rows), hiding: hiding, helps: helps}, nil
}

// facts is the combat as the advantage rules read it.
func (in attackModeInputs) facts(enc playdb.Encounter, sight *fogSight) modeFacts {
	return modeFacts{cs: in.cs, states: in.states, theatre: isTheatre(enc), sight: sight, hiding: in.hiding, helps: in.helps}
}

// RequestRollMode implements playv1connect.CombatServiceHandler.
func (s *Service) RequestRollMode(
	ctx context.Context,
	req *connect.Request[playv1.RequestRollModeRequest],
) (*connect.Response[playv1.RequestRollModeResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	if m.Role == authz.RoleMaster {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("the master sets the mode on the roll"))
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	attackerID, err := parseCombatID(req.Msg.GetAttackerId(), "combatant")
	if err != nil {
		return nil, err
	}
	targetID, err := parseCombatID(req.Msg.GetTargetId(), "combatant")
	if err != nil {
		return nil, err
	}
	attackKey := req.Msg.GetAttackKey()
	if attackKey == "" || len(attackKey) > 100 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("attack_key must name one of the attacker's attacks"))
	}
	want, ok := modeFromProto(req.Msg.GetRollMode())
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("roll_mode must be set"))
	}
	reason, err := cleanReason(req.Msg.GetReason())
	if err != nil {
		return nil, err
	}

	var made actionEvent
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventRollModeRequested, encounterID: encID}, func(c *combatTx) (any, error) {
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		v := c.viewer(m, cs)
		attacker, err := findCombatant(cs, attackerID, v)
		if err != nil {
			return nil, err
		}
		if err := v.mayAct(attacker); err != nil {
			return nil, err
		}
		target, err := findCombatant(cs, targetID, v)
		if err != nil {
			return nil, err
		}
		if err := s.mustActNow(ctx, c, attacker); err != nil {
			return nil, err
		}
		sheet, err := s.sheetOf(ctx, c.tx, m.CampaignID, attacker)
		if err != nil {
			return nil, err
		}
		i := slices.IndexFunc(sheet.Attacks, func(a link.Attack) bool { return a.Key == attackKey })
		if i < 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("attack_key is not one of the attacker's attacks"))
		}
		in, err := s.readModeInputs(ctx, c, cs)
		if err != nil {
			return nil, err
		}
		names, err := s.namerFor(ctx, c.tx, m.CampaignID)
		if err != nil {
			return nil, err
		}
		truth := in.facts(c.enc, c.sight).attackMode(attacker, target, sheet.Traits, shapeOfAttack(sheet.Attacks[i]), actsNow(c.enc, attacker), v, names)
		if !want.Better(truth.Mode) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("roll_mode must be better than the suggestion: roll the suggested mode, or disadvantage, without asking"))
		}
		if _, err := c.q.CloseRollModeRequestsOf(ctx, playdb.CloseRollModeRequestsOfParams{CombatantID: attacker.ID, AnsweredAt: &c.now}); err != nil {
			return nil, fmt.Errorf("close the earlier requests: %w", err)
		}
		row, err := c.q.InsertRollModeRequest(ctx, playdb.InsertRollModeRequestParams{
			EncounterID: c.enc.ID, CombatantID: attacker.ID, TargetID: target.ID, AttackKey: attackKey,
			SuggestedMode: modeKey(truth.Mode), RequestedMode: modeKey(want), Reason: reason, CreatedAt: c.now,
		})
		if err != nil {
			return nil, fmt.Errorf("keep the request: %w", err)
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		c.characterID = &attacker.CharacterID
		made = actionEvent{
			Round: c.enc.Round, Secret: secretOf(attacker, target), Actor: attacker.ID, Target: target.ID, Key: attackKey,
			RequestID: row.ID, RollMode: modeKey(want), SuggestedMode: modeKey(truth.Mode),
		}
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "ask for a roll mode", err)
	}
	ev, err := resultEvent(res, made)
	if err != nil {
		return nil, s.dbError(ctx, "read the request", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChanged(ctx, m.CampaignID, d.enc)
	})
	if err != nil {
		return nil, err
	}
	resp := &playv1.RequestRollModeResponse{Encounter: out}
	for _, r := range out.GetRollModeRequests() {
		if r.GetId() == ev.RequestID {
			resp.Request = r
		}
	}
	return connect.NewResponse(resp), nil
}

// AnswerRollModeRequest implements playv1connect.CombatServiceHandler.
func (s *Service) AnswerRollModeRequest(
	ctx context.Context,
	req *connect.Request[playv1.AnswerRollModeRequestRequest],
) (*connect.Response[playv1.AnswerRollModeRequestResponse], error) {
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
	reqID, err := parseCombatID(req.Msg.GetRequestId(), "roll mode request")
	if err != nil {
		return nil, err
	}
	decided, ok := modeFromProto(req.Msg.GetDecidedMode())
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("decided_mode must be set"))
	}

	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventRollModeAnswered, encounterID: encID}, func(c *combatTx) (any, error) {
		row, err := c.q.GetRollModeRequest(ctx, playdb.GetRollModeRequestParams{EncounterID: c.enc.ID, ID: reqID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("roll mode request not found"))
		}
		if err != nil {
			return nil, fmt.Errorf("find the roll mode request: %w", err)
		}
		switch {
		case row.Status == "closed":
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ROLL_MODE_REQUEST_CLOSED, "the request is closed")
		case row.Status == "answered" && modeOfKey(deref(row.DecidedMode)) != decided:
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ROLL_MODE_REQUEST_CLOSED, "the request is answered already")
		case row.Status == "answered":
			return nil, nil // the same answer again: nothing differs
		}
		mode := modeKey(decided)
		if _, err := c.q.AnswerRollModeRequest(ctx, playdb.AnswerRollModeRequestParams{ID: row.ID, DecidedMode: &mode, AnsweredAt: &c.now}); err != nil {
			return nil, fmt.Errorf("answer the roll mode request: %w", err)
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		attacker, _ := findCombatant(cs, row.CombatantID, combatViewer{master: true})
		target, _ := findCombatant(cs, row.TargetID, combatViewer{master: true})
		c.characterID = &attacker.CharacterID
		return actionEvent{
			Round: c.enc.Round, Secret: secretOf(attacker, target), Actor: attacker.ID, Target: target.ID, Key: row.AttackKey,
			RequestID: row.ID, RollMode: mode, SuggestedMode: row.SuggestedMode, Requested: row.RequestedMode,
		}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "answer a roll mode request", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChanged(ctx, m.CampaignID, d.enc)
		s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, true)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.AnswerRollModeRequestResponse{Encounter: out}), nil
}

// CancelRollModeRequest implements playv1connect.CombatServiceHandler.
func (s *Service) CancelRollModeRequest(
	ctx context.Context,
	req *connect.Request[playv1.CancelRollModeRequestRequest],
) (*connect.Response[playv1.CancelRollModeRequestResponse], error) {
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
	reqID, err := parseCombatID(req.Msg.GetRequestId(), "roll mode request")
	if err != nil {
		return nil, err
	}
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventRollModeAnswered, encounterID: encID}, func(c *combatTx) (any, error) {
		row, err := c.q.GetRollModeRequest(ctx, playdb.GetRollModeRequestParams{EncounterID: c.enc.ID, ID: reqID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("roll mode request not found"))
		}
		if err != nil {
			return nil, fmt.Errorf("find the roll mode request: %w", err)
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		v := c.viewer(m, cs)
		attacker, err := findCombatant(cs, row.CombatantID, v)
		if err != nil {
			return nil, err
		}
		if err := v.mayAct(attacker); err != nil {
			return nil, err
		}
		if row.Status == "closed" {
			return nil, nil // taken back already
		}
		if err := c.q.CloseRollModeRequest(ctx, playdb.CloseRollModeRequestParams{ID: row.ID, AnsweredAt: &c.now}); err != nil {
			return nil, fmt.Errorf("close the roll mode request: %w", err)
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		c.characterID = &attacker.CharacterID
		return actionEvent{
			Round: c.enc.Round, Secret: attacker.Hidden, Actor: attacker.ID, Target: row.TargetID, Key: row.AttackKey,
			RequestID: row.ID, Canceled: true, SuggestedMode: row.SuggestedMode, Requested: row.RequestedMode,
		}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "cancel a roll mode request", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChanged(ctx, m.CampaignID, d.enc)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.CancelRollModeRequestResponse{Encounter: out}), nil
}

// closeRequestsOf closes the open roll mode requests of a combatant: its turn
// ended, or its attack used one. It returns how many it closed.
func closeRequestsOf(ctx context.Context, c *combatTx, combatantID string) error {
	if _, err := c.q.CloseRollModeRequestsOf(ctx, playdb.CloseRollModeRequestsOfParams{CombatantID: combatantID, AnsweredAt: &c.now}); err != nil {
		return fmt.Errorf("close the roll mode requests: %w", err)
	}
	return nil
}

// rollModeRequestProto is a request as the caller reads it.
func rollModeRequestProto(r playdb.RollModeRequest, attackName string) *playv1.RollModeRequest {
	status := playv1.RollModeRequestStatus_ROLL_MODE_REQUEST_STATUS_PENDING
	switch r.Status {
	case "answered":
		status = playv1.RollModeRequestStatus_ROLL_MODE_REQUEST_STATUS_ANSWERED
	case "closed":
		status = playv1.RollModeRequestStatus_ROLL_MODE_REQUEST_STATUS_CLOSED
	}
	out := &playv1.RollModeRequest{
		Id: r.ID, CombatantId: r.CombatantID, TargetId: r.TargetID, AttackKey: r.AttackKey, AttackNamePt: attackName,
		SuggestedMode: modeToProto[modeOfKey(r.SuggestedMode)], RequestedMode: modeToProto[modeOfKey(r.RequestedMode)],
		Status: status, Reason: r.Reason, CreatedAt: timestampOrNil(&r.CreatedAt),
	}
	if r.DecidedMode != nil {
		out.DecidedMode = modeToProto[modeOfKey(*r.DecidedMode)]
	}
	return out
}
