package characters

import (
	"context"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

const (
	ringA = "item:ring-of-protection"
	ringB = "item:cloak-of-protection"
	ringC = "item:amulet-of-health"
	ringD = "item:ring-of-spell-storing"
)

// attuneTable is a table with Dona's fighter holding the given magic items, unattuned, saved by the master.
func attuneTable(t *testing.T, keys ...string) (*editorTable, *charactersv1.Character) {
	t.Helper()
	et := newEditorTable(t, nil)
	pc := et.editorStandardFighter(t)
	req := editorSaveOf(pc)
	req.Sheet = editorClone(pc.GetSheet(), func(f *charactersv1.FullSheet) {
		for _, k := range keys {
			f.MagicItems = append(f.MagicItems, &charactersv1.SheetMagicItem{Key: k})
		}
	})
	saved, err := editorUpdate(et.master, req)
	if err != nil {
		t.Fatalf("the master puts the items on the sheet: %v", err)
	}
	return et, saved
}

func (et *editorTable) openWindow(t *testing.T, id string, open bool) {
	t.Helper()
	if err := et.h.svc.queries.SetAttunementOpen(context.Background(), charactersdb.SetAttunementOpenParams{CharacterID: id, Open: open, Now: et.h.clock.Now()}); err != nil {
		t.Fatalf("SetAttunementOpen: %v", err)
	}
}

// attune asks for the entry that holds key: the first one not yet in the state asked, or the key itself as the entry id
// when the sheet has none.
func (et *editorTable) attune(u *user, c *charactersv1.Character, key string, attuned bool) (*charactersv1.Character, error) {
	id := key
	for _, it := range c.GetSheet().GetFull().GetMagicItems() {
		if it.GetKey() == key && it.GetAttuned() != attuned {
			id = it.GetId()
			break
		}
		if it.GetKey() == key {
			id = it.GetId()
		}
	}
	return et.attuneEntry(u, c, id, attuned)
}

func (et *editorTable) attuneEntry(u *user, c *charactersv1.Character, id string, attuned bool) (*charactersv1.Character, error) {
	res, err := u.api.SetAttunement(context.Background(), connect.NewRequest(&charactersv1.SetAttunementRequest{
		CampaignId: et.campaign, CharacterId: c.GetId(), EntryId: id, Attuned: attuned,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetCharacter(), nil
}

func attunedKeys(c *charactersv1.Character) []string {
	var out []string
	for _, it := range c.GetSheet().GetFull().GetMagicItems() {
		if it.GetAttuned() {
			out = append(out, it.GetKey())
		}
	}
	return out
}

const (
	needsRest  = `failed_precondition: attuning takes a rest: the owner attunes once after a short or long rest | CharacterBlocked{reason=CHARACTER_BLOCKED_REASON_ATTUNEMENT_NEEDS_REST character_id="`
	limit      = `failed_precondition: three items are already attuned: end one before attuning another | CharacterBlocked{reason=CHARACTER_BLOCKED_REASON_ATTUNEMENT_LIMIT character_id="`
	sameItem   = `failed_precondition: already attuned to another copy of the same item | CharacterBlocked{reason=CHARACTER_BLOCKED_REASON_ATTUNEMENT_SAME_ITEM character_id="`
	restricted = `failed_precondition: the item asks for something the character is not: only the master may attune it | CharacterBlocked{reason=CHARACTER_BLOCKED_REASON_ATTUNEMENT_RESTRICTED character_id="`
)

// The owner attunes only inside the window a rest opens, and one attunement spends it; the master needs no window.
func TestSetAttunementTheOwnerNeedsTheWindowAndOneAttunementSpendsIt(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, ringA, ringB)
	_, err := et.attune(et.dona, pc, ringA, true)
	wantEditorRefusal(t, "the owner without a window", err, needsRest+pc.GetId()+`"}`)
	et.openWindow(t, pc.GetId(), true)
	got, err := et.attune(et.dona, pc, ringA, true)
	if err != nil {
		t.Fatalf("the owner in the window: %v", err)
	}
	if k := attunedKeys(got); len(k) != 1 || k[0] != ringA {
		t.Errorf("attuned = %v, want [%s]", k, ringA)
	}
	_, err = et.attune(et.dona, got, ringB, true)
	wantEditorRefusal(t, "a second attunement in the same window", err, needsRest+pc.GetId()+`"}`)
	if got, err = et.attune(et.master, got, ringB, true); err != nil || len(attunedKeys(got)) != 2 {
		t.Errorf("the master needs no window: err %v, attuned %v", err, attunedKeys(got))
	}
}

// A character is attuned to at most three items: the fourth is refused to the owner and to the master alike.
func TestSetAttunementRefusesTheFourthItem(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, ringA, ringB, ringC, ringD)
	for _, k := range []string{ringA, ringB, ringC} {
		var err error
		if pc, err = et.attune(et.master, pc, k, true); err != nil {
			t.Fatalf("attune %s: %v", k, err)
		}
	}
	et.openWindow(t, pc.GetId(), true)
	_, err := et.attune(et.dona, pc, ringD, true)
	wantEditorRefusal(t, "the owner's fourth", err, limit+pc.GetId()+`"}`)
	_, err = et.attune(et.master, pc, ringD, true)
	wantEditorRefusal(t, "the master's fourth", err, limit+pc.GetId()+`"}`)
	if pc, err = et.attune(et.master, pc, ringA, false); err != nil {
		t.Fatalf("ending one: %v", err)
	}
	if _, err = et.attune(et.master, pc, ringD, true); err != nil {
		t.Errorf("a fourth after one ended: %v", err)
	}
}

// A restriction the character does not meet is the master's to override; the owner is refused, and so is the owner for a
// restriction the sheet cannot decide (an alignment left unset).
func TestSetAttunementRestrictionAndTheMastersOverride(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, "item:holy-avenger", "item:talisman-of-pure-good")
	et.openWindow(t, pc.GetId(), true)
	_, err := et.attune(et.dona, pc, "item:holy-avenger", true)
	wantEditorRefusal(t, "a fighter, a paladin's sword", err, restricted+pc.GetId()+`"}`)
	_, err = et.attune(et.dona, pc, "item:talisman-of-pure-good", true)
	wantEditorRefusal(t, "an alignment left unset", err, restricted+pc.GetId()+`"}`)
	if got, err := et.attune(et.master, pc, "item:holy-avenger", true); err != nil || len(attunedKeys(got)) != 1 {
		t.Errorf("the master's override: err %v, attuned %v", err, attunedKeys(got))
	}
	read := et.dona.get(t, et.campaign, pc.GetId())
	for _, it := range read.GetSheet().GetFull().GetMagicItems() {
		unmet := it.GetRestriction() == charactersv1.AttunementRestriction_ATTUNEMENT_RESTRICTION_NOT_MET || it.GetRestriction() == charactersv1.AttunementRestriction_ATTUNEMENT_RESTRICTION_UNKNOWN
		if !it.GetRequiresAttunement() || !strings.HasPrefix(it.GetRestrictionPt(), "Só ") || !unmet {
			t.Errorf("%s: requires %v, restriction %q (%v); the read says what the item asks and that the character does not meet it", it.GetKey(), it.GetRequiresAttunement(), it.GetRestrictionPt(), it.GetRestriction())
		}
	}
}

// Another player never reaches the character; a dead character's owner gets the window refusal, even with the window
// open, and the master still changes it.
func TestSetAttunementWhoMayAsk(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, ringA)
	_, err := et.attune(et.caio, pc, ringA, true)
	wantEditorRefusal(t, "another player", err, `not_found: character not found`)
	et.openWindow(t, pc.GetId(), true)
	if _, err = et.master.api.MarkCharacterDead(context.Background(), connect.NewRequest(&charactersv1.MarkCharacterDeadRequest{CampaignId: et.campaign, CharacterId: pc.GetId()})); err != nil {
		t.Fatalf("MarkCharacterDead: %v", err)
	}
	_, err = et.attune(et.dona, pc, ringA, true)
	wantEditorRefusal(t, "the dead character's owner", err, needsRest+pc.GetId()+`"}`)
	if _, err = et.attune(et.master, pc, ringA, true); err != nil {
		t.Errorf("the master on a dead character: %v", err)
	}
}

