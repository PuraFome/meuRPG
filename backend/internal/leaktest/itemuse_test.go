package leaktest

import (
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1/charactersv1connect"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
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

// The ammunition a character shoots is as private as its charges: the line of the shot, the events, the count in the turn
// options and the pieces on the sheet are the master's and the owner's (RN-10). The needles are the words of those reads
// and the number of pieces left, a count nobody else's answer may hold.
var ammunitionWords = []string{`"ammunitionNamePt"`, `"ammunitionLeft"`, `"spentLastCombat"`, "COMBAT_LOG_KIND_AMMUNITION_SPENT", "ammunition_spent", "ammunition_recovered"}

const (
	arrows         = "equipment:arrow"
	arrowsHeld     = 4321
	arrowsAfterOne = arrowsHeld - 1
)

// torenShoots has Toren, on his turn next to a bandit, hold a shortbow and 4,321 arrows (the master's count), and returns the bandit.
func (w *world) torenShoots() *playv1.Combatant {
	w.t.Helper()
	ctx := w.t.Context()
	bandit := w.reactionFightOf("monster:bandit", w.secrets.public("ammunition-target"))
	w.placeForced(w.master, w.encounter.GetId(), placement{bandit.GetId(), 12, 12}) // 10 ft away: no disadvantage for a shot at an adjacent foe
	cur := must(w.master.characters.GetCharacter(ctx, rq(&charactersv1.GetCharacterRequest{CampaignId: w.campaign, CharacterId: w.toren.GetId()}))).GetCharacter()
	sheet := proto.CloneOf(cur.GetSheet())
	sheet.GetFull().WeaponKeys = append(sheet.GetFull().WeaponKeys, "equipment:shortbow")
	must(w.master.characters.UpdateCharacter(ctx, rq(&charactersv1.UpdateCharacterRequest{
		CampaignId: w.campaign, CharacterId: w.toren.GetId(), Revision: cur.GetRevision(), Name: cur.GetName(), Sheet: sheet,
	})))
	must(w.master.charges.SetAmmunition(ctx, rq(&playv1.SetAmmunitionRequest{CampaignId: w.campaign, CharacterId: w.toren.GetId(), AmmoKey: arrows, Quantity: arrowsHeld})))
	for _, n := range ammunitionWords {
		w.secrets.add(&canary{needle: n, kind: "ammunition"})
	}
	w.secrets.number("ammunition-left", arrowsAfterOne)
	return bandit
}

func (w *world) torenShot(target *playv1.Combatant) reply {
	w.t.Helper()
	return w.caio.call(playv1connect.CombatServiceRollAttackProcedure, &playv1.RollAttackRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), AttackerId: w.combatant(w.toren).GetId(), AttackKey: "equipment:shortbow", TargetId: target.GetId(),
		IdempotencyKey: newKey(), Roll: &playv1.RollAttackRequest_D20Face{D20Face: 12},
	})
}

// ammunitionReads are the answers that could hold the shot: the combat, the log, Toren's turn options and Toren's sheet.
func (w *world) ammunitionReads(p *person) []reply {
	w.t.Helper()
	return append(w.reads(p),
		w.turnOptions(p, w.combatant(w.toren)),
		p.call(charactersv1connect.CharacterServiceGetCharacterProcedure, &charactersv1.GetCharacterRequest{CampaignId: w.campaign, CharacterId: w.toren.GetId()}),
	)
}

func TestAnAmmunitionShotIsTheOwnersAndTheMastersAlone(t *testing.T) {
	w := newWorld(t)
	bandit := w.torenShoots()
	if shot := w.torenShot(bandit); !shot.ok() {
		t.Fatalf("Toren's shot: status %d %s", shot.status, shot.body)
	}

	for _, p := range []*person{w.master, w.caio} {
		entries := w.logOfPerson(p)
		if itemLines(entries) != 1 {
			t.Fatalf("%s reads %d item lines, want 1", p.name, itemLines(entries))
		}
		for _, e := range entries {
			if e.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_AMMUNITION_SPENT {
				if u := e.GetItemUse(); u.GetAmmunitionNamePt() == "" || u.GetLeft() != arrowsAfterOne || u.GetAmount() != 1 {
					t.Errorf("%s: the line is wrong: %v", p.name, u)
				}
			}
		}
		// The turn options carry the count, for the master and the owner.
		opts := w.turnOptions(p, w.combatant(w.toren))
		if !opts.ok() || !contains(opts.body, `"ammunitionLeft"`) {
			t.Errorf("%s: the turn options hold no ammunition count: status %d", p.name, opts.status)
		}
	}
	if got := itemLines(w.logOfPerson(w.ana)); got != 0 {
		t.Errorf("Ana reads %d item lines, want none", got)
	}
	if other := w.turnOptions(w.ana, w.combatant(w.toren)); other.ok() {
		t.Errorf("Ana reads Toren's turn options: %s", other.body)
	}
	for _, p := range []*person{w.ana, w.pending} {
		for _, r := range w.ammunitionReads(p) {
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
	}
	found := map[string]bool{}
	for _, r := range w.ammunitionReads(w.master) {
		for _, n := range ammunitionWords {
			found[n] = found[n] || contains(r.body, n)
		}
	}
	for _, n := range []string{`"ammunitionNamePt"`, `"ammunitionLeft"`, `"spentLastCombat"`, "COMBAT_LOG_KIND_AMMUNITION_SPENT"} {
		if !found[n] {
			t.Errorf("the master's reads hold no %s: the canary is unreachable", n)
		}
	}
}

// The hints of a shot, and of the recovery after the combat, carry none of the words to another player.
func TestAnAmmunitionShotAndItsRecoveryTellAnotherPlayerNothingOfIt(t *testing.T) {
	w := newWorld(t)
	bandit := w.torenShoots()
	got := w.hiddenOnlyEvents(func() {
		if shot := w.torenShot(bandit); !shot.ok() {
			t.Errorf("Toren's shot: status %d %s", shot.status, shot.body)
		}
		must(w.master.combat.EndEncounter(t.Context(), rq(&playv1.EndEncounterRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: newKey()})))
		must(w.caio.charges.RecoverAmmunition(t.Context(), rq(&playv1.RecoverAmmunitionRequest{CampaignId: w.campaign, CharacterId: w.toren.GetId(), IdempotencyKey: newKey()})))
	})
	vitals := func(name string) bool {
		return slices.ContainsFunc(got[name], func(h string) bool { return strings.HasPrefix(h, "vitals_changed") })
	}
	for _, name := range []string{w.master.name, w.caio.name} {
		if !vitals(name) {
			t.Errorf("%s hears %v, want a vitals_changed (the arrows left)", name, got[name])
		}
	}
	if vitals(w.ana.name) {
		t.Errorf("Ana hears %v, want no vitals_changed", got[w.ana.name])
	}
	for _, name := range []string{w.ana.name} {
		for _, ev := range got[name] {
			for _, n := range ammunitionWords {
				if strings.Contains(ev, n) {
					t.Errorf("%s hears %q: %s", name, n, ev)
				}
			}
		}
	}
}
