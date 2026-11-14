package play

import (
	"context"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The causes a dismount window keeps in its trigger.
const (
	mountCauseMountProne    = "mount_prone"
	mountCauseMountDefeated = "mount_defeated"
)

// openDismount asks the rider whose mount was knocked prone, or defeated, whether it lands on its feet.
func (s *Service) openDismount(_ context.Context, _ *combatTx, _ []playdb.Combatant, _, _ playdb.Combatant, _ string) error {
	return nil
}