// A key that is no item of the sheet, or asks for no attunement, is refused; asking for the state it is in changes
// nothing and spends no window.
func TestSetAttunementKeyAndNoOp(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, ringA, "item:bag-of-holding")
	_, err := et.attune(et.master, pc, "item:ring-of-nothing", true)
	wantEditorRefusal(t, "an item not on the sheet", err, `invalid_argument: entry_id is not an entry of the sheet | InvalidField{field="entry_id"}`)
	_, err = et.attune(et.master, pc, "item:bag-of-holding", true)
	wantEditorRefusal(t, "an item that asks for none", err, `invalid_argument: entry_id is an item that does not ask for attunement | InvalidField{field="entry_id"}`)
	et.openWindow(t, pc.GetId(), true)
	if _, err = et.attune(et.dona, pc, ringA, false); err != nil {
		t.Fatalf("ending what is not attuned: %v", err)
	}
	if open, _ := et.h.svc.queries.GetAttunementOpen(context.Background(), pc.GetId()); !open {
		t.Error("a request for the state the item is in spent the window")
	}
}

// Two attunements racing on one window: exactly one is allowed, the other is told to rest.
func TestSetAttunementTwoRacingOnOneWindow(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, ringA, ringB)
	et.openWindow(t, pc.GetId(), true)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, k := range []string{ringA, ringB} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = et.attune(et.dona, pc, k, true)
		}()
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("%d attunements went through one window, want 1 (errors %v)", ok, errs)
	}
	if got := et.dona.get(t, et.campaign, pc.GetId()); len(attunedKeys(got)) != 1 {
		t.Errorf("attuned %v, want exactly one", attunedKeys(got))
	}
}

