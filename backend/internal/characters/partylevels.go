package characters

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// PartyLevels returns the total level of each living, active player character
// of the campaign, oldest first: what the treasure generator takes the party
// level from (MR-044, TreasureService, the lowest of them). The maps module asks
// for it after its own authorization check, so it takes no caller. It reads
// inside tx when the caller has one (nil: the pool) and implements
// maps.CharacterDirectory.
func (s *Service) PartyLevels(ctx context.Context, tx pgx.Tx, campaignID string) ([]int, error) {
	rows, err := s.queriesIn(tx).ListCombatParty(ctx, campaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list the party", err)
	}
	content, err := s.contentFor(ctx, tx, campaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read rules content", err)
	}
	out := make([]int, 0, len(rows))
	for _, row := range rows {
		sheet, err := loadSheet(row.ID, row.Sheet)
		if err != nil {
			return nil, s.dbError(ctx, "list the party", err)
		}
		full := sheet.GetFull()
		if full == nil {
			continue // never: a player character has a full sheet
		}
		out = append(out, rules.Derive(buildOf(full), content).TotalLevel)
	}
	return out, nil
}
