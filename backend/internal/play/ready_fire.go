package play

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// FireReadied (ready.proto): the master says the trigger of a readied action happened. It opens a reaction window of kind
// READIED for the readier, through the same machinery as every other window (combat_reaction_window.go): the window waits
// in its own group, holds the turn (REACTION_PENDING), closes by itself when the readier cannot react any more or the row
// stops waiting, and is answered with AnswerReaction (ready_answer.go). Nothing is spent here: the reaction is spent only
// when the readier uses it (SRD 5.1, "Ready"); a refusal leaves the row waiting until it is lost.

// FireReadied implements playv1connect.ReadyServiceHandler.
func (s *Service) FireReadied(
	ctx context.Context,
	req *connect.Request[playv1.FireReadiedRequest],
) (*connect.Response[playv1.FireReadiedResponse], error) {
	m, key, encID, err := masterCall(ctx, req.Msg.GetCampaignId(), req.Msg.GetIdempotencyKey(), req.Msg.GetEncounterId())
	if err != nil {
		return nil, err
	}
	id, err := parseCombatID(req.Msg.GetReadiedId(), "readied action")
	if err != nil {
		return nil, err
	}
	var made actionEvent
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventReadiedFired, encounterID: encID}, func(c *combatTx) (any, error) {
		ev, err := s.fireReadiedTx(ctx, c, id)
		made = ev
		return ev, err
	})
	if err != nil {
		return nil, s.dbError(ctx, "fire a readied action", err)
	}
	enc, err := s.readiedAnswer(ctx, m, res, made)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.FireReadiedResponse{Encounter: enc}), nil
}

// fireReadiedTx checks the row and its readier and opens the window. Its event writes no line (Opened): the line is the
// answer's.
func (s *Service) fireReadiedTx(ctx context.Context, c *combatTx, id string) (actionEvent, error) {
	if err := notEnded(c.enc); err != nil {
		return actionEvent{}, err
	}
	if c.enc.Status != statusActive {
		return actionEvent{}, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_ACTIVE, "the combat is not running")
	}
	row, err := c.q.GetWaitingReadiedAction(ctx, playdb.GetWaitingReadiedActionParams{ID: id, EncounterID: c.enc.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return actionEvent{}, errReadiedNotFound()
	}
	if err != nil {
		return actionEvent{}, fmt.Errorf("read the readied action: %w", err)
	}
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return actionEvent{}, err
	}
	who, ok := combatantByID(cs, row.CombatantID)
	if !ok || who.Dismissed { // out of the order: its row is lost with it (loseReadiedOfDefeated)
		return actionEvent{}, errReadiedNotFound()
	}
	if err := s.mayFire(ctx, c, who, row.ID); err != nil {
		return actionEvent{}, err
	}
	ws, err := s.openWindows(ctx, c, newGroup(), nil, []windowSpec{{kind: reaction.Readied, reactor: &who, trigger: readiedTriggerOf(row)}})
	if err != nil {
		return actionEvent{}, err
	}
	if err := touch(ctx, c); err != nil {
		return actionEvent{}, err
	}
	c.characterID = &who.CharacterID
	ev := readiedRowEvent(row, who, c.enc.Round)
	ev.Readied.Opened, ev.Readied.Window = true, ws[0].ID
	return ev, nil
}

// mayFire refuses a readier that cannot use its reaction now: out of the fight, down, surprised, acting on its own turn,
// incapacitated, with its reaction spent, or already asked.
func (s *Service) mayFire(ctx context.Context, c *combatTx, who playdb.Combatant, rowID string) error {
	if who.Defeated {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_COMBATANT_DOWN, "the readier is out of the fight")
	}
	if err := s.mustReactNowOrOnTurn(ctx, c, who); err != nil {
		return err
	}
	if actsNow(c.enc, who) {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_READY_OWN_TURN, "the readier acts now; the trigger cannot come on its own turn")
	}
	if err := s.readiedGate(ctx, c, who); err != nil {
		return err
	}
	if who.EffectNoReaction || slices.ContainsFunc(who.Conditions, func(k string) bool { return slices.Contains(incapacitating, k) }) {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CANNOT_ACT, "the readier cannot react now")
	}
	if who.ReactionUsed {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_REACTION_USED, "the readier's reaction is used")
	}
	open, err := openWindowsOf(ctx, c)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(open, func(w playdb.ReactionWindow) bool {
		return reaction.Kind(w.Kind) == reaction.Readied && windowTriggerOf(w).Readied == rowID
	}) {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_REACTION_PENDING, "the readied action was fired already")
	}
	return nil
}

// readiedTriggerOf is the trigger a READIED window keeps: the row, and for the master's trigger the readier, the target
// named at Ready and the held action. No text: the trigger's words stay in the row.
func readiedTriggerOf(row playdb.ReadiedAction) windowTrigger {
	return windowTrigger{Actor: row.CombatantID, Target: conv.OrZero(row.TargetID), Key: conv.OrZero(row.ActionKey), Readied: row.ID}
}

// readiedGone says the readied action a READIED window was opened for does not wait any more (lost, given up, used).
func readiedGone(ctx context.Context, c *combatTx, t windowTrigger) (bool, error) {
	_, err := c.q.GetWaitingReadiedAction(ctx, playdb.GetWaitingReadiedActionParams{ID: t.Readied, EncounterID: c.enc.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("read the readied action of a window: %w", err)
	}
	return false, nil
}
