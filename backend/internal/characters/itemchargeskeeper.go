package characters

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/uid"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The keeper of the charges: what play asks of the characters module when an item spends its charges, rolls its last
// one or is corrected by the master. Each method runs in the caller's transaction, locks the character's row and
// never asks who the caller is: play authorizes first. They take and return proto values only.

// chargedItem is one entry of a character's sheet with its catalog numbers and stored row, read under the lock.
type chargedItem struct {
	characterID string
	entryID     string
	entry       *charactersv1.SheetMagicItem
	ic          rules.ItemCharges
	row         charactersdb.ListItemChargesRow
	set         bool
}

// view is the entry as a read gives it: with its charges, loss and pending roll.
func (c chargedItem) view() *charactersv1.SheetMagicItem {
	out := proto.CloneOf(c.entry)
	applyItemCharges(out, c.ic, c.row, c.set)
	return out
}

// lockChargedItem locks the character and finds the entry on its sheet: `not_found` for a character that is not in
// the campaign, an entry that is not on the sheet, and `invalid_argument` for an item that holds no charges.
func (s *Service) lockChargedItem(ctx context.Context, tx pgx.Tx, campaignID, characterID, entryID string) (chargedItem, *charactersdb.Queries, error) {
	id, ok := uid.Canonical(characterID)
	if !ok {
		return chargedItem{}, nil, errCharacterNotFound()
	}
	q := s.queries.WithTx(tx)
	row, err := q.GetCharacterForUpdate(ctx, charactersdb.GetCharacterForUpdateParams{CampaignID: campaignID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return chargedItem{}, nil, errCharacterNotFound()
	}
	if err != nil {
		return chargedItem{}, nil, wrap("lock the character", err)
	}
	sheet, err := loadSheet(row.ID, row.Sheet)
	if err != nil {
		return chargedItem{}, nil, err
	}
	var entry *charactersv1.SheetMagicItem
	for _, it := range sheet.GetFull().GetMagicItems() {
		if it.GetId() == entryID {
			entry = it
		}
	}
	if entry == nil {
		return chargedItem{}, nil, connect.NewError(connect.CodeNotFound, errors.New("the item is not on the sheet"))
	}
	content, err := s.contentFor(ctx, tx, campaignID)
	if err != nil {
		return chargedItem{}, nil, wrap("read rules content", err)
	}
	ic, ok := content.ItemCharges(entry.GetKey())
	if !ok {
		return chargedItem{}, nil, invalidArgument(fieldErr("entry_id", "is an item with no charges"))
	}
	rows, err := q.ListItemCharges(ctx, row.ID)
	if err != nil {
		return chargedItem{}, nil, wrap("read item charges", err)
	}
	c := chargedItem{characterID: row.ID, entryID: entryID, entry: entry, ic: ic}
	for _, r := range rows {
		if r.EntryID == entryID {
			c.row, c.set = r, true
		}
	}
	return c, q, nil
}

// write stores the state of the entry: used charges, the loss and the pending roll. A row that reads as a new item is
// deleted unless the item rolls its start (its row says the master set it).
func (c chargedItem) write(ctx context.Context, q *charactersdb.Queries, now time.Time, used int32, lost string, pending bool, dawns int32) error {
	if used == 0 && lost == "" && !pending && dawns == 0 && !c.ic.HasStart() {
		if err := q.DeleteItemCharges(ctx, charactersdb.DeleteItemChargesParams{CharacterID: c.characterID, EntryID: c.entryID}); err != nil {
			return wrap("delete item charges", err)
		}
		return nil
	}
	err := q.SetItemCharges(ctx, charactersdb.SetItemChargesParams{
		CharacterID: c.characterID, EntryID: c.entryID, ChargesUsed: used, Lost: lost, DawnsWaited: dawns, LastRollPending: pending, Now: now,
	})
	if err != nil {
		return wrap("write item charges", err)
	}
	return nil
}

// SpendItemCharges spends amount charges of an entry and returns the entry before and after. It refuses with
// CharacterBlocked ITEM_DESTROYED (the item was lost), ITEM_NO_CHARGES (none left, or the master has not set the
// count of an item that rolls it) and CHARGES_NOT_ENOUGH (more than left); `invalid_argument` for an amount below 1 or
// an item with no charges, `not_found` for a character or entry that is not there. Spending the last charge loses
// the item at once when the catalog says "always", and leaves the d20 to ResolveLastCharge when it says "d20".
func (s *Service) SpendItemCharges(ctx context.Context, tx pgx.Tx, campaignID, characterID, entryID string, amount int32) (before, after *charactersv1.SheetMagicItem, err error) {
	c, q, err := s.lockChargedItem(ctx, tx, campaignID, characterID, entryID)
	if err != nil {
		return nil, nil, err
	}
	before = c.view()
	switch {
	case amount < 1:
		return nil, nil, invalidArgument(fieldErr("amount", "must be at least 1"))
	case before.GetLost() != charactersv1.ItemLoss_ITEM_LOSS_UNSPECIFIED:
		return nil, nil, errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_ITEM_DESTROYED, c.characterID)
	case before.GetChargesLeft() <= 0:
		return nil, nil, errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_ITEM_NO_CHARGES, c.characterID)
	case amount > before.GetChargesLeft():
		return nil, nil, errBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CHARGES_NOT_ENOUGH, c.characterID)
	}
	left := before.GetChargesLeft() - amount
	lost, pending := "", false
	if left == 0 {
		switch c.ic.Empty {
		case rules.ChargesEmptyAlways:
			lost = c.ic.Lost
		case rules.ChargesEmptyD20:
			pending = true
		}
	}
	if err := c.write(ctx, q, s.now(), c.ic.Max-left, lost, pending, c.row.DawnsWaited); err != nil {
		return nil, nil, err
	}
	c.row.ChargesUsed, c.row.Lost, c.row.LastRollPending, c.set = c.ic.Max-left, lost, pending, true
	return before, c.view(), nil
}

