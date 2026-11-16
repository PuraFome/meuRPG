package characters

import (
	"context"

	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
)

// The record store of an Artificer's infusions (character_infusions). The queries run on q, so a caller passes the
// queries of its transaction; the time is the service's clock.

// What an infusion is on (character_infusions.item_kind).
const (
	infusionItemGear   = "gear"
	infusionItemObject = "object"
)

// Why a record ended (character_infusions.ended_reason).
const (
	// infusionEndedEnded: the owner or the master ended it.
	infusionEndedEnded = "ended"
	// infusionEndedReplaced: it was over the limit and a new infusion ended it.
	infusionEndedReplaced = "replaced"
	// infusionEndedSwapped: it was given up at a level-up.
	infusionEndedSwapped = "swapped"
	// infusionEndedItemGone: the piece left the holder's sheet.
	infusionEndedItemGone = "item_gone"
	// infusionEndedTaken: another infusion took the item of a record that is not valid now.
	infusionEndedTaken = "taken"
)

// endInfusion ends a record that is live and says whether it was: an ended record is left as it is.
func (s *Service) endInfusion(ctx context.Context, q *charactersdb.Queries, id, reason string) (bool, error) {
	at := s.now()
	n, err := q.EndCharacterInfusion(ctx, charactersdb.EndCharacterInfusionParams{ID: id, Reason: &reason, At: &at})
	if err != nil {
		return false, wrap("end an infusion", err)
	}
	return n > 0, nil
}

// moveInfusion puts a live record on another item and starts its time again, and says whether it was live.
func (s *Service) moveInfusion(ctx context.Context, q *charactersdb.Queries, id, itemKind, itemKey, objectName string) (bool, error) {
	n, err := q.MoveCharacterInfusion(ctx, charactersdb.MoveCharacterInfusionParams{ID: id, ItemKind: itemKind, ItemKey: itemKey, ObjectName: objectName, At: s.now()})
	if err != nil {
		return false, wrap("move an infusion", err)
	}
	return n > 0, nil
}

// setInfusionAttuned says whether the holder is attuned to the item of a live record, and whether the record was live.
func (s *Service) setInfusionAttuned(ctx context.Context, q *charactersdb.Queries, id string, attuned bool) (bool, error) {
	n, err := q.SetCharacterInfusionAttuned(ctx, charactersdb.SetCharacterInfusionAttunedParams{ID: id, Attuned: attuned})
	if err != nil {
		return false, wrap("set the attunement of an infusion", err)
	}
	return n > 0, nil
}
