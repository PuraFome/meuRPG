package play

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/play/cutting"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// A check that waits is read as things stand, not as they stood when it was rolled. The next read of the held rolls or of the check, and
// the next answer, find what changed since and settle it in a transaction of their own, before they go on: the creature's token the master
// hid closes every question as the skip does (the bard asked now and the master are told), and a bard that is not a character in play of a
// member any more (in the reserve, dead, deleted, or with no player) has its question let pass, the next bard is asked and told, and the
// check settles as the last answer settles it when none is left. The hide is a write of the maps module (and the stage's of this one, which
// only changes the name the offers give), so nothing is settled by it directly: the checks settle where they are read.

// errNoCheckToKeep says the session closed between the read and the write: there is nothing to keep.
var errNoCheckToKeep = errors.New("no open session to keep the checks of")

// npcCheckView is what a check that waits is measured against: the tokens of the session's map, and the characters in play of a member.
type npcCheckView struct {
	tokens map[string]cutting.Token
	inPlay map[string]bool
}

// npcCheckViewOf reads the tokens of the session's current map and the party of the campaign.
func (s *Service) npcCheckViewOf(ctx context.Context, tx pgx.Tx, session playdb.GameSession) (npcCheckView, error) {
	tokens, err := s.mapTokensOf(ctx, tx, session)
	if err != nil {
		return npcCheckView{}, err
	}
	party, err := s.roster.CombatParty(ctx, tx, session.CampaignID)
	if err != nil {
		return npcCheckView{}, fmt.Errorf("read the party: %w", err)
	}
	v := npcCheckView{tokens: tokens, inPlay: map[string]bool{}}
	for _, c := range party {
		v.inPlay[c.ID] = c.PlayerUserID != ""
	}
	return v, nil
}

// moveOf is what the check needs now, with its asks.
func (v npcCheckView) moveOf(hold playdb.InspirationHold, asks []playdb.NpcCheckAsk) (cutting.Move, int) {
	inPlay := make([]bool, len(asks))
	for i, a := range asks {
		inPlay[i] = a.BardCharacterID != nil && v.inPlay[*a.BardCharacterID]
	}
	return cutting.NextMove(askStates(asks), inPlay, v.tokens[hold.CharacterID].Hidden)
}

