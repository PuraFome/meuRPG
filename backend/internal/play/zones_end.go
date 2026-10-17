package play

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// EndSpellZone implements playv1connect.CombatServiceHandler.
func (s *Service) EndSpellZone(
	_ context.Context,
	_ *connect.Request[playv1.EndSpellZoneRequest],
) (*connect.Response[playv1.EndSpellZoneResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not yet"))
}
