package characters

import (
	"context"
	"uuid"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
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

// newInfusion is what insertInfusion writes. GearKey is the gear key of the holder's sheet; an object ignores it, since
// its key is made from its own id.
type newInfusion struct {
	CampaignID, ArtificerID, HolderID, InfusionKey string
	ItemKind, GearKey, ObjectName, Parameter       string
	Attuned                                        bool
}

// objectItemKey is the item key of an object record: its own id, so two objects never collide.
func objectItemKey(id string) string { return "object:" + id }

// insertInfusion writes a live record that begins now. The id is made here, so an object's key can hold it.
func (s *Service) insertInfusion(ctx context.Context, q *charactersdb.Queries, n newInfusion) (charactersdb.CharacterInfusion, error) {
	id := uuid.New().String()
	itemKey := n.GearKey
	if n.ItemKind == infusionItemObject {
		itemKey = objectItemKey(id)
	}
	at := s.now()
	rec, err := q.InsertCharacterInfusion(ctx, charactersdb.InsertCharacterInfusionParams{
		ID: id, CampaignID: n.CampaignID, ArtificerCharacterID: n.ArtificerID, HolderCharacterID: n.HolderID,
		InfusionKey: n.InfusionKey, ItemKind: n.ItemKind, ItemKey: itemKey, ObjectName: n.ObjectName, Parameter: n.Parameter,
		Attuned: n.Attuned, Since: at, CreatedAt: at,
	})
	if err != nil {
		return charactersdb.CharacterInfusion{}, wrap("insert an infusion", err)
	}
	return rec, nil
}

// endInfusion ends a record of the campaign that is live and says whether it was: an ended record is left as it is.
func (s *Service) endInfusion(ctx context.Context, q *charactersdb.Queries, campaignID, id, reason string) (bool, error) {
	n, err := q.EndCharacterInfusion(ctx, charactersdb.EndCharacterInfusionParams{CampaignID: campaignID, ID: id, Reason: reason, At: s.now()})
	if err != nil {
		return false, wrap("end an infusion", err)
	}
	return n > 0, nil
}

// moveInfusion puts a live record on another item, starts its time again and clears its attunement (the item changed), and
// says whether it was live. A move to an object keeps the key made from the record's own id; a move to gear sets gearKey.
func (s *Service) moveInfusion(ctx context.Context, q *charactersdb.Queries, campaignID, id, itemKind, gearKey, objectName string) (bool, error) {
	itemKey := gearKey
	if itemKind == infusionItemObject {
		itemKey = objectItemKey(id)
	}
	n, err := q.MoveCharacterInfusion(ctx, charactersdb.MoveCharacterInfusionParams{CampaignID: campaignID, ID: id, ItemKind: itemKind, ItemKey: itemKey, ObjectName: objectName, At: s.now()})
	if err != nil {
		return false, wrap("move an infusion", err)
	}
	return n > 0, nil
}

// setInfusionAttuned says whether the holder is attuned to the item of a live record of the campaign, and whether the record was live.
func (s *Service) setInfusionAttuned(ctx context.Context, q *charactersdb.Queries, campaignID, id string, attuned bool) (bool, error) {
	n, err := q.SetCharacterInfusionAttuned(ctx, charactersdb.SetCharacterInfusionAttunedParams{CampaignID: campaignID, ID: id, Attuned: attuned})
	if err != nil {
		return false, wrap("set the attunement of an infusion", err)
	}
	return n > 0, nil
}

// endInfusionsOfGoneGear ends, with item_gone, the live gear records of the holder whose piece the sheet no longer lists:
// the armor, the shield or a weapon taken off the sheet. A piece moved between equipped and carried is still listed, so
// its record stays. It runs in the transaction of the write that changed the sheet, after that write locked the holder's
// row. A sheet that is not a full one lists no gear and has no records to end.
func (s *Service) endInfusionsOfGoneGear(ctx context.Context, q *charactersdb.Queries, campaignID, holderID string, next *charactersv1.FullSheet) error {
	if next == nil {
		return nil
	}
	live, err := q.ListLiveInfusionsOfHolder(ctx, holderID)
	if err != nil {
		return wrap("read the infusions of a holder", err)
	}
	build := buildOf(next)
	for _, r := range live {
		if r.ItemKind != infusionItemGear || build.ListsGear(r.ItemKey) {
			continue
		}
		if _, err := s.endInfusion(ctx, q, campaignID, r.ID, infusionEndedItemGone); err != nil {
			return err
		}
	}
	return nil
}
