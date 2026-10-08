package play

import (
	"context"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// PreviewSpellArea implements playv1connect.CombatServiceHandler.
func (s *Service) PreviewSpellArea(
	_ context.Context,
	_ *connect.Request[playv1.PreviewSpellAreaRequest],
) (*connect.Response[playv1.PreviewSpellAreaResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// ResolveHiddenReveal implements playv1connect.CombatServiceHandler.
func (s *Service) ResolveHiddenReveal(
	_ context.Context,
	_ *connect.Request[playv1.ResolveHiddenRevealRequest],
) (*connect.Response[playv1.ResolveHiddenRevealResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
