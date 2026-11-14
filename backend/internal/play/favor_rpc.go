package play

import (
	"context"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
)

// PassFavor hands the holder's Inspiration to another player character. It is not built yet and answers unimplemented, after
// the check of who asks: a member of the campaign, like the reads, so a stranger learns nothing.
func (s *Service) PassFavor(ctx context.Context, req *connect.Request[playv1.PassFavorRequest]) (*connect.Response[playv1.PassFavorResponse], error) {
	if _, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId()); err != nil {
		return nil, err
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
