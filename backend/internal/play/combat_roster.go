package play

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
)

// The roster of a combat about to start (plan B, B2): who has a token on the map of the fight and the
// square it gives. StartEncounter already puts each combatant on its token's square; this read lets the
// master see it first and choose who enters.

// rosterEntry is a character of the party and its place on the map of the fight.
type rosterEntry struct {
	id           string
	onMap        bool
	col, row     int32
	sharesSquare bool
}

// rosterOf lists the party with the squares of their tokens, in the party's order. A character
// whose token stands on a square an earlier one's does would start with no square (joinRow).
func rosterOf(party []link.Character, tokens []link.TokenPosition, g link.Grid) []rosterEntry {
	out := make([]rosterEntry, 0, len(party))
	for _, c := range party {
		e := rosterEntry{id: c.ID}
		if t, ok := tokenOf(tokens, c.ID); ok {
			e.onMap = true
			e.col, e.row = squareOf(g, t.XBP, t.YBP)
			for _, o := range out {
				if o.onMap && !o.sharesSquare && o.col == e.col && o.row == e.row {
					e.sharesSquare = true
				}
			}
		}
		out = append(out, e)
	}
	return out
}

// GetStartRoster implements playv1connect.CombatServiceHandler.
func (s *Service) GetStartRoster(
	ctx context.Context,
	req *connect.Request[playv1.GetStartRosterRequest],
) (*connect.Response[playv1.GetStartRosterResponse], error) {
	m, err := requireBuilder(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	pointMap := ""
	if req.Msg.GetMapPointId() != "" {
		pointID, _, err := s.battlePointID(ctx, m.CampaignID, req.Msg.GetMapPointId())
		if err != nil {
			return nil, err
		}
		point, err := s.maps.BattlePoint(ctx, nil, m.CampaignID, pointID)
		if err != nil {
			return nil, s.dbError(ctx, "find the battle point", err)
		}
		pointMap = point.TargetMapID
	}
	res := &playv1.GetStartRosterResponse{}
	err = db.ReadTx(ctx, s.pool, func(tx pgx.Tx) error {
		return s.fillStartRoster(ctx, tx, m.CampaignID, pointMap, res)
	})
	if err != nil {
		return nil, s.dbError(ctx, "read the party's places", err)
	}
	return connect.NewResponse(res), nil
}

// fillStartRoster reads the open session's map (or the battle point's destination), its grid and its
// tokens and the party, and fills the answer.
func (s *Service) fillStartRoster(ctx context.Context, tx pgx.Tx, campaignID, pointMap string, res *playv1.GetStartRosterResponse) error {
	session, err := openSessionWith(ctx, s.queries.WithTx(tx), campaignID)
	if err != nil {
		return err
	}
	party, err := s.roster.CombatParty(ctx, tx, campaignID)
	if err != nil {
		return fmt.Errorf("read the party: %w", err)
	}
	mapID := conv.OrZero(session.CurrentMapID)
	if pointMap != "" {
		mapID = pointMap
	}
	var g link.Grid
	var tokens []link.TokenPosition
	if mapID != "" {
		if g, err = s.maps.MapGrid(ctx, tx, campaignID, mapID); err != nil && !isNotFound(err) {
			return err
		}
		if g.OK() {
			res.MapId = mapID
			if tokens, err = s.maps.MapTokens(ctx, tx, mapID); err != nil {
				return fmt.Errorf("read the tokens: %w", err)
			}
		}
	}
	for _, e := range rosterOf(party, tokens, g) {
		res.Party = append(res.Party, &playv1.StartRosterEntry{CharacterId: e.id, OnMap: e.onMap, Col: e.col, Row: e.row, SharesSquare: e.sharesSquare})
	}
	return nil
}

// isNotFound says an error is a Connect not_found.
func isNotFound(err error) bool {
	var ce *connect.Error
	return errors.As(err, &ce) && ce.Code() == connect.CodeNotFound
}
