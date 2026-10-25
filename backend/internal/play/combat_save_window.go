package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Answering a reaction window: the window the caller may answer, whichever its kind, and for a save
// window who may choose what and how the roll is handed to the master.

// handSaveToMaster marks a concentration save as left to the master: the owner can no longer
// roll it, and the master's card asks for it.
func (s *Service) handSaveToMaster(ctx context.Context, c *combatTx, w playdb.ReactionWindow, t windowTrigger) error {
	t.Handed = true
	body, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("encode the window's trigger: %w", err)
	}
	if _, err := c.q.SetReactionWindowStep(ctx, playdb.SetReactionWindowStepParams{ID: w.ID, Step: w.Step, Trigger: body}); err != nil {
		return fmt.Errorf("hand the save to the master: %w", err)
	}
	return touch(ctx, c)
}

// refuseSaveAnswer checks who may answer a save window, for the concentration save and the
// effect's: the master's own options (keep, skip) are refused to a player, "leave it to the
// master" to the master, and a save already handed over to anybody but the master.
func refuseSaveAnswer(t windowTrigger, master, masterOnly, handOver bool) error {
	switch {
	case masterOnly && !master:
		return errWindowDenied()
	case handOver && master:
		return connect.NewError(connect.CodeInvalidArgument, errors.New("the master rolls it: use roll_in_app or d20_face"))
	case t.Handed && !master:
		return errWindowDenied() // the roll was left to the master
	}
	return nil
}

// windowToAnswer finds the window an answer is for, inside the change's transaction,
// and checks the caller may answer it and that it is its turn: the master answers any
// window; a player only the ones of their own character, and any other window id, real
// or not, is refused the same way (RN-10). A window that is gone or already answered
// is "not your turn to answer".
func (s *Service) windowToAnswer(ctx context.Context, c *combatTx, m authz.Membership, id string) (playdb.ReactionWindow, playdb.Combatant, error) {
	var none playdb.ReactionWindow
	master := m.Role == authz.RoleMaster
	w, err := c.q.GetReactionWindowForUpdate(ctx, playdb.GetReactionWindowForUpdateParams{EncounterID: c.enc.ID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		if master {
			return none, playdb.Combatant{}, connect.NewError(connect.CodeNotFound, errors.New("reaction window not found"))
		}
		return none, playdb.Combatant{}, errWindowDenied()
	}
	if err != nil {
		return none, playdb.Combatant{}, fmt.Errorf("find the reaction window: %w", err)
	}
	var reactor playdb.Combatant
	if w.ReactorID != nil {
		cs, err := listCombatants(ctx, c)
		if err != nil {
			return none, reactor, err
		}
		if r, ok := combatantByID(cs, *w.ReactorID); ok {
			reactor = r
		}
	}
	if !master && (w.ReactorID == nil || reactor.ID == "" || reactor.UserID == nil || *reactor.UserID != m.UserID || isCreature(reactor)) {
		return none, reactor, errWindowDenied()
	}
	if w.Status == windowAnswered { // the other of the player and the master answered first
		return none, reactor, errReactionAlreadyAnswered()
	}
	if w.Status != windowOpen {
		return none, reactor, errNotYourTurnToAnswer()
	}
	if err := s.mustBeFirstOfGroup(ctx, c, w); err != nil {
		return none, reactor, err
	}
	return w, reactor, nil
}
