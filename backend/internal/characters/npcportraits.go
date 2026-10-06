package characters

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
)

// NpcPortraits returns, for those of ids that are living NPCs of the campaign, the
// gallery image of their portrait ("" for an NPC without one), by character ID. IDs
// that are not UUIDs, or name anything else, are left out.
//
// Package maps calls it (its CharacterDirectory interface), after its own
// authorization check, when the master asks for a picture that shows NPCs
// (MR-039): the portrait goes to the image model as a character reference. Like
// MapCharacters it takes no caller, and the transaction when the caller has one.
func (s *Service) NpcPortraits(ctx context.Context, tx pgx.Tx, campaignID string, ids []string) (map[string]string, error) {
	valid := make([]string, 0, len(ids))
	for _, id := range ids {
		if id, ok := parseUUID(id); ok {
			valid = append(valid, id)
		}
	}
	out := map[string]string{}
	if len(valid) == 0 {
		return out, nil
	}
	rows, err := s.queriesIn(tx).ListNpcPortraits(ctx, charactersdb.ListNpcPortraitsParams{CampaignID: campaignID, Ids: valid})
	if err != nil {
		return nil, s.dbError(ctx, "read the portraits of NPCs", err)
	}
	for _, row := range rows {
		out[row.ID] = row.PortraitImageID
	}
	return out, nil
}
