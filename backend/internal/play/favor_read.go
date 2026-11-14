package play

import (
	"context"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
)

// GetFavor implements playv1connect.FavorServiceHandler: the master reads every holder of the campaign, a player reads only the
// holders among their own characters, and a member with none reads an empty list. It needs no open session.
func (s *Service) GetFavor(ctx context.Context, req *connect.Request[playv1.GetFavorRequest]) (*connect.Response[playv1.GetFavorResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListCampaignFavor(ctx, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list who has the inspiration", err)
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.CharacterID
	}
	// The names are the characters module's: the query cannot join its table.
	chars, err := s.roster.SessionCharacters(ctx, nil, m.CampaignID, ids)
	if err != nil {
		return nil, s.dbError(ctx, "name who has the inspiration", err)
	}
	byID := make(map[string]link.Character, len(chars))
	for _, c := range chars {
		byID[c.ID] = c
	}
	holders := make([]*playv1.FavorHolder, 0, len(rows))
	for _, r := range rows {
		if c, ok := byID[r.CharacterID]; ok && (m.Role == authz.RoleMaster || (c.PlayerUserID == m.UserID && !c.Reserved)) {
			holders = append(holders, &playv1.FavorHolder{CharacterId: c.ID, CharacterName: c.Name})
		}
	}
	return connect.NewResponse(&playv1.GetFavorResponse{Holders: holders}), nil
}
