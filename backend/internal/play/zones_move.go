package play

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// MoveSpellZone implements playv1connect.CombatServiceHandler.
func (s *Service) MoveSpellZone(
	_ context.Context,
	_ *connect.Request[playv1.MoveSpellZoneRequest],
) (*connect.Response[playv1.MoveSpellZoneResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not yet"))
}
