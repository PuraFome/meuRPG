package play

import (
	"slices"
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// TestSpentOfReadsWhatAnNPCFightSpent: the count on the combatant (slot levels, "pact" and
// "res:<key>") reaches the options as the usage the rules read; a player's character is
// counted in its vitals and gives nothing here.
func TestSpentOfReadsWhatAnNPCFightSpent(t *testing.T) {
	t.Parallel()
	body := []byte(`{"1":2,"3":1,"pact":1,"res:innate_3_day_bless":2,"10":4,"x":1}`)
	got := spentOf(playdb.Combatant{Kind: kindNPC, SlotsUsed: body})
	want := link.Spent{SlotsUsed: [9]int{2, 0, 1}, PactSlotsUsed: 1, ResourcesUsed: map[string]int{"innate_3_day_bless": 2}}
	if got.SlotsUsed != want.SlotsUsed || got.PactSlotsUsed != want.PactSlotsUsed || len(got.ResourcesUsed) != 1 || got.ResourcesUsed["innate_3_day_bless"] != 2 {
		t.Errorf("spentOf(npc) = %+v, want %+v", got, want)
	}
	if p := spentOf(playdb.Combatant{Kind: kindPlayer, SlotsUsed: body}); p.SlotsUsed != [9]int{} || p.PactSlotsUsed != 0 || p.ResourcesUsed != nil {
		t.Errorf("spentOf(player) = %+v, want nothing: its vitals count", p)
	}
	if e := spentOf(playdb.Combatant{Kind: kindNPC}); e.SlotsUsed != [9]int{} || e.ResourcesUsed != nil {
		t.Errorf("spentOf(no count) = %+v, want nothing", e)
	}
}

// usedOf reads the count an NPC's fight spent, as stored.
func (a *armed) usedOf(t *testing.T, label string) string {
	t.Helper()
	var used string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT slots_used::TEXT FROM combatants WHERE id = $1`, a.id(t, label)).Scan(&used); err != nil {
		t.Fatalf("slots_used of %s: %v", label, err)
	}
	return used
}

// TestAnNPCMagesCastSpendsASlotAndTheUndoGivesItBack: the Mage casts Mísseis Mágicos with a
// 1st-level slot as the master; the count on the combatant takes it, and the undo
// gives it back. Needs the database.
func TestAnNPCMagesCastSpendsASlotAndTheUndoGivesItBack(t *testing.T) {
	t.Parallel()
	a, e := mageFight(t)
	e = a.passTo(t, e, "Mago")
	a.mustCast(t, a.master, e, "Mago", magicMissileSpell, slotOfLevel(1), []*playv1.SpellTarget{darts(a, t, "Toren", 3)}, noCastRoll)
	if used := a.usedOf(t, "Mago"); !strings.Contains(used, `"1": 1`) {
		t.Fatalf("the Mage's count after the cast = %s, want one 1st-level slot", used)
	}
	a.undoLast(t, e)
	if used := a.usedOf(t, "Mago"); used != `{"1": 0}` {
		t.Errorf("the Mage's count after the undo = %s, want the level kept with no use", used)
	}
}

// TestTwoChangesToOneNPCsCountInOneTransactionBothLand: a slot and a resource use are
// counted on the Mage through the combatant a caller loaded before either; both stay,
// where a count written back whole would keep only the last. Needs the database.
func TestTwoChangesToOneNPCsCountInOneTransactionBothLand(t *testing.T) {
	t.Parallel()
	a, e := mageFight(t)
	cs, err := a.h.svc.queries.ListCombatants(t.Context(), e.GetId())
	if err != nil {
		t.Fatalf("ListCombatants() error = %v", err)
	}
	mago := cs[slices.IndexFunc(cs, func(c playdb.Combatant) bool { return c.Label == "Mago" })]
	tx, err := a.h.pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	defer func() { _ = tx.Rollback(t.Context()) }()
	c := &combatTx{tx: tx, q: a.h.svc.queries.WithTx(tx)}
	if err := a.h.svc.bumpUsed(t.Context(), c, mago, "1", 1); err != nil {
		t.Fatalf("bumpUsed(slot) error = %v", err)
	}
	if err := a.h.svc.bumpUsed(t.Context(), c, mago, usedResource+"innate_3_day_bless", 1); err != nil { // the same stale copy of the Mage
		t.Fatalf("bumpUsed(resource) error = %v", err)
	}
	var used string
	if err := tx.QueryRow(t.Context(), `SELECT slots_used::TEXT FROM combatants WHERE id = $1`, mago.ID).Scan(&used); err != nil {
		t.Fatalf("slots_used: %v", err)
	}
	if !strings.Contains(used, `"1": 1`) || !strings.Contains(used, `"res:innate_3_day_bless": 1`) {
		t.Errorf("the count after two changes = %s, want both", used)
	}
}

// freeSlotsOf reads the free slots of a level a spell is offered with.
func freeSlotsOf(res *playv1.GetTurnOptionsResponse, spell string, level int32) (free, total int32, ok bool) {
	for _, sp := range res.GetOptions().GetSpells() {
		if sp.GetSpell().GetKey() != spell {
			continue
		}
		for _, s := range sp.GetSlots() {
			if s.GetLevel() == level {
				return s.GetFree(), s.GetTotal(), true
			}
		}
	}
	return 0, 0, false
}

// TestAnNPCMagesSlotsLeftAreTheMastersAlone (RN-20, RN-29): the master's options for the Mage
// show its 1st-level slots, one less after a cast, and the options the cast sheet already
// carries need no screen change; a player asking for the same options is refused, so the
// slots, the spells and the uses left never reach the table. Needs the database.
func TestAnNPCMagesSlotsLeftAreTheMastersAlone(t *testing.T) {
	t.Parallel()
	a, e := mageFight(t)
	e = a.passTo(t, e, "Mago")
	before := a.mustOptions(t, a.master, e, "Mago")
	free, total, ok := freeSlotsOf(before, magicMissileSpell, 1) // positive control: the master reads them
	if !ok || total != 4 || free != 4 {
		t.Fatalf("the master reads %d of %d 1st-level slots (offered %v), want 4 of 4", free, total, ok)
	}
	a.mustCast(t, a.master, e, "Mago", magicMissileSpell, slotOfLevel(1), []*playv1.SpellTarget{darts(a, t, "Toren", 3)}, noCastRoll)
	after := a.mustOptions(t, a.master, a.get(t, a.master), "Mago")
	if free, _, _ := freeSlotsOf(after, magicMissileSpell, 1); free != 3 {
		t.Errorf("the master reads %d free 1st-level slots after a cast, want 3", free)
	}
	for _, player := range []*user{a.caio, a.ana} {
		res, err := a.options(t, player, a.get(t, a.master), "Mago")
		if err == nil {
			t.Errorf("a player reads the Mage's options = %v, want them refused", res)
		}
	}
}

// grantOf reads the grant a spell is offered with.
func grantOf(res *playv1.GetTurnOptionsResponse, spell string) *rulesv1.SpellGrantOption {
	for _, sp := range res.GetOptions().GetSpells() {
		if sp.GetSpell().GetKey() == spell && sp.GetGrant() != nil {
			return sp.GetGrant()
		}
	}
	return nil
}

// TestAnNPCsInnateSpellsCountTheirUses: the Couatl casts Bless 3/day with the stat block's
// numbers; the use is counted on the combatant, the master's options show one less, the undo gives
// it back. Needs the database.
func TestAnNPCsInnateSpellsCountTheirUses(t *testing.T) {
	t.Parallel()
	a, e := monsterFight(t, "monster:couatl", "Couatl")
	e = a.passTo(t, e, "Couatl")
	first := grantOf(a.mustOptions(t, a.master, e, "Couatl"), "spell:bless")
	if first == nil || first.GetUsesLeft() != 3 { // positive control: the master reads the uses
		t.Fatalf("the master reads the Bless grant = %v, want 3 uses left", first)
	}
	byGrant := func(r *playv1.CastSpellRequest) { r.GrantedBy = first.GetSourceKey() }
	a.mustCast(t, a.master, e, "Couatl", "spell:bless", nil, a.at(t, "Toren"), byGrant)
	if used := a.usedOf(t, "Couatl"); used != `{"res:innate_3_day_bless": 1}` {
		t.Fatalf("the Couatl's count after Bless = %s, want one use of the resource", used)
	}
	if g := grantOf(a.mustOptions(t, a.master, a.get(t, a.master), "Couatl"), "spell:bless"); g.GetUsesLeft() != 2 {
		t.Errorf("the master reads %d uses left after Bless, want 2", g.GetUsesLeft())
	}
	a.undoLast(t, a.get(t, a.master))
	if used := a.usedOf(t, "Couatl"); used != `{"res:innate_3_day_bless": 0}` {
		t.Errorf("the Couatl's count after the undo = %s, want the use back", used)
	}
}

// TestAnNPCsAtWillSpellSpendsNothingAndItsUndoGivesNothingBack: the Couatl casts Bless (one
// use counted), then Detect Magic at will; the count stays as it was, with no new key, and
// the undo of the at-will cast leaves the one use of Bless where it is. Needs the database.
func TestAnNPCsAtWillSpellSpendsNothingAndItsUndoGivesNothingBack(t *testing.T) {
	t.Parallel()
	a, e := monsterFight(t, "monster:couatl", "Couatl")
	e = a.passTo(t, e, "Couatl")
	bless := grantOf(a.mustOptions(t, a.master, e, "Couatl"), "spell:bless")
	a.mustCast(t, a.master, e, "Couatl", "spell:bless", nil, a.at(t, "Toren"), func(r *playv1.CastSpellRequest) { r.GrantedBy = bless.GetSourceKey() })
	const counted = `{"res:innate_3_day_bless": 1}`
	if used := a.usedOf(t, "Couatl"); used != counted {
		t.Fatalf("the Couatl's count after Bless = %s, want %s", used, counted)
	}
	e = a.passTo(t, a.mustEndTurn(t, a.master, e), "Couatl")
	detect := grantOf(a.mustOptions(t, a.master, e, "Couatl"), "spell:detect-magic")
	if detect == nil || !detect.GetAtWill() { // positive control: the master reads it at will
		t.Fatalf("the master reads the Detect Magic grant = %v, want at will", detect)
	}
	a.mustCast(t, a.master, e, "Couatl", "spell:detect-magic", nil, nil, func(r *playv1.CastSpellRequest) { r.GrantedBy = detect.GetSourceKey() })
	if used := a.usedOf(t, "Couatl"); used != counted {
		t.Errorf("the Couatl's count after Detect Magic at will = %s, want %s", used, counted)
	}
	a.undoLast(t, a.get(t, a.master))
	if used := a.usedOf(t, "Couatl"); used != counted {
		t.Errorf("the Couatl's count after the undo of Detect Magic = %s, want %s", used, counted)
	}
}

// TestUndoingAnNPCCastRecordedBeforeSlotsWereCounted: an NPC's cast of the time its slot was
// not counted stores the slot all the same; undoing it must not take back a slot the NPC
// spent on a Shield of the same level. The cast of today says it counted its slot and the
// undo gives it back. Needs the database.
func TestUndoingAnNPCCastRecordedBeforeSlotsWereCounted(t *testing.T) {
	t.Parallel()
	cast := func(t *testing.T) (*armed, *playv1.Encounter) {
		t.Helper()
		a, e := mageFight(t)
		e = a.passTo(t, e, "Mago")
		a.mustCast(t, a.master, e, "Mago", magicMissileSpell, slotOfLevel(1), []*playv1.SpellTarget{darts(a, t, "Toren", 3)}, noCastRoll)
		return a, e
	}
	t.Run("an old event", func(t *testing.T) {
		t.Parallel()
		a, e := cast(t)
		// The 1st-level slot counted is the Shield's: the old event has the slot but no slot_counted.
		if tag, err := a.h.pool.Exec(t.Context(), `UPDATE session_events SET payload = payload - 'slot_counted' WHERE kind = 'spell_cast' AND payload ? 'slot_counted'`); err != nil || tag.RowsAffected() != 1 {
			t.Fatalf("make the cast an old event: %v, %d rows", err, tag.RowsAffected())
		}
		a.undoLast(t, e)
		if used := a.usedOf(t, "Mago"); used != `{"1": 1}` {
			t.Errorf("the Mage's count after the undo of the old cast = %s, want the Shield's slot kept", used)
		}
	})
	t.Run("a cast of today", func(t *testing.T) {
		t.Parallel()
		a, e := cast(t)
		var counted bool
		if err := a.h.pool.QueryRow(t.Context(), `SELECT payload ? 'slot_counted' FROM session_events WHERE kind = 'spell_cast'`).Scan(&counted); err != nil || !counted {
			t.Fatalf("the cast says it counted its slot = %v (%v), want true", counted, err)
		}
		a.undoLast(t, e)
		if used := a.usedOf(t, "Mago"); used != `{"1": 0}` {
			t.Errorf("the Mage's count after the undo = %s, want the slot back", used)
		}
	})
}
