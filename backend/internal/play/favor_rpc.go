package play

import (
	"context"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
)

// AnswerFavorRefund is the master's answer to a refund prompt. It is not built yet and answers unimplemented, after the check of
// who asks: only the master.
func (s *Service) AnswerFavorRefund(ctx context.Context, req *connect.Request[playv1.AnswerFavorRefundRequest]) (*connect.Response[playv1.AnswerFavorRefundResponse], error) {
	if _, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster); err != nil {
		return nil, err
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