// A save never changes attunement: a new entry starts unattuned, a stored one keeps its state, and the fields the
// server fills are not kept.
func TestSaveNeverChangesAttunement(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, ringA)
	pc, err := et.attune(et.master, pc, ringA, true)
	if err != nil {
		t.Fatalf("attune: %v", err)
	}
	req := editorSaveOf(pc)
	req.Sheet = editorClone(pc.GetSheet(), func(f *charactersv1.FullSheet) {
		f.MagicItems[0].Attuned = false // the client says it ended
		f.MagicItems = append(f.MagicItems, &charactersv1.SheetMagicItem{Id: "m1", Key: ringB, Attuned: true, Restriction: charactersv1.AttunementRestriction_ATTUNEMENT_RESTRICTION_MET})
	})
	saved, err := editorUpdate(et.master, req)
	if err != nil {
		t.Fatalf("the save: %v", err)
	}
	got := map[string]bool{}
	for _, it := range saved.GetSheet().GetFull().GetMagicItems() {
		got[it.GetKey()] = it.GetAttuned()
	}
	if !got[ringA] || got[ringB] || len(got) != 2 {
		t.Errorf("after the save %v, want %s kept attuned and %s new and unattuned", got, ringA, ringB)
	}
	if proto.Equal(saved.GetSheet(), pc.GetSheet()) {
		t.Error("the save changed nothing")
	}
}

// The lock does not hold attunement: after the lock the owner still attunes in a window, while adding an item is the
// master's.
func TestAttunementIsNotBoundToTheSheetLock(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, ringA)
	et.h.lockSheets(et.campaign)
	et.openWindow(t, pc.GetId(), true)
	if _, err := et.attune(et.dona, pc, ringA, true); err != nil {
		t.Errorf("the owner attunes on a locked sheet: %v", err)
	}
	cur := et.dona.get(t, et.campaign, pc.GetId())
	req := editorSaveOf(cur)
	req.Sheet = editorClone(cur.GetSheet(), func(f *charactersv1.FullSheet) {
		f.MagicItems = append(f.MagicItems, &charactersv1.SheetMagicItem{Key: ringB})
	})
	_, err := editorUpdate(et.dona, req)
	wantEditorRefusal(t, "the owner adds an item on a locked sheet", err,
		`failed_precondition: the sheet is locked: a game session has started, so only the master edits it | CharacterBlocked{reason=CHARACTER_BLOCKED_REASON_SHEET_LOCKED character_id="`+pc.GetId()+`"}`)
	if _, err := editorUpdate(et.master, req); err != nil {
		t.Errorf("the master adds an item: %v", err)
	}
}

func entryIDs(c *charactersv1.Character) []string {
	var out []string
	for _, it := range c.GetSheet().GetFull().GetMagicItems() {
		out = append(out, it.GetId())
	}
	return out
}

