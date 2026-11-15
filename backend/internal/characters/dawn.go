package characters

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"

	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The dawn (SRD 5.1, the text of each wand, staff, ring and figurine): a new day recharges the magic items of the
// campaign's characters. Package play decides who may ask and when, rolls the dice (RN-18) and writes the history;
// this file locks each character that has a stored row, works out what its items regain and writes it, all in the
// caller's transaction.

// dawnStep is what a dawn does to one stored row.
type dawnStep struct {
	used, dawns int32
	// regained is the charges the item gained; write says the row changes (the count of dawns may move with no gain).
	regained int32
	write    bool
}

// stepDawn works out what a dawn does to an entry, and rolls the item's dice when it regains some. An item that is
// lost, waits for its last d20, never recharges or is at its maximum is left as it is, and so is a rolled-start item
// whose count the master has not set (it has no row).
func stepDawn(ic rules.ItemCharges, r charactersdb.ListItemChargesRow, roll link.DawnRoll) (dawnStep, error) {
	keep := dawnStep{used: r.ChargesUsed, dawns: r.DawnsWaited}
	if r.Lost != "" || r.LastRollPending {
		return keep, nil
	}
	left := ic.Left(r.ChargesUsed)
	switch ic.Recharge {
	case rules.RechargeDawn:
		return stepDawnRegain(ic, keep, left, roll)
	case rules.RechargeDawnsAfterEmpty:
		return stepDawnWait(ic, keep, left), nil
	}
	return keep, nil
}

// stepDawnRegain is the dawn of an item that regains charges each dawn: all of them, or the dice, up to the maximum.
func stepDawnRegain(ic rules.ItemCharges, keep dawnStep, left int32, roll link.DawnRoll) (dawnStep, error) {
	room := ic.Max - left
	if room <= 0 {
		return keep, nil
	}
	regain := room
	if !ic.RegainAll {
		if ic.Regain.Count == 0 {
			return keep, nil
		}
		total, err := roll(ic.Regain.Count, ic.Regain.Sides, ic.Regain.Bonus)
		if err != nil {
			return keep, fmt.Errorf("roll the charges of the dawn: %w", err)
		}
		regain = min(max(total, 0), room)
	}
	if regain == 0 {
		return keep, nil
	}
	return dawnStep{used: room - regain, dawns: keep.dawns, regained: regain, write: true}, nil
}

// stepDawnWait is the dawn of an item that waits some dawns after its last charge and then regains them all: an item
// with a charge left counts no dawn (a count left over is cleared), an empty one counts this dawn and, on the last one
// of the wait, is full and starts over.
func stepDawnWait(ic rules.ItemCharges, keep dawnStep, left int32) dawnStep {
	if left > 0 {
		return dawnStep{used: keep.used, dawns: 0, write: keep.dawns != 0}
	}
	dawns := keep.dawns + 1
	if dawns >= ic.WaitDawns {
		return dawnStep{used: 0, dawns: 0, regained: ic.Max, write: true}
	}
	return dawnStep{used: keep.used, dawns: dawns, write: true}
}

// RestoreDawn is the dawn: every magic item of the campaign's characters that recharges does as its catalog entry
// says (see stepDawn), with the dice rolled by roll. It locks each character that has a stored row, in the order of
// their ids, and returns the items that regained charges in that order (an item that only counted a dawn is not
// among them). It writes nothing for an item at its maximum.
func (s *Service) RestoreDawn(ctx context.Context, tx pgx.Tx, campaignID string, roll link.DawnRoll) ([]link.DawnItem, error) {
	q := s.queries.WithTx(tx)
	refs, err := q.ListCampaignItemCharges(ctx, campaignID)
	if err != nil {
		return nil, wrap("list the item charges of the campaign", err)
	}
	content, err := s.contentFor(ctx, tx, campaignID)
	if err != nil {
		return nil, wrap("read rules content", err)
	}
	var characters []string
	for _, r := range refs {
		if !slices.Contains(characters, r.CharacterID) {
			characters = append(characters, r.CharacterID)
		}
	}
	var out []link.DawnItem
	for _, id := range characters {
		items, err := s.dawnOfCharacter(ctx, q, content, campaignID, id, roll)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	return out, nil
}

// dawnOfCharacter locks the character and gives its items their dawn.
func (s *Service) dawnOfCharacter(ctx context.Context, q *charactersdb.Queries, content *rules.Content, campaignID, characterID string, roll link.DawnRoll) ([]link.DawnItem, error) {
	row, err := q.GetCharacterForUpdate(ctx, charactersdb.GetCharacterForUpdateParams{CampaignID: campaignID, ID: characterID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, wrap("lock the character", err)
	}
	sheet, err := loadSheet(row.ID, row.Sheet)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListItemCharges(ctx, row.ID) // after the lock: a spend that ran first is counted
	if err != nil {
		return nil, wrap("read item charges", err)
	}
	var out []link.DawnItem
	for _, r := range rows {
		item, ok, err := s.dawnOfEntry(ctx, q, content, sheet, row.ID, r, roll)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, item)
		}
	}
	return out, nil
}

// dawnOfEntry gives one stored row its dawn and says whether the item regained charges (then it is the item after). A row
// of an entry the sheet no longer has, or of an item with no charges in the content, is left alone.
func (s *Service) dawnOfEntry(ctx context.Context, q *charactersdb.Queries, content *rules.Content, sheet *charactersv1.CharacterSheet, characterID string, r charactersdb.ListItemChargesRow, roll link.DawnRoll) (link.DawnItem, bool, error) {
	i := slices.IndexFunc(sheet.GetFull().GetMagicItems(), func(it *charactersv1.SheetMagicItem) bool { return it.GetId() == r.EntryID })
	if i < 0 {
		return link.DawnItem{}, false, nil
	}
	entry := sheet.GetFull().GetMagicItems()[i]
	ic, ok := content.ItemCharges(entry.GetKey())
	if !ok {
		return link.DawnItem{}, false, nil
	}
	step, err := stepDawn(ic, r, roll)
	if err != nil || !step.write {
		return link.DawnItem{}, false, err
	}
	c := chargedItem{characterID: characterID, entryID: r.EntryID, entry: entry, ic: ic, row: r, set: true}
	if err := c.write(ctx, q, s.now(), step.used, "", false, step.dawns); err != nil {
		return link.DawnItem{}, false, err
	}
	if step.regained == 0 {
		return link.DawnItem{}, false, nil
	}
	c.row.ChargesUsed, c.row.DawnsWaited = step.used, step.dawns
	return link.DawnItem{CharacterID: characterID, Item: c.view(), Regained: step.regained}, true, nil
}
