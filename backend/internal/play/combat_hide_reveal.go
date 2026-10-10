package play

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
)

// The master reveals a hider (SRD 5.1, "Hiding": the DM decides what a creature notices,
// and a hider is found when it makes noise, steps into the open or is searched for). The
// hiding ends for every creature, through endHiding, the function an attack or a cast uses.

// contestLineHideRevealed is the log line of the reveal. It is the master's and the hider's
// player's: the other players are not told that anyone was hiding (RN-10).
const contestLineHideRevealed = "hide_revealed"

// RevealHider implements playv1connect.ContestServiceHandler.
func (s *Service) RevealHider(
	ctx context.Context,
	req *connect.Request[playv1.RevealHiderRequest],
) (*connect.Response[playv1.RevealHiderResponse], error) {
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

	var made actionEvent
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventHideResolved, encounterID: encID}, func(c *combatTx) (any, error) {
		made = actionEvent{}
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		hider, ok := combatantByID(cs, combID)
		if !ok {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("combatant not found"))
		}
		snaps, err := s.endHiding(ctx, c, hider)
		if err != nil {
			return nil, err
		}
		if len(snaps) == 0 {
			return nil, errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_HIDDEN, "the combatant is not hidden")
		}
		if err := touch(ctx, c); err != nil {
			return nil, err
		}
		c.characterID = &hider.CharacterID
		// Secret keeps the line from the other players; the log shows it to the hider's
		// own player (logEntry.ownerToo). HidBefore is what the master's undo gives back.
		made = actionEvent{
			Round: c.enc.Round, Secret: true, Actor: hider.ID, HidBefore: snaps,
			Contest: &contestEvent{Line: contestLineHideRevealed},
		}
		return made, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "reveal a hider", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChanged(ctx, m.CampaignID, d.enc) // the hider's player reads the state change
		s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, true)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.RevealHiderResponse{Encounter: out}), nil
}

// undoableHideResolved says whether a hide_resolved event can be undone: only the reveal
// can (its hiding comes back); the master's decision on an attempt cannot.
func undoableHideResolved(payload []byte) bool {
	ev, err := readEvent(payload)
	return err == nil && ev.Contest != nil && ev.Contest.Line == contestLineHideRevealed
}