// Two identical rings are two entries with their own ids; only one copy is attuned at a time, and the blocked copy is
// marked on the sheet.
func TestSetAttunementOnlyOneCopyOfAnItem(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, ringA, ringA)
	ids := entryIDs(pc)
	if len(ids) != 2 || ids[0] == "" || ids[0] == ids[1] {
		t.Fatalf("entry ids %v, want two distinct non-empty ids", ids)
	}
	pc, err := et.attuneEntry(et.master, pc, ids[0], true)
	if err != nil {
		t.Fatalf("the first copy: %v", err)
	}
	_, err = et.attuneEntry(et.master, pc, ids[1], true)
	wantEditorRefusal(t, "the second copy", err, sameItem+pc.GetId()+`"}`)
	items := pc.GetSheet().GetFull().GetMagicItems()
	if items[0].GetSameItemAttuned() || !items[1].GetSameItemAttuned() {
		t.Errorf("same_item_attuned = %v, %v; want false (the attuned copy) and true (the blocked one)", items[0].GetSameItemAttuned(), items[1].GetSameItemAttuned())
	}
	if pc, err = et.attuneEntry(et.master, pc, ids[0], false); err != nil {
		t.Fatalf("ending the first copy: %v", err)
	}
	if _, err = et.attuneEntry(et.master, pc, ids[1], true); err != nil {
		t.Errorf("the second copy after the first ended: %v", err)
	}
}

// The checks run in the order stale revision, window, limit, one copy, restriction: the owner who lost the race to the
// master gets the limit, and the one who lost the window to their own other device gets the window.
func TestSetAttunementRefusalOrder(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, ringA, ringB, ringC, ringD, ringA)
	for _, k := range []string{ringA, ringB, ringC} {
		var err error
		if pc, err = et.attune(et.master, pc, k, true); err != nil {
			t.Fatalf("attune %s: %v", k, err)
		}
	}
	ids := entryIDs(pc)
	// A fourth that is also a second copy and has a restriction: the limit is the first refusal after the window.
	_, err := et.attuneEntry(et.dona, pc, ids[4], true)
	wantEditorRefusal(t, "no window comes before the limit", err, needsRest+pc.GetId()+`"}`)
	et.openWindow(t, pc.GetId(), true)
	_, err = et.attuneEntry(et.dona, pc, ids[4], true)
	wantEditorRefusal(t, "the owner lost the race to the master", err, limit+pc.GetId()+`"}`)
	if pc, err = et.attune(et.master, pc, ringC, false); err != nil {
		t.Fatalf("ending one: %v", err)
	}
	_, err = et.attuneEntry(et.dona, pc, ids[4], true)
	wantEditorRefusal(t, "the copy comes before the restriction", err, sameItem+pc.GetId()+`"}`)
	// The owner's own other device spent the window first.
	if _, err = et.attuneEntry(et.dona, pc, ids[3], true); err != nil {
		t.Fatalf("the owner's first attunement: %v", err)
	}
	_, err = et.attuneEntry(et.dona, pc, ids[2], true)
	wantEditorRefusal(t, "the owner lost the window to their other device", err, needsRest+pc.GetId()+`"}`)
}

// An attuned item can be removed from the sheet: the attunement ends with it and the counter drops.
func TestRemovingAnAttunedItemEndsItsAttunement(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, ringA, ringB)
	for _, k := range []string{ringA, ringB} {
		var err error
		if pc, err = et.attune(et.master, pc, k, true); err != nil {
			t.Fatalf("attune %s: %v", k, err)
		}
	}
	req := editorSaveOf(pc)
	req.Sheet = editorClone(pc.GetSheet(), func(f *charactersv1.FullSheet) { f.MagicItems = f.MagicItems[1:] })
	saved, err := editorUpdate(et.master, req)
	if err != nil {
		t.Fatalf("remove an attuned item: %v", err)
	}
	if k := attunedKeys(saved); len(k) != 1 || k[0] != ringB {
		t.Errorf("attuned %v, want only %s", k, ringB)
	}
	// A new entry takes a new id, never the removed one.
	req = editorSaveOf(saved)
	req.Sheet = editorClone(saved.GetSheet(), func(f *charactersv1.FullSheet) {
		f.MagicItems = append(f.MagicItems, &charactersv1.SheetMagicItem{Key: ringC, Id: pc.GetSheet().GetFull().GetMagicItems()[1].GetId()})
	})
	again, err := editorUpdate(et.master, req)
	if err != nil {
		t.Fatalf("add an item: %v", err)
	}
	ids := entryIDs(again)
	if len(ids) != 2 || ids[0] == ids[1] || again.GetSheet().GetFull().GetMagicItems()[1].GetAttuned() {
		t.Errorf("ids %v, attuned %v: a client's repeated id must not give the new entry the old one's attunement", ids, attunedKeys(again))
	}
}

