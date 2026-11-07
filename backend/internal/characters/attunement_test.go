package characters

import (
	"context"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
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

func (et *editorTable) attune(u *user, c *charactersv1.Character, key string, attuned bool) (*charactersv1.Character, error) {
	res, err := u.api.SetAttunement(context.Background(), connect.NewRequest(&charactersv1.SetAttunementRequest{
		CampaignId: et.campaign, CharacterId: c.GetId(), Key: key, Attuned: attuned,
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
	limit      = `failed_precondition: the character is already attuned to three items: end one first | CharacterBlocked{reason=CHARACTER_BLOCKED_REASON_ATTUNEMENT_LIMIT character_id="`
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
		if !it.GetRequiresAttunement() || it.GetAttunementByPt() == "" || it.GetRestrictionMet() {
			t.Errorf("%s: requires %v, by %q, met %v; the read says what the item asks and that the character does not meet it", it.GetKey(), it.GetRequiresAttunement(), it.GetAttunementByPt(), it.GetRestrictionMet())
		}
	}
}

// Another player never reaches the character; a dead character's owner is refused and the master still changes it.
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
	wantEditorRefusal(t, "the dead character's owner", err, `failed_precondition: the character is dead, so only the master edits its sheet | CharacterBlocked{reason=CHARACTER_BLOCKED_REASON_CHARACTER_DEAD character_id="`+pc.GetId()+`"}`)
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
	wantEditorRefusal(t, "an item not on the sheet", err, `invalid_argument: key is not an item of the sheet that asks for attunement | InvalidField{field="key"}`)
	_, err = et.attune(et.master, pc, "item:bag-of-holding", true)
	wantEditorRefusal(t, "an item that asks for none", err, `invalid_argument: key is not an item of the sheet that asks for attunement | InvalidField{field="key"}`)
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
		f.MagicItems = append(f.MagicItems, &charactersv1.SheetMagicItem{Key: ringB, Attuned: true, RestrictionMet: true})
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
