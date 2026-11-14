package play

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// RollMountSave implements playv1connect.CombatServiceHandler.
func (s *Service) RollMountSave(
	_ context.Context,
	_ *connect.Request[playv1.RollMountSaveRequest],
) (*connect.Response[playv1.RollMountSaveResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("RollMountSave is not written yet"))
}

// openMountSave opens the save of a rider that sits on the mount (combat_mount_save_open_test.go calls it as the triggers do).
func (s *Service) openMountSave(_ context.Context, _ *combatTx, _ []playdb.Combatant, _, _ playdb.Combatant, _ playv1.MountSaveCause) error {
	return nil
}
