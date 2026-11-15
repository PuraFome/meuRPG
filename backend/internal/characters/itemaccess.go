package characters

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/platform/uid"
)

// ItemAccess tells package play what it needs to decide who may touch a character's charges and ammunition: the
// user who plays the character ("" for an NPC), whether the character is dead, and whether its player may still edit
// the sheet (a draft or a sheet that waits for approval). `not_found` for a character that is not in the campaign.
// It reads in tx and locks nothing.
func (s *Service) ItemAccess(ctx context.Context, tx pgx.Tx, campaignID, characterID string) (playerUserID string, dead, sheetEditable bool, err error) {
	id, ok := uid.Canonical(characterID)
	if !ok {
		return "", false, false, errCharacterNotFound()
	}
	row, err := s.queriesIn(tx).GetCharacter(ctx, charactersdb.GetCharacterParams{CampaignID: campaignID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, false, errCharacterNotFound()
	}
	if err != nil {
		return "", false, false, wrap("read the character", err)
	}
	state := characterState(row.Status, row.SheetLockedAt)
	return conv.OrZero(row.PlayerUserID), state == charactersv1.CharacterState_CHARACTER_STATE_DEAD, playerEditsSheet(state), nil
}

// ChargedItem reads an entry of a character's sheet as a read gives it (its charges, loss and pending roll), and locks
// the character until the transaction ends. `not_found` for a character or an entry that is not there,
// `invalid_argument` for an entry that holds no charges. Package play reads the item before it rolls the dice it needs.
func (s *Service) ChargedItem(ctx context.Context, tx pgx.Tx, campaignID, characterID, entryID string) (*charactersv1.SheetMagicItem, error) {
	c, _, err := s.lockChargedItem(ctx, tx, campaignID, characterID, entryID)
	if err != nil {
		return nil, err
	}
	return c.view(), nil
}
