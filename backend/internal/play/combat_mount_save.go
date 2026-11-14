package play

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// RollMountSave implements playv1connect.CombatServiceHandler.
func (s *Service) RollMountSave(
	_ context.Context,
	_ *connect.Request[playv1.RollMountSaveRequest],
) (*connect.Response[playv1.RollMountSaveResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("RollMountSave is not written yet"))
}
