package characters

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
)

// applyPlayers creates the players' characters of a package as reserved: with
// no owner, invisible to the players until one claims it through a link
// (MR-049, RN-10). The master's notes about each come with it.
func (p *packagePart) applyPlayers(ctx context.Context, tx pgx.Tx, in *campaignpackage.Import, players []*stagedCharacter) error {
	q := p.s.queries.WithTx(tx)
	for _, c := range players {
		if err := q.InsertImportedReservedCharacter(ctx, charactersdb.InsertImportedReservedCharacterParams{
			ID: c.id, CampaignID: in.CampaignID, Name: c.name, Sheet: c.sheet, Story: c.story, Now: c.createdAt,
		}); err != nil {
			return fmt.Errorf("insert an imported reserved character: %w", err)
		}
		if c.notes != "" {
			if _, err := q.UpsertMasterNotes(ctx, charactersdb.UpsertMasterNotesParams{CampaignID: in.CampaignID, CharacterID: c.id, Notes: c.notes, UpdatedAt: c.createdAt}); err != nil {
				return fmt.Errorf("save the master's notes: %w", err)
			}
		}
	}
	return nil
}
