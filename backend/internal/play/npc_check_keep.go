package play

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/play/cutting"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// A check that waits is read as things stand, not as they stood when it was rolled. The next read of the held rolls or of the check, and
// the next answer, find what changed since and settle it in a transaction of their own, before they go on: the creature's token the master
// hid on the map the check was rolled on closes every question as the skip does (the bard asked now and the master are told), and a bard that is not a character in play of a
// member any more (in the reserve, dead, deleted, or with no player), or whose player the open scene is no longer open to, has its question let pass, the next bard is asked and told, and the
// check settles as the last answer settles it when none is left. The hide is a write of the maps module (and the stage's of this one, which
// only changes the name the offers give), so nothing is settled by it directly: the checks settle where they are read.

// errNoCheckToKeep says the session closed between the read and the write: there is nothing to keep.
var errNoCheckToKeep = errors.New("no open session to keep the checks of")

// npcCheckView is what a check that waits is measured against: the tokens of the map each check was rolled on, and the characters in
// play of a member whose player the open scene is open to.
type npcCheckView struct {
	s      *Service
	tokens map[string]map[string]cutting.Token // by map; "" is no map
	inPlay map[string]bool
}

// npcCheckViewOf reads the viewers of the open scene and the party of the campaign.
func (s *Service) npcCheckViewOf(ctx context.Context, tx pgx.Tx, session playdb.GameSession) (*npcCheckView, error) {
	scene, err := sceneViewersOf(ctx, s.queries.WithTx(tx), session)
	if err != nil {
		return nil, err
	}
	party, err := s.roster.CombatParty(ctx, tx, session.CampaignID)
	if err != nil {
		return nil, fmt.Errorf("read the party: %w", err)
	}
	v := &npcCheckView{s: s, tokens: map[string]map[string]cutting.Token{}, inPlay: map[string]bool{}}
	for _, c := range party {
		v.inPlay[c.ID] = c.PlayerUserID != "" && scene.has(c.PlayerUserID)
	}
	return v, nil
}

// tokensOf reads the tokens of the map the check was rolled on, once for each map.
func (v *npcCheckView) tokensOf(ctx context.Context, tx pgx.Tx, campaignID string, mapID *string) (map[string]cutting.Token, error) {
	if t, ok := v.tokens[conv.OrZero(mapID)]; ok {
		return t, nil
	}
	t, err := v.s.tokensOfMap(ctx, tx, campaignID, mapID)
	if err != nil {
		return nil, err
	}
	v.tokens[conv.OrZero(mapID)] = t
	return t, nil
}

// moveOf is what the check needs now, with its asks: the creature is hidden when its token is hidden on the map the check was rolled on.
func (v *npcCheckView) moveOf(ctx context.Context, tx pgx.Tx, hold playdb.InspirationHold, asks []playdb.NpcCheckAsk) (cutting.Move, int, error) {
	tokens, err := v.tokensOf(ctx, tx, hold.CampaignID, rolledOn(asks))
	if err != nil {
		return cutting.Stand, -1, err
	}
	inPlay := make([]bool, len(asks))
	for i, a := range asks {
		inPlay[i] = a.BardCharacterID != nil && v.inPlay[*a.BardCharacterID]
	}
	move, i := cutting.NextMove(askStates(asks), inPlay, tokens[hold.CharacterID].Hidden)
	return move, i, nil
}

// rolledOn is the map the check was rolled on, nil when it was rolled with no map: every ask of a check keeps it.
func rolledOn(asks []playdb.NpcCheckAsk) *string {
	if len(asks) == 0 {
		return nil
	}
	return asks[0].MapID
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
		return s.keepError(ctx, err)
	}
	err = s.txOpenOr(ctx, campaignID, func() error { return errNoCheckToKeep }, func(tx pgx.Tx, h *hints, session playdb.GameSession) error {
		return s.keepNpcChecksIn(ctx, tx, h, session)
	})
	if errors.Is(err, errNoCheckToKeep) {
		return nil
	}
	return s.keepError(ctx, err)
}