// keepNpcChecks settles what changed in the checks that wait (see the top of the file). It writes only when a check needs it.
func (s *Service) keepNpcChecks(ctx context.Context, campaignID string) error {
	holds, err := s.queries.ListOpenNpcCheckHolds(ctx, campaignID)
	if err != nil {
		return s.dbError(ctx, "list the checks held for the bards", err)
	}
	if len(holds) == 0 {
		return nil
	}
	session, err := s.queries.GetOpenGameSession(ctx, campaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return s.dbError(ctx, "find the open session", err)
	}
	var due bool
	err = db.ReadTx(ctx, s.pool, func(tx pgx.Tx) error { // a read first: a table where nothing changed takes no lock
		var err error
		due, err = s.npcChecksDue(ctx, tx, s.queries.WithTx(tx), session, holds)
		return err
	})
	if err != nil || !due {
		return wrapKeep(s, ctx, err)
	}
	err = s.txOpenOr(ctx, campaignID, func() error { return errNoCheckToKeep }, func(tx pgx.Tx, h *hints, session playdb.GameSession) error {
		q := s.queries.WithTx(tx)
		holds, err := q.ListOpenNpcCheckHolds(ctx, campaignID)
		if err != nil {
			return fmt.Errorf("list the checks held for the bards: %w", err)
		}
		view, err := s.npcCheckViewOf(ctx, tx, session)
		if err != nil {
			return err
		}
		tc, err := s.openTx(ctx, combatTx{tx: tx, q: q, session: session, now: s.now()})
		if err != nil {
			return err
		}
		for _, hold := range holds {
			if hold.CreatedAt.Before(session.StartedAt) {
				continue
			}
			if err := s.keepNpcCheck(ctx, tc, h, view, hold); err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, errNoCheckToKeep) {
		return nil
	}
	return wrapKeep(s, ctx, err)
}

// wrapKeep is the error of keepNpcChecks as the paths that call it answer it.
func wrapKeep(s *Service, ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	return s.dbError(ctx, "settle the checks held for the bards", err)
}

// npcChecksDue says some check of this session needs a move.
func (s *Service) npcChecksDue(ctx context.Context, tx pgx.Tx, q *playdb.Queries, session playdb.GameSession, holds []playdb.InspirationHold) (bool, error) {
	var view *npcCheckView
	for _, hold := range holds {
		if hold.CreatedAt.Before(session.StartedAt) {
			continue
		}
		if view == nil {
			v, err := s.npcCheckViewOf(ctx, tx, session)
			if err != nil {
				return false, err
			}
			view = &v
		}
		asks, err := q.ListNpcCheckAsks(ctx, hold.ID)
		if err != nil {
			return false, fmt.Errorf("list the asks of the check: %w", err)
		}
		if move, _ := view.moveOf(hold, asks); move != cutting.Stand {
			return true, nil
		}
	}
	return false, nil
}

// keepNpcCheck makes the moves one check needs: the creature hidden closes it as a skip; each question whose bard is out of play is let pass
// in turn, the bard asked next and the master are told, and the check settles when no question is left.
func (s *Service) keepNpcCheck(ctx context.Context, tc *combatTx, h *hints, view npcCheckView, hold playdb.InspirationHold) error {
	moved := false
	for {
		asks, err := tc.q.ListNpcCheckAsks(ctx, hold.ID)
		if err != nil {
			return fmt.Errorf("list the asks of the check: %w", err)
		}
		move, i := view.moveOf(hold, asks)
		switch move {
		case cutting.Skip:
			_, err := s.skipNpcCheck(ctx, tc, h, hold, asks, uuid.NewString())
			return err
		case cutting.Finish:
			if _, err := s.settleNpcCheck(ctx, tc, hold, uuid.NewString(), false); err != nil {
				return err
			}
			tellNpcCheck(h, tc.session.CampaignID)
			return nil
		case cutting.Pass:
			if _, err := tc.q.PassNpcCheckAsk(ctx, playdb.PassNpcCheckAskParams{ID: asks[i].ID, AnsweredAt: &tc.now}); err != nil {
				return fmt.Errorf("let the question of a bard that left pass: %w", err)
			}
			moved = true
		default:
			if moved { // the next bard is asked now
				cur := cutting.Current(askStates(asks))
				tellNpcCheck(h, tc.session.CampaignID, s.bardUser(ctx, tc.tx, tc.session.CampaignID, asks[cur].BardCharacterID))
			}
			return nil
		}
	}
}

// npcHiddenNow says the master hides the creature from the players now: its mark on the stage says so (stage), or its token on the
// session's current map (token). What the bards read of a check is read when the offer is built, not when the check was rolled.
func (s *Service) npcHiddenNow(ctx context.Context, session playdb.GameSession, npcID string) (stage, token bool, err error) {
	rows, err := s.queries.ListStage(ctx, session.ID)
	if err != nil {
		return false, false, fmt.Errorf("list the stage: %w", err)
	}
	stage = slices.ContainsFunc(rows, func(r playdb.StageNpc) bool { return r.CharacterID == npcID && r.Hidden })
	if session.CurrentMapID == nil {
		return stage, false, nil
	}
	tokens, err := s.maps.MapTokens(ctx, nil, *session.CurrentMapID)
	if err != nil {
		return false, false, fmt.Errorf("read the tokens: %w", err)
	}
	token = slices.ContainsFunc(tokens, func(t link.TokenPosition) bool { return t.CharacterID == npcID && t.Hidden })
	return stage, token, nil
}
