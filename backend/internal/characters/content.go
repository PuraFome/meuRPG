package characters

import (
	"context"

	"connectrpc.com/connect"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
)

// ListContent implements rulesv1connect.ContentServiceHandler.
//
// It lives here, not in package campaigns as ADR-0008 sketches: the content
// comes from the ContentSource, which today gives every campaign the SRD
// snapshot that package rules embeds. The catalog is built once per content
// (catalogFor), so a campaign with its own content gets its own.
func (s *Service) ListContent(
	ctx context.Context,
	req *connect.Request[rulesv1.ListContentRequest],
) (*connect.Response[rulesv1.ListContentResponse], error) {
	// A pending member may read it too (RN-15): the editor needs it to
	// create their character. The catalog is the same for everyone.
	if _, err := authz.RequireCampaignMemberOrPending(ctx, req.Msg.GetCampaignId()); err != nil {
		return nil, err
	}
	content, err := s.contentFor(ctx, nil, req.Msg.GetCampaignId())
	if err != nil {
		return nil, s.dbError(ctx, "read rules content", err)
	}
	// A content's catalog is shared and never modified; marshaling it from
	// several requests at once is safe.
	return connect.NewResponse(&rulesv1.ListContentResponse{Content: s.catalogFor(content)}), nil
}
