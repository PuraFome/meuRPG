package play

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// MountCombatant implements playv1connect.CombatServiceHandler.
func (s *Service) MountCombatant(
	_ context.Context,
	_ *connect.Request[playv1.MountCombatantRequest],
) (*connect.Response[playv1.MountCombatantResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// DismountCombatant implements playv1connect.CombatServiceHandler.
func (s *Service) DismountCombatant(
	_ context.Context,
	_ *connect.Request[playv1.DismountCombatantRequest],
) (*connect.Response[playv1.DismountCombatantResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}
