package play

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// A contest waits in the reaction window of kind CONTEST (PM-04, decision 3 of W7-X): the
// same mechanism as every other wait, with no timeout. The window is derived from the
// contest row, so no write has to open or close it: settleReactions calls syncContests
// after every change, and the window closes by itself (trigger gone) once the contest no
// longer waits. The reactor is whoever owes the next step: the defender's player for the
// defender's roll, the shover's player for the shove's choice; the master holds it for an
// NPC, and for a roll a player left to him. Players read "Esperando o mestre" or
// "Esperando <nome>" (reaction.Wait).

// contestMaxListed is how many of the latest contests a sync looks at.
const contestMaxListed = 200

// contestReactor is who a waiting contest asks next: nil for the master.
func contestReactor(row playdb.CombatContest, byID map[string]playdb.Combatant) *playdb.Combatant {
	switch row.Status {
	case contestAwaitingDefender:
		d, ok := byID[row.DefenderID]
		if !ok || d.Kind != kindPlayer {
			return nil
		}
		if r, err := decodeRoll(row.DefenderRoll); err == nil && r != nil && r.Deferred {
			return nil
		}
		return &d
	case contestAwaitingOutcome:
		i, ok := byID[row.InitiatorID]
		if ok && i.Kind == kindPlayer {
			return &i
		}
	}
	return nil
}

// syncContests opens the window of each waiting contest that has none, and replaces the
// window whose reactor changed (a roll left to the master).
func (s *Service) syncContests(ctx context.Context, c *combatTx) error {
	if c.enc.ID == "" || c.enc.Status != statusActive {
		return nil
	}
	rows, err := c.q.ListContests(ctx, playdb.ListContestsParams{EncounterID: c.enc.ID, Limit: contestMaxListed})
	if err != nil {
		return fmt.Errorf("list the contests: %w", err)
	}
	waiting := rows[:0:0]
	for _, r := range rows {
		if r.Status == contestAwaitingDefender || r.Status == contestAwaitingOutcome {
			waiting = append(waiting, r)
		}
	}
	open, err := openWindowsOf(ctx, c)
	if err != nil {
		return err
	}
	have := map[string]playdb.ReactionWindow{}
	for _, w := range open {
		if reaction.Kind(w.Kind) == reaction.Contest {
			have[windowTriggerOf(w).Contest] = w
		}
	}
	if len(waiting) == 0 && len(have) == 0 {
		return nil
	}
	cs, err := c.q.ListCombatants(ctx, c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the combatants: %w", err)
	}
	byID := make(map[string]playdb.Combatant, len(cs))
	for _, x := range cs {
		byID[x.ID] = x
	}
	for _, row := range waiting {
		reactor := contestReactor(row, byID)
		if w, ok := have[row.ID]; ok {
			if deref(w.ReactorID) == idOf(reactor) {
				continue
			}
			if _, err := s.closeWindow(ctx, c, w, windowClosed, reaction.ReasonTriggerGone, windowOutcome{}, ""); err != nil {
				return err
			}
		}
		if _, err := s.openWindows(ctx, c, row.ID, nil, []windowSpec{{
			kind: reaction.Contest, reactor: reactor,
			trigger: windowTrigger{Contest: row.ID, Actor: row.InitiatorID, Target: row.DefenderID},
		}}); err != nil {
			return err
		}
	}
	return nil
}

func idOf(c *playdb.Combatant) string {
	if c == nil {
		return ""
	}
	return c.ID
}

// contestStillWaits says whether the contest of a CONTEST window still waits.
func contestStillWaits(ctx context.Context, c *combatTx, w playdb.ReactionWindow) (bool, error) {
	row, err := c.q.GetContest(ctx, playdb.GetContestParams{EncounterID: c.enc.ID, ID: windowTriggerOf(w).Contest})
	if err != nil {
		return false, nil //nolint:nilerr // a contest that is not there waits for nothing
	}
	return row.Status == contestAwaitingDefender || row.Status == contestAwaitingOutcome, nil
}

// errContestWindow is the refusal of AnswerReaction for a CONTEST window.
func errContestWindow() error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New("a contest is answered with RespondContest or ResolveShove"))
}
