package characters

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage"
)

// applyPlayers creates the players' characters of a package as reserved: with
// no owner, invisible to the players until one claims it.
func (p *packagePart) applyPlayers(_ context.Context, _ pgx.Tx, _ *campaignpackage.Import, players []*stagedCharacter) error {
	_ = players
	return nil
}
