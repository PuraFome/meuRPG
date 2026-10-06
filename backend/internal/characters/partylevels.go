package characters

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// PartyTotalLevels returns the total level of each living, active player
// character of the campaign, oldest first: what the treasure generator takes the
// party level from (MR-044, TreasureService, the lowest of them). It is
// PartyLevels (the encounter builder's party) with only the levels. The maps
// module asks for it after its own authorization check, so it takes no caller. It
// reads inside tx when the caller has one (nil: the pool) and implements
// maps.CharacterDirectory.
func (s *Service) PartyTotalLevels(ctx context.Context, tx pgx.Tx, campaignID string) ([]int, error) {
	party, err := s.PartyLevels(ctx, tx, campaignID)
	if err != nil {
		return nil, err
	}
	out := make([]int, len(party))
	for i, m := range party {
		out[i] = m.Level
	}
	return out, nil
}
