package characters

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
)

const (
	wandKey = "item:wand-of-magic-missiles"
	bagKey  = "item:bag-of-holding"
)

func (et *editorTable) setCharges(t *testing.T, characterID, entryID string, used int32, destroyed bool) {
	t.Helper()
	err := et.h.svc.queries.SetItemCharges(context.Background(), charactersdb.SetItemChargesParams{
		CharacterID: characterID, EntryID: entryID, ChargesUsed: used, Destroyed: destroyed, Now: et.h.clock.Now(),
	})
	if err != nil {
		t.Fatalf("SetItemCharges: %v", err)
	}
}

func (et *editorTable) read(t *testing.T, u *user, id string) *charactersv1.Character {
	t.Helper()
	res, err := u.api.GetCharacter(context.Background(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: et.campaign, CharacterId: id}))
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	return res.Msg.GetCharacter()
}

func magicItemOf(c *charactersv1.Character, key string) *charactersv1.SheetMagicItem {
	for _, it := range c.GetSheet().GetFull().GetMagicItems() {
		if it.GetKey() == key {
			return it
		}
	}
	return nil
}

// A wand reads its 7 charges full, what the character spent leaves the rest, and an item with no charges reads none.
func TestSheetReadsTheChargesOfItsMagicItems(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, wandKey, bagKey)
	wand := magicItemOf(pc, wandKey)
	if wand.GetChargesMax() != 7 || wand.GetChargesLeft() != 7 || wand.GetDestroyed() {
		t.Fatalf("a new wand: %+v", wand)
	}
	if bag := magicItemOf(pc, bagKey); bag.GetChargesMax() != 0 || bag.GetChargesLeft() != 0 {
		t.Errorf("a bag has charges: %+v", bag)
	}
	et.setCharges(t, pc.GetId(), wand.GetId(), 3, false)
	got := magicItemOf(et.read(t, et.master, pc.GetId()), wandKey)
	if got.GetChargesMax() != 7 || got.GetChargesLeft() != 4 {
		t.Errorf("after 3 spent: %+v", got)
	}
	et.setCharges(t, pc.GetId(), wand.GetId(), 7, true)
	got = magicItemOf(et.read(t, et.master, pc.GetId()), wandKey)
	if !got.GetDestroyed() || got.GetChargesLeft() != 0 {
		t.Errorf("a lost wand: %+v", got)
	}
}

// What the catalog does not say about the entry never leaves a number below 0: a row that spent more than the item
// holds (the catalog changed) reads as empty.
func TestSheetChargesNeverGoBelowZero(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, wandKey)
	et.setCharges(t, pc.GetId(), magicItemOf(pc, wandKey).GetId(), 99, false)
	if got := magicItemOf(et.read(t, et.master, pc.GetId()), wandKey); got.GetChargesLeft() != 0 {
		t.Errorf("left %d", got.GetChargesLeft())
	}
}

// A save never sets the charges: what the client sends is ignored, and the stored state stays.
func TestSaveIgnoresTheChargesTheClientSends(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, wandKey)
	id := magicItemOf(pc, wandKey).GetId()
	et.setCharges(t, pc.GetId(), id, 2, false)
	req := editorSaveOf(pc)
	req.Sheet = editorClone(pc.GetSheet(), func(f *charactersv1.FullSheet) {
		it := f.GetMagicItems()[0]
		it.ChargesMax, it.ChargesLeft, it.Destroyed = 99, 99, true
	})
	saved, err := editorUpdate(et.master, req)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := magicItemOf(saved, wandKey); got.GetChargesMax() != 7 || got.GetChargesLeft() != 5 || got.GetDestroyed() {
		t.Errorf("after the save: %+v", got)
	}
}

// Removing an entry drops what it spent: a new entry that takes the freed id starts full.
func TestSaveRemovingAnItemDropsItsSpentCharges(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, wandKey)
	id := magicItemOf(pc, wandKey).GetId()
	et.setCharges(t, pc.GetId(), id, 4, false)
	req := editorSaveOf(pc)
	req.Sheet = editorClone(pc.GetSheet(), func(f *charactersv1.FullSheet) { f.MagicItems = nil })
	removed, err := editorUpdate(et.master, req)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	rows, err := et.h.svc.queries.ListItemCharges(context.Background(), pc.GetId())
	if err != nil || len(rows) != 0 {
		t.Fatalf("rows after the removal: %v, %v", rows, err)
	}
	req = editorSaveOf(removed)
	req.Sheet = editorClone(removed.GetSheet(), func(f *charactersv1.FullSheet) {
		f.MagicItems = append(f.MagicItems, &charactersv1.SheetMagicItem{Key: wandKey})
	})
	again, err := editorUpdate(et.master, req)
	if err != nil {
		t.Fatalf("add again: %v", err)
	}
	if got := magicItemOf(again, wandKey); got.GetId() != id || got.GetChargesLeft() != 7 {
		t.Errorf("the new wand: %+v (old id %s)", got, id)
	}
}

// Another player never reads the charges: the character is not found for them.
func TestAnotherPlayerNeverReadsTheCharges(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, wandKey)
	et.setCharges(t, pc.GetId(), magicItemOf(pc, wandKey).GetId(), 1, false)
	_, err := et.caio.api.GetCharacter(context.Background(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: et.campaign, CharacterId: pc.GetId()}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("another player: %v", err)
	}
}
