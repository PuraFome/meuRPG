package progression

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// AwardedXP says how much XP the campaign has given and not undone: how many
// awards stand (a milestone mark counts, with no XP) and the XP the characters
// got from them (RN-09). Package campaigns asks it through its own interface
// before it changes the XP mode, to know whether the change needs a
// confirmation. It reads inside tx when the caller has one (nil: the pool).
func (s *Service) AwardedXP(ctx context.Context, tx pgx.Tx, campaignID string) (awards int32, totalXP int64, err error) {
	row, err := s.queriesIn(tx).SumLiveAwards(ctx, campaignID)
	if err != nil {
		return 0, 0, fmt.Errorf("sum the live awards: %w", err)
	}
	return row.Awards, row.TotalXp, nil
}