// A +N piece names its catalog item: the server takes requires_attunement from the catalog and ignores the client's flag,
// the item must fit the piece, and the piece is then one attunable entry that counts toward the three.
func TestAPlusNPieceNamesItsCatalogItem(t *testing.T) {
	t.Parallel()
	et := newEditorTable(t, nil)
	pc := et.editorStandardFighter(t)
	with := func(ib *charactersv1.ItemBonus) *charactersv1.UpdateCharacterRequest {
		req := editorSaveOf(pc)
		req.Sheet = editorClone(pc.GetSheet(), func(f *charactersv1.FullSheet) {
			f.WeaponKeys = []string{"equipment:longsword"}
			f.ItemBonuses = []*charactersv1.ItemBonus{ib}
		})
		return req
	}
	_, err := editorUpdate(et.master, with(&charactersv1.ItemBonus{Key: "equipment:longsword", Bonus: 3, ItemKey: "item:armor-of-resistance"}))
	wantEditorRefusal(t, "an armor item on a weapon", err, `invalid_argument: sheet.full.item_bonuses[0].item_key is not a catalog item (no family) that fits the piece: armor for the armor and the shield, a weapon for a weapon | InvalidField{field="sheet.full.item_bonuses[0].item_key"}`)
	_, err = editorUpdate(et.master, with(&charactersv1.ItemBonus{Key: "equipment:longsword", Bonus: 3, ItemKey: "item:weapon"}))
	wantEditorRefusal(t, "a family", err, `invalid_argument: sheet.full.item_bonuses[0].item_key is not a catalog item (no family) that fits the piece: armor for the armor and the shield, a weapon for a weapon | InvalidField{field="sheet.full.item_bonuses[0].item_key"}`)
	saved, err := editorUpdate(et.master, with(&charactersv1.ItemBonus{Key: "equipment:longsword", Bonus: 3, ItemKey: "item:defender", RequiresAttunement: false, Attuned: true}))
	if err != nil {
		t.Fatalf("the Defender: %v", err)
	}
	ib := saved.GetSheet().GetFull().GetItemBonuses()[0]
	if !ib.GetRequiresAttunement() || ib.GetAttuned() || ib.GetId() != "bonus:equipment:longsword" || ib.GetItemNamePt() == "" {
		t.Errorf("the Defender's entry = %v, want it asking for attunement, unattuned, named, with the id bonus:equipment:longsword", ib)
	}
	if got, err := et.attuneEntry(et.master, saved, ib.GetId(), true); err != nil || !got.GetSheet().GetFull().GetItemBonuses()[0].GetAttuned() {
		t.Errorf("attune the Defender: err %v", err)
	}
}

// ListMagicItems pages the picker and ties a token to its search.
func TestListMagicItemsPagesAndRefusesAForeignToken(t *testing.T) {
	t.Parallel()
	et := newEditorTable(t, nil)
	ctx := context.Background()
	list := func(q string, size int32, token string) (*rulesv1.ListMagicItemsResponse, error) {
		res, err := et.dona.content.ListMagicItems(ctx, connect.NewRequest(&rulesv1.ListMagicItemsRequest{CampaignId: et.campaign, Query: q, PageSize: size, PageToken: token}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	first, err := list("", 10, "")
	if err != nil || len(first.GetItems()) != 10 || first.GetNextPageToken() == "" || first.GetTotal() < 100 {
		t.Fatalf("first page: %v, err %v", first, err)
	}
	next, err := list("", 10, first.GetNextPageToken())
	if err != nil || next.GetItems()[0].GetKey() == first.GetItems()[0].GetKey() {
		t.Fatalf("second page: %v, err %v", next, err)
	}
	_, err = list("anel", 10, first.GetNextPageToken())
	wantEditorRefusal(t, "a token of another search", err, `invalid_argument: page_token is not a token of this list | InvalidField{field="page_token"}`)
	_, err = list(strings.Repeat("a", 101), 10, "")
	wantEditorRefusal(t, "a long query", err, `invalid_argument: query must have at most 100 characters | InvalidField{field="query"}`)
	_, err = list("", 401, "")
	wantEditorRefusal(t, "a page too long", err, `invalid_argument: page_size must be 1 to 400 | InvalidField{field="page_size"}`)
}

// A +N piece stored before the entry ids reads with its id, so the owner can name it.
func TestAReadGivesALegacyPieceItsID(t *testing.T) {
	t.Parallel()
	f := &charactersv1.FullSheet{ItemBonuses: []*charactersv1.ItemBonus{{Key: "equipment:longsword", Bonus: 1, RequiresAttunement: true}}}
	fillMagicItems(nil, f, rules.Build{}, rules.Derived{})
	if got := f.GetItemBonuses()[0].GetId(); got != "bonus:equipment:longsword" {
		t.Errorf("id = %q", got)
	}
}
