package play

import (
	"context"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// RollMountSave answers a rider's pending mount save.
func (s *Service) RollMountSave(_ context.Context, _ *connect.Request[playv1.RollMountSaveRequest]) (*connect.Response[playv1.RollMountSaveResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
