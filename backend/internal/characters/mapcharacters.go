package characters

import (
	"context"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
)

// MapCharacters returns those of ids that may stand on one of the
// campaign's maps as tokens (MR-012): the campaign's living characters, a
// player's (neither dead nor waiting for approval) or an NPC. Players'
// characters come first, then NPCs, each group oldest first. IDs that are
// not UUIDs, or name any other character, are left out.
//
// Package maps calls it (its CharacterDirectory interface), after its own
// authorization check, and decides what each caller sees; so this method
// takes no caller, like LockSheets. Only id, kind, name and player_user_id
// are set: a token needs no sheet, and never the master's notes (RN-11).
func (s *Service) MapCharacters(ctx context.Context, campaignID string, ids []string) ([]*charactersv1.CharacterSummary, error) {
	valid := make([]string, 0, len(ids))
	for _, id := range ids {
		if id, ok := parseUUID(id); ok {
			valid = append(valid, id)
		}
	}
	if len(valid) == 0 {
		return nil, nil
	}
	rows, err := s.queries.ListMapCharacters(ctx, charactersdb.ListMapCharactersParams{CampaignID: campaignID, Ids: valid})
	if err != nil {
		return nil, s.dbError(ctx, "list the characters on a map", err)
	}
	out := make([]*charactersv1.CharacterSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, &charactersv1.CharacterSummary{
			Id:           row.ID,
			Kind:         kindFromDB[row.Kind],
			Name:         row.Name,
			PlayerUserId: deref(row.PlayerUserID),
		})
	}
	return out, nil
}
