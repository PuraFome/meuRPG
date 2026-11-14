package leaktest

import (
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A charge of a magic item spent in a combat leaves a line in the combat log with what was spent and what is left: the
// master's and the owner's (RN-10), as the gear list is. Another player reads no word of it, in the log, the stream's
// hints or any other read. The master's and the owner's reads are the positive controls.

var itemUseWords = []string{`"itemUse"`, "COMBAT_LOG_KIND_ITEM_CHARGE_SPENT", "COMBAT_LOG_KIND_ITEM_LAST_CHARGE_ROLLED"}

const chargedWand = "item:wand-of-magic-missiles"

// torenHasAWand puts a wand on Toren's sheet as the master and returns the entry's id.
func (w *world) torenHasAWand() string {
	w.t.Helper()
	ctx := w.t.Context()
	cur := must(w.master.characters.GetCharacter(ctx, rq(&charactersv1.GetCharacterRequest{CampaignId: w.campaign, CharacterId: w.toren.GetId()}))).GetCharacter()
	sheet := proto.CloneOf(cur.GetSheet())
	sheet.GetFull().MagicItems = append(sheet.GetFull().MagicItems, &charactersv1.SheetMagicItem{Key: chargedWand})
	saved := must(w.master.characters.UpdateCharacter(ctx, rq(&charactersv1.UpdateCharacterRequest{
		CampaignId: w.campaign, CharacterId: w.toren.GetId(), Revision: cur.GetRevision(), Name: cur.GetName(), Sheet: sheet,
	}))).GetCharacter()
	for _, it := range saved.GetSheet().GetFull().GetMagicItems() {
		if it.GetKey() == chargedWand {
			return it.GetId()
		}
	}
	w.t.Fatal("the wand is not on the sheet")
	return ""
}

func (w *world) spendTorensCharge(entry string) {
	w.t.Helper()
	must(w.caio.charges.SpendCharges(w.t.Context(), rq(&playv1.SpendChargesRequest{
		CampaignId: w.campaign, CharacterId: w.toren.GetId(), EntryId: entry, Amount: 1, IdempotencyKey: newKey(),
	})))
}

func itemLines(entries []*playv1.CombatLogEntry) int {
	n := 0
	for _, e := range entries {
		switch e.GetKind() {
		case playv1.CombatLogKind_COMBAT_LOG_KIND_ITEM_CHARGE_SPENT, playv1.CombatLogKind_COMBAT_LOG_KIND_ITEM_LAST_CHARGE_ROLLED, playv1.CombatLogKind_COMBAT_LOG_KIND_AMMUNITION_SPENT:
			n++
		default:
			if e.GetItemUse() != nil {
				n++
			}
		}
	}
	return n
}

// The line is in the master's and the owner's log, in no other player's, and nothing else a player reads holds a word of it.
func TestAnItemUseLineIsTheOwnersAndTheMastersAlone(t *testing.T) {
	w := newWorld(t)
	w.onTorenTurn()
	entry := w.torenHasAWand()
	for _, n := range itemUseWords {
		w.secrets.add(&canary{needle: n, kind: "item-use"})
	}
	w.spendTorensCharge(entry)

	for _, p := range []*person{w.master, w.caio} {
		entries := w.logOfPerson(p)
		if itemLines(entries) != 1 {
			t.Fatalf("%s reads %d item lines, want 1", p.name, itemLines(entries))
		}
		for _, e := range entries {
			if u := e.GetItemUse(); u != nil && (u.GetItemNamePt() == "" || u.GetLeft() != 6) {
				t.Errorf("%s: the line is wrong: %v", p.name, u)
			}
		}
	}
	if got := itemLines(w.logOfPerson(w.ana)); got != 0 {
		t.Errorf("Ana reads %d item lines, want none", got)
	}
	for _, p := range []*person{w.ana, w.pending} { // a pending member has no log at all: the reads are the check
		for _, r := range w.reads(p) {
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
	}
	found := false
	for _, r := range w.reads(w.master) {
		for _, n := range itemUseWords {
			found = found || contains(r.body, n)
		}
	}
	if !found {
		t.Error("the master's reads hold no needle: the canary is unreachable")
	}
}

// The hint that the log changed goes to the master and the owner; another player hears the encounter change (its
// revision moved) and nothing of the log.
func TestAnItemUseTellsTheLogToTheOwnerAndTheMasterAlone(t *testing.T) {
	w := newWorld(t)
	w.onTorenTurn()
	entry := w.torenHasAWand()
	got := w.hiddenOnlyEvents(func() { w.spendTorensCharge(entry) })
	heard := func(name string) bool {
		return slices.ContainsFunc(got[name], func(h string) bool { return strings.HasPrefix(h, "combat_log_changed") })
	}
	for _, name := range []string{w.master.name, w.caio.name} {
		if !heard(name) {
			t.Errorf("%s hears %v, want a combat_log_changed", name, got[name])
		}
	}
	if heard(w.ana.name) {
		t.Errorf("Ana hears %v, want no combat_log_changed", got[w.ana.name])
	}
}
