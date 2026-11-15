package characters

import (
	"context"
	"fmt"
	"slices"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// What a character's magic items spent and the pieces of ammunition it holds travel with the character in a package
// (character_item_charges and character_ammunition): the charges used, how an item was lost, the dawns an item that
// waits for them has counted, the d20 an item waits for, and the pieces held of each kind. What a combat fired and the
// combat it belongs to do not travel: a combat stays behind.

// packageItemState is what the export read for one character.
type packageItemState struct {
	charges []*pkgv1.PackageItemCharge
	ammo    []*pkgv1.PackageAmmunition
}

// itemStates reads the stored charges and ammunition of every character of the campaign, by character.
func itemStates(ctx context.Context, q *charactersdb.Queries, campaignID string) (map[string]*packageItemState, error) {
	out := map[string]*packageItemState{}
	of := func(id string) *packageItemState {
		if out[id] == nil {
			out[id] = &packageItemState{}
		}
		return out[id]
	}
	charges, err := q.ListCampaignItemCharges(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("list the item charges: %w", err)
	}
	for _, r := range charges {
		st := of(r.CharacterID)
		st.charges = append(st.charges, &pkgv1.PackageItemCharge{
			EntryId: r.EntryID, ChargesUsed: r.ChargesUsed, Lost: itemLossOf(r.Lost), DawnsWaited: r.DawnsWaited, LastRollPending: r.LastRollPending,
		})
	}
	ammo, err := q.ListCampaignAmmunition(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("list the ammunition: %w", err)
	}
	for _, r := range ammo {
		st := of(r.CharacterID)
		st.ammo = append(st.ammo, &pkgv1.PackageAmmunition{AmmoKey: r.AmmoKey, Quantity: r.Quantity})
	}
	return out, nil
}

// stagedItemState is the state of a character's items as it will be stored.
type stagedItemState struct {
	charges []charactersdb.SetItemChargesParams
	ammo    []charactersdb.SetAmmunitionQuantityParams
}

// stageItemState checks the state the package gives a character's items against the sheet and the rules content that
// come with it, as the calls that set it by hand do, and returns it ready to store. It says false for an entry that is
// not on the sheet or holds no charges, one named twice, a count out of the item's range, a loss that is none, a pending
// d20 on an item that does not wait for one or still has charges, a kind of ammunition the rules do not have, one named
// twice, and a quantity out of range.
func stageItemState(content *rules.Content, sheet *charactersv1.CharacterSheet, charges []*pkgv1.PackageItemCharge, ammo []*pkgv1.PackageAmmunition) (stagedItemState, bool) {
	var out stagedItemState
	keys := map[string]string{}
	for _, it := range sheet.GetFull().GetMagicItems() {
		keys[it.GetId()] = it.GetKey()
	}
	seen := map[string]bool{}
	for _, ch := range charges {
		row, ok := stageItemCharge(content, keys[ch.GetEntryId()], ch)
		if !ok || seen[ch.GetEntryId()] {
			return stagedItemState{}, false
		}
		seen[ch.GetEntryId()] = true
		out.charges = append(out.charges, row)
	}
	held := map[string]bool{}
	for _, a := range ammo {
		if !slices.Contains(content.AmmunitionKinds(), a.GetAmmoKey()) || a.GetQuantity() < 0 || a.GetQuantity() > MaxAmmunition || held[a.GetAmmoKey()] {
			return stagedItemState{}, false
		}
		held[a.GetAmmoKey()] = true
		out.ammo = append(out.ammo, charactersdb.SetAmmunitionQuantityParams{AmmoKey: a.GetAmmoKey(), Quantity: a.GetQuantity()})
	}
	return out, true
}

// stageItemCharge checks one item's state; key is the catalog key of the entry, "" for an entry the sheet lacks.
func stageItemCharge(content *rules.Content, key string, ch *pkgv1.PackageItemCharge) (charactersdb.SetItemChargesParams, bool) {
	ic, ok := content.ItemCharges(key)
	lost, known := itemLossStored(ch.GetLost())
	switch {
	case !ok || !known:
		return charactersdb.SetItemChargesParams{}, false
	case ch.GetChargesUsed() < 0 || ch.GetChargesUsed() > ic.Max:
		return charactersdb.SetItemChargesParams{}, false
	case ch.GetDawnsWaited() < 0 || ch.GetDawnsWaited() > ic.WaitDawns:
		return charactersdb.SetItemChargesParams{}, false
	case ch.GetDawnsWaited() > 0 && ch.GetChargesUsed() != ic.Max: // the dawns count only while the item is empty
		return charactersdb.SetItemChargesParams{}, false
	case ch.GetLastRollPending() && (ic.Empty != rules.ChargesEmptyD20 || ch.GetChargesUsed() != ic.Max || lost != ""):
		return charactersdb.SetItemChargesParams{}, false
	}
	return charactersdb.SetItemChargesParams{
		EntryID: ch.GetEntryId(), ChargesUsed: ch.GetChargesUsed(), Lost: lost, DawnsWaited: ch.GetDawnsWaited(), LastRollPending: ch.GetLastRollPending(),
	}, true
}

// applyItemState stores the state of a new character's items.
func applyItemState(ctx context.Context, q *charactersdb.Queries, c *stagedCharacter) error {
	for _, row := range c.items.charges {
		row.CharacterID, row.Now = c.id, c.createdAt
		if err := q.SetItemCharges(ctx, row); err != nil {
			return fmt.Errorf("store the charges of an imported item: %w", err)
		}
	}
	for _, row := range c.items.ammo {
		row.CharacterID, row.Now = c.id, c.createdAt
		if _, err := q.SetAmmunitionQuantity(ctx, row); err != nil {
			return fmt.Errorf("store the ammunition of an imported character: %w", err)
		}
	}
	return nil
}

// chargesOf and ammoOf are the state of a character that may have none (a nil state).
func (s *packageItemState) chargesOf() []*pkgv1.PackageItemCharge {
	if s == nil {
		return nil
	}
	return s.charges
}

func (s *packageItemState) ammoOf() []*pkgv1.PackageAmmunition {
	if s == nil {
		return nil
	}
	return s.ammo
}
