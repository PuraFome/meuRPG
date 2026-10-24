package play

import (
	"context"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// SetSceneAudience implements playv1connect.PlayServiceHandler.
func (s *Service) SetSceneAudience(
	_ context.Context,
	_ *connect.Request[playv1.SetSceneAudienceRequest],
) (*connect.Response[playv1.SetSceneAudienceResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// SetShownImageAudience implements playv1connect.PlayServiceHandler.
func (s *Service) SetShownImageAudience(
	_ context.Context,
	_ *connect.Request[playv1.SetShownImageAudienceRequest],
) (*connect.Response[playv1.SetShownImageAudienceResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// ImageShownTo says whether the image is the one the open session shows and the user sees it.
// It implements maps.LiveSession.
func (s *Service) ImageShownTo(ctx context.Context, campaignID, _, imageID string) (bool, error) {
	_, shown, err := s.OnScreen(ctx, campaignID)
	return shown != "" && shown == imageID, err
}
