package play

import (
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
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
	if err := a.h.pool.QueryRow(t.Context(), `SELECT slots_used::STRING FROM combatants WHERE id = $1`, a.id(t, label)).Scan(&used); err != nil {
		t.Fatalf("slots_used of %s: %v", label, err)
	}
	return used
}

// TestAnNPCMagesCastSpendsASlotAndTheUndoGivesItBack: the Mage casts Mísseis Mágicos with a
// 1st-level slot as the master; the count on the combatant takes it, the options show one
// slot less to the master, and the undo restores both. Needs the database.
func TestAnNPCMagesCastSpendsASlotAndTheUndoGivesItBack(t *testing.T) {
	t.Parallel()
	a, e := mageFight(t)
	e = a.passTo(t, e, "Mago")
	a.mustCast(t, a.master, e, "Mago", magicMissileSpell, slotOfLevel(1), []*playv1.SpellTarget{darts(a, t, "Toren", 3)}, noCastRoll)
	if used := a.usedOf(t, "Mago"); !strings.Contains(used, `"1": 1`) {
		t.Fatalf("the Mage's count after the cast = %s, want one 1st-level slot", used)
	}
	a.undoLast(t, e)
	if used := a.usedOf(t, "Mago"); strings.Contains(used, `"1": 1`) {
		t.Errorf("the Mage's count after the undo = %s, want the slot back", used)
	}
}
