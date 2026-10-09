package play

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// CharacterInCombat says whether the character is a combatant of the campaign's combat
// that is not ended, inside tx. Package characters asks before it gives a claimed
// character back to the reserve (MR-049): a reserved character is invisible to the
// players, and a combatant is on every player's screen.
func (s *Service) CharacterInCombat(ctx context.Context, tx pgx.Tx, campaignID, characterID string) (bool, error) {
	in, err := s.queries.WithTx(tx).CharacterIsInOpenCombat(ctx, playdb.CharacterIsInOpenCombatParams{CampaignID: campaignID, CharacterID: characterID})
	if err != nil {
		return false, fmt.Errorf("find the character in a combat: %w", err)
	}
	return in, nil
}
