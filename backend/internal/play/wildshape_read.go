package play

import (
	"context"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
)

// GetCharacterWildShape implements playv1connect.PlayServiceHandler: the sheet's read of a druid's form when no
// session is open (GetLiveSession needs one). It is the form of the vitals, with its clock, for the readers of
// WildShapeState: the master and the character's player. Everyone else, and a character that is not readable at
// all, gets the one `not_found` the sheet already has for a character it cannot read, so no answer says a form exists.
func (s *Service) GetCharacterWildShape(
	ctx context.Context,
	req *connect.Request[playv1.GetCharacterWildShapeRequest],
) (*connect.Response[playv1.GetCharacterWildShapeResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	v, err := s.vitals.GetVitals(ctx, m.CampaignID, req.Msg.GetCharacterId())
	if err != nil {
		return nil, err
	}
	if !maySee(m, v) {
		return nil, errCharacterNotFound()
	}
	return connect.NewResponse(&playv1.GetCharacterWildShapeResponse{WildShape: v.GetWildShape()}), nil
}