// ResolveLastCharge settles the d20 of an item whose last charge went (see SpendItemCharges): a 1 loses the item as
// its catalog entry says (when it says), a 20 regains extra charges (the dice the caller rolled for the item's
// empty_high_regain, cut to the maximum) when the item has that rule, and any other face only ends the wait. It
// returns the charges regained. `invalid_argument` for a d20 out of 1..20 and for an item that waits for no roll.
func (s *Service) ResolveLastCharge(ctx context.Context, tx pgx.Tx, campaignID, characterID, entryID string, d20, extra int32) (before, after *charactersv1.SheetMagicItem, regained int32, err error) {
	c, q, err := s.lockChargedItem(ctx, tx, campaignID, characterID, entryID)
	if err != nil {
		return nil, nil, 0, err
	}
	before = c.view()
	if d20 < 1 || d20 > 20 {
		return nil, nil, 0, invalidArgument(fieldErr("d20", "must be 1 to 20"))
	}
	if !before.GetLastRollPending() {
		return nil, nil, 0, invalidArgument(fieldErr("entry_id", "has no last charge to roll"))
	}
	used, lost := c.row.ChargesUsed, ""
	switch {
	case d20 == 1 && c.ic.Lost != "":
		lost = c.ic.Lost
	case d20 == 20 && c.ic.EmptyHighRegain.Count > 0:
		regained = min(max(extra, 0), c.ic.Max)
		used = c.ic.Max - regained
	}
	if err := c.write(ctx, q, s.now(), used, lost, false, c.row.DawnsWaited); err != nil {
		return nil, nil, 0, err
	}
	c.row.ChargesUsed, c.row.Lost, c.row.LastRollPending, c.set = used, lost, false, true
	return before, c.view(), regained, nil
}

// SetItemCharges is the master's correction of an entry: left charges (0 to the maximum, which also sets the count of
// an item that rolls it) and how it was lost, ITEM_LOSS_UNSPECIFIED when it is not. It ends a pending roll.
// `invalid_argument` for a count out of range, an unknown loss and an item with no charges.
func (s *Service) SetItemCharges(ctx context.Context, tx pgx.Tx, campaignID, characterID, entryID string, left int32, lost charactersv1.ItemLoss) (before, after *charactersv1.SheetMagicItem, err error) {
	c, q, err := s.lockChargedItem(ctx, tx, campaignID, characterID, entryID)
	if err != nil {
		return nil, nil, err
	}
	before = c.view()
	if left < 0 || left > c.ic.Max {
		return nil, nil, invalidArgument(fieldErr("charges_left", "must be 0 to the item's maximum"))
	}
	stored, ok := itemLossStored(lost)
	if !ok {
		return nil, nil, invalidArgument(fieldErr("lost", "is not a way an item is lost"))
	}
	if err := c.write(ctx, q, s.now(), c.ic.Max-left, stored, false, 0); err != nil {
		return nil, nil, err
	}
	c.row.ChargesUsed, c.row.Lost, c.row.LastRollPending, c.set = c.ic.Max-left, stored, false, true
	return before, c.view(), nil
}

// itemLossStored is the text a row stores for a loss, and false for a value that is no loss.
func itemLossStored(l charactersv1.ItemLoss) (string, bool) {
	switch l {
	case charactersv1.ItemLoss_ITEM_LOSS_UNSPECIFIED:
		return "", true
	case charactersv1.ItemLoss_ITEM_LOSS_GONE:
		return rules.LostGone, true
	case charactersv1.ItemLoss_ITEM_LOSS_PLAIN_ITEM:
		return rules.LostPlain, true
	case charactersv1.ItemLoss_ITEM_LOSS_PARTLY_MAGIC:
		return rules.LostPartly, true
	}
	return "", false
}
