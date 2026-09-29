package characters

import (
	"context"

	"connectrpc.com/connect"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
)

// ListContent implements rulesv1connect.ContentServiceHandler.
//
// It lives here, not in package campaigns as ADR-0008 sketches, because
// today the content is only the SRD snapshot that package rules embeds:
// there is no table content to read from the database yet. Every campaign
// gets the same catalog, built once in New.
func (s *Service) ListContent(
	ctx context.Context,
	req *connect.Request[rulesv1.ListContentRequest],
) (*connect.Response[rulesv1.ListContentResponse], error) {
	if _, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId()); err != nil {
		return nil, err
	}
	// The catalog is shared and never modified; marshaling it from several
	// requests at once is safe.
	return connect.NewResponse(&rulesv1.ListContentResponse{Content: s.catalog}), nil
}
