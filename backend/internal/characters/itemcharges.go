package characters

import (
	"context"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// Charges (#563, SRD 5.1): the maximum of an item comes from the catalog (rules.ItemCharges) and what a character
// spent from character_item_charges, one row for each entry of the sheet's magic items. A read fills
// charges_max, charges_left and destroyed on the entries; a save never changes them.

// fillItemCharges fills the charges of the character's magic items from the stored state. The caller is the player
// the character belongs to or the master (canSee), so the counts reach nobody else (RN-10).
func (s *Service) fillItemCharges(ctx context.Context, content *rules.Content, c *charactersv1.Character) error {
	full := c.GetSheet().GetFull()
	if len(full.GetMagicItems()) == 0 {
		return nil
	}
	rows, err := s.queries.ListItemCharges(ctx, c.GetId())
	if err != nil {
		return wrap("read item charges", err)
	}
	state := map[string]charactersdb.ListItemChargesRow{}
	for _, r := range rows {
		state[r.EntryID] = r
	}
	for _, it := range full.GetMagicItems() {
		ic, ok := content.ItemCharges(it.GetKey())
		if !ok {
			continue
		}
		r := state[it.GetId()]
		it.ChargesMax, it.Destroyed = ic.Max, r.Destroyed
		it.ChargesLeft = max(0, ic.Max-r.ChargesUsed)
		if r.Destroyed {
			it.ChargesLeft = 0
		}
	}
	return nil
}

// dropRemovedItemCharges deletes the spent charges of the entries a saved sheet no longer has, so an entry that
// takes a freed id starts full.
func dropRemovedItemCharges(ctx context.Context, q *charactersdb.Queries, characterID string, full *charactersv1.FullSheet) error {
	if full == nil {
		return nil
	}
	keep := make([]string, 0, len(full.GetMagicItems()))
	for _, it := range full.GetMagicItems() {
		keep = append(keep, it.GetId())
	}
	if err := q.DeleteItemChargesExcept(ctx, charactersdb.DeleteItemChargesExceptParams{CharacterID: characterID, KeepIds: keep}); err != nil {
		return wrap("drop the charges of removed items", err)
	}
	return nil
}
