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

// AnswerFavorPass is the master's answer to a pending pass. It is not built yet and answers unimplemented, after the check of
// who asks: only the master, like the grant.
func (s *Service) AnswerFavorPass(ctx context.Context, req *connect.Request[playv1.AnswerFavorPassRequest]) (*connect.Response[playv1.AnswerFavorPassResponse], error) {
	if _, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster); err != nil {
		return nil, err
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// CancelFavorPass gives a pending ask up. It is not built yet and answers unimplemented, after the check of who asks: a member of
// the campaign (the giver's owner is checked when it is built, so a stranger learns nothing).
func (s *Service) CancelFavorPass(ctx context.Context, req *connect.Request[playv1.CancelFavorPassRequest]) (*connect.Response[playv1.CancelFavorPassResponse], error) {
	if _, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId()); err != nil {
		return nil, err
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// AnswerFavorRefund is the master's answer to a refund prompt. It is not built yet and answers unimplemented, after the check of
// who asks: only the master.
func (s *Service) AnswerFavorRefund(ctx context.Context, req *connect.Request[playv1.AnswerFavorRefundRequest]) (*connect.Response[playv1.AnswerFavorRefundResponse], error) {
	if _, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster); err != nil {
		return nil, err
	}
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
