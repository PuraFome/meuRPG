package play

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// What happens to a readied action besides Ready: it is lost when the readier's next turn starts, it goes with the combat,
// the master's undo of the Ready takes it back, and the views show it to the master and the readier's player.

// loseReadiedAtTurnStart ends the readied actions of the combatants whose turn starts: a readied action lasts until the
// start of the readier's next turn (SRD 5.1, "Ready"). Nothing comes back: the action was spent, the reaction never was.
func loseReadiedAtTurnStart(ctx context.Context, c *combatTx, ids []string, round int32) error {
	for _, id := range ids {
		row, err := c.q.GetWaitingReadiedOf(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read the readied action: %w", err)
		}
		if err := loseReadied(ctx, c, row, round); err != nil {
			return err
		}
	}
	return nil
}

// loseReadied marks the row lost and writes its line.
func loseReadied(ctx context.Context, c *combatTx, row playdb.ReadiedAction, round int32) error {
	if err := c.q.SetReadiedActionStatus(ctx, playdb.SetReadiedActionStatusParams{ID: row.ID, Status: readiedLost}); err != nil {
		return fmt.Errorf("lose the readied action: %w", err)
	}
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return err
	}
	who, _ := combatantByID(cs, row.CombatantID)
	return insertEvent(ctx, c, eventReadiedLost, &c.actorUserID, nil, readiedLostEvent(row, who, round))
}

// dropReadied deletes the readied actions of a combat that ends: none can fire any more.
func dropReadied(ctx context.Context, c *combatTx) error {
	if err := c.q.DeleteReadiedActionsOfEncounter(ctx, c.enc.ID); err != nil {
		return fmt.Errorf("drop the readied actions: %w", err)
	}
	return nil
}

// readiedBack is the undo of a Ready: the economy and the turn as TakeAction's undo puts them back, and the row goes.
func (u *undoer) readiedBack(ctx context.Context, ev actionEvent) error {
	if err := u.actionTaken(ctx, ev); err != nil {
		return err
	}
	if ev.Readied == nil {
		return nil
	}
	if err := u.c.q.DeleteReadiedAction(ctx, ev.Readied.ID); err != nil {
		return fmt.Errorf("take the readied action back: %w", err)
	}
	return nil
}

// readiedView is the readied action the combatant holds, for the master and its player (the caller checks who reads).
func readiedView(rows []playdb.ReadiedAction, c playdb.Combatant, names func(key string) string) *playv1.ReadiedAction {
	for _, row := range rows {
		if row.CombatantID != c.ID || row.Status != readiedWaiting {
			continue
		}
		out := &playv1.ReadiedAction{
			Id: row.ID, TriggerText: row.TriggerText, Kind: readiedKindToProto[row.Kind], MoveFt: row.MoveFt, Round: row.Round,
			Status: readiedStatusToProto[row.Status],
		}
		if row.ActionKey != nil {
			out.ActionKey, out.ActionNamePt = *row.ActionKey, names(*row.ActionKey)
		}
		if row.TargetID != nil {
			out.TargetCombatantId = *row.TargetID
		}
		return out
	}
	return nil
}

// readiedLine is the log handler of the events of a readied action: a line of the kind for everybody who sees the
// readier; fillReadied gives the trigger and the action to the master and the readier's player alone.
func readiedLine(kind playv1.CombatLogKind) logHandler {
	return func(_ *logBuilder, _ playdb.ListEncounterEventsRow, entry *logEntry) bool {
		entry.kind = kind
		return entry.ev.Readied != nil
	}
}

// fillReadied writes the readied action of the line for the master and the readier's player; any other player reads the
// kind and the actor alone (RN-10).
func (e *logEntry) fillReadied(c *logViewing, out *playv1.CombatLogEntry) bool {
	if !c.v.master && !c.v.owns(c.actor) {
		out.Key = ""
		return true
	}
	r := e.ev.Readied
	name := r.Name
	if name == "" && r.ActionKey != "" {
		name = c.names.of(c.ctx, c.actor, r.ActionKey)
	}
	detail := &playv1.CombatLogReadied{
		ReadiedId: r.ID, TriggerText: r.Trigger, Kind: readiedKindToProto[r.Kind], ActionKey: r.ActionKey, ActionNamePt: name,
		Canceled: r.Canceled,
	}
	switch e.kind {
	case playv1.CombatLogKind_COMBAT_LOG_KIND_READIED_FIRED:
		out.ReadiedFired = detail
	case playv1.CombatLogKind_COMBAT_LOG_KIND_READIED_LOST:
		out.ReadiedLost = detail
	default:
		out.Readied = detail
	}
	return true
}