// keepNpcChecksIn makes the moves every check of the session needs, in the transaction of the keep.
func (s *Service) keepNpcChecksIn(ctx context.Context, tx pgx.Tx, h *hints, session playdb.GameSession) error {
	q := s.queries.WithTx(tx)
	holds, err := q.ListOpenNpcCheckHolds(ctx, session.CampaignID)
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
}

// keepError is the error of keepNpcChecks as the paths that call it answer it.
func (s *Service) keepError(ctx context.Context, err error) error {
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
			view = v
		}
		asks, err := q.ListNpcCheckAsks(ctx, hold.ID)
		if err != nil {
			return false, fmt.Errorf("list the asks of the check: %w", err)
		}
		move, _, err := view.moveOf(ctx, tx, hold, asks)
		if err != nil {
			return false, err
		}
		if move != cutting.Stand {
			return true, nil
		}
	}
	return false, nil
}

// keepNpcCheck makes the moves one check needs: the creature hidden closes it as a skip; each question whose bard is out of play is let pass
// in turn, the bard asked next and the master are told, and the check settles when no question is left.
func (s *Service) keepNpcCheck(ctx context.Context, tc *combatTx, h *hints, view *npcCheckView, hold playdb.InspirationHold) error {
	moved := false
	for {
		asks, err := tc.q.ListNpcCheckAsks(ctx, hold.ID)
		if err != nil {
			return fmt.Errorf("list the asks of the check: %w", err)
		}
		move, i, err := view.moveOf(ctx, tc.tx, hold, asks)
		if err != nil {
			return err
		}
		switch move {
		case cutting.Skip:
			_, err := s.skipNpcCheck(ctx, tc, h, hold, asks, uuid.NewString())
			return err
		case cutting.Finish:
			return s.finishNpcCheck(ctx, tc, h, hold)
		case cutting.Pass:
			if _, err := tc.q.PassNpcCheckAsk(ctx, playdb.PassNpcCheckAskParams{ID: asks[i].ID, AnsweredAt: &tc.now}); err != nil {
				return fmt.Errorf("let the question of a bard that left pass: %w", err)
			}
			moved = true
		default:
			if moved { // the next bard is asked now
				return s.tellNextBard(ctx, tc, h, asks)
			}
			return nil
		}
	}
}

// finishNpcCheck settles a check no question of which is open any more, as the last answer settles it.
func (s *Service) finishNpcCheck(ctx context.Context, tc *combatTx, h *hints, hold playdb.InspirationHold) error {
	if _, err := s.settleNpcCheck(ctx, tc, hold, uuid.NewString(), false); err != nil {
		return err
	}
	tellNpcCheck(h, tc.session.CampaignID)
	return nil
}

// tellNextBard tells the master and the bard asked now that the question moved.
func (s *Service) tellNextBard(ctx context.Context, tc *combatTx, h *hints, asks []playdb.NpcCheckAsk) error {
	cur := cutting.Current(askStates(asks))
	next, err := s.bardUser(ctx, tc.tx, tc.session.CampaignID, asks[cur].BardCharacterID)
	if err != nil {
		return err
	}
	tellNpcCheck(h, tc.session.CampaignID, next)
	return nil
}

// npcHiddenNow says the master hides the creature from the players now: its mark on the stage says so (stage), or its token on the
// map the check was rolled on (token; none with no map). What the bards read of a check is read when the offer is built, not when the
// check was rolled.
func (s *Service) npcHiddenNow(ctx context.Context, session playdb.GameSession, rolledOn *string, npcID string) (stage, token bool, err error) {
	rows, err := s.queries.ListStage(ctx, session.ID)
	if err != nil {
		return false, false, fmt.Errorf("list the stage: %w", err)
	}
	stage = slices.ContainsFunc(rows, func(r playdb.StageNpc) bool { return r.CharacterID == npcID && r.Hidden })
	if rolledOn == nil {
		return stage, false, nil
	}
	tokens, err := s.maps.MapTokens(ctx, nil, *rolledOn)
	if err != nil {
		return false, false, fmt.Errorf("read the tokens: %w", err)
	}
	token = slices.ContainsFunc(tokens, func(t link.TokenPosition) bool { return t.CharacterID == npcID && t.Hidden })
	return stage, token, nil
}
