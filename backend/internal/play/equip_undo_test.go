package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Undoing a swap of gear in a combat (#787): the swap is an event of its own, so the master's
// "undo the last action" takes it back first, with the economy and the sheet's equipped state
// from before it, and only then the action under it.

// lastEventID is the id the master's undo expects.
func (a *armed) lastEventID(t *testing.T, e *playv1.Encounter) string {
	t.Helper()
	id := a.log(t, a.master, e).GetUndoableEventId()
	if id == "" {
		t.Fatal("nothing to undo, want an event")
	}
	return id
}

func (a *armed) mustUndo(t *testing.T, e *playv1.Encounter) {
	t.Helper()
	if err := a.undo(t, a.master, e, a.lastEventID(t, e)); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
}

func (a *armed) hasCarried(t *testing.T, key string) bool {
	t.Helper()
	return slices.Contains(a.carriedOf(t, a.pens), key)
}

// attack, swap, undo, undo: the swap goes first and gives back the free interaction and the
// piece; the second undo takes the attack and its action.
func TestUndoTakesTheSwapBeforeTheActionUnderIt(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	e = a.passTo(t, e, "Reator")
	a.mustAttack(t, a.ana, e, "Reator", longswordKey, "Goblin", d20(15))
	a.wantTurn(t, "after the attack", false, true)
	a.mustSwap(t, a.ana, a.pens, daggerKey, false)
	a.wantTurn(t, "after the swap", true, true)
	if !a.hasCarried(t, daggerKey) {
		t.Fatal("the dagger is not stowed after the swap")
	}

	a.mustUndo(t, e)
	a.wantTurn(t, "after undoing the swap", false, true)
	if a.hasCarried(t, daggerKey) {
		t.Error("the dagger is still stowed after undoing the swap")
	}

	a.mustUndo(t, e)
	a.wantTurn(t, "after undoing the attack", false, false)
}

// Undoing an earlier event is not possible past the swap: the swap is undone first, and it
// does not give back what it spent when the attack under it is undone.
func TestUndoAnEarlierEventKeepsWhatTheSwapSpentUntilTheSwapIsUndone(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	e = a.passTo(t, e, "Reator")
	a.mustAttack(t, a.ana, e, "Reator", longswordKey, "Goblin", d20(15))
	first := a.lastEventID(t, e)
	a.mustSwap(t, a.ana, a.pens, daggerKey, false)
	if err := a.undo(t, a.master, e, first); err == nil {
		t.Fatal("UndoLastAction(the attack under the swap) succeeded, want it refused")
	}
	a.wantTurn(t, "after the refused undo", true, true)
	if !a.hasCarried(t, daggerKey) {
		t.Error("a refused undo gave the dagger back")
	}
}

// A second swap costs the action: undoing it gives the action back and leaves the first swap.
func TestUndoTheSecondSwapGivesTheActionBack(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	e = a.passTo(t, e, "Reator")
	a.mustSwap(t, a.ana, a.pens, longswordKey, false)
	a.mustSwap(t, a.ana, a.pens, daggerKey, false)
	a.wantTurn(t, "after two swaps", true, true)
	a.mustUndo(t, e)
	a.wantTurn(t, "after undoing the second", true, false)
	if a.hasCarried(t, daggerKey) || !a.hasCarried(t, longswordKey) {
		t.Errorf("carried = %v, want the longsword only", a.carriedOf(t, a.pens))
	}
	a.mustUndo(t, e)
	a.wantTurn(t, "after undoing the first", false, false)
	if got := a.carriedOf(t, a.pens); len(got) != 0 {
		t.Errorf("carried = %v, want nothing stowed", got)
	}
}

// The shield is the action: undoing its doffing gives the action and the shield back.
func TestUndoTheShieldGivesTheActionAndTheShieldBack(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	e = a.passTo(t, e, "Reator")
	a.mustSwap(t, a.ana, a.pens, shieldItem, false)
	a.wantTurn(t, "after doffing", false, true)
	a.mustUndo(t, e)
	a.wantTurn(t, "after the undo", false, false)
	if a.hasCarried(t, shieldItem) {
		t.Error("the shield is still stowed")
	}
	// the action is back: the shield can be doffed again.
	a.mustSwap(t, a.ana, a.pens, shieldItem, false)
}

// Haste's extra action paid for the swap: undoing gives it back, not the action.
func TestUndoTheSwapPaidByTheExtraActionGivesItBack(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	e = a.passTo(t, e, "Reator")
	a.mustAddEffect(t, e, "spell:haste", []string{"Reator"}, a.rounds(t, 3, "Reator"))
	a.mustSwap(t, a.ana, a.pens, shieldItem, false)
	a.mustSwap(t, a.ana, a.pens, longswordKey, false)
	a.mustSwap(t, a.ana, a.pens, daggerKey, false)
	a.mustUndo(t, e)
	var extra bool
	if err := a.h.pool.QueryRow(t.Context(), `SELECT extra_action_used FROM combatants WHERE id = $1`, a.id(t, "Reator")).Scan(&extra); err != nil || extra {
		t.Fatalf("extra action used = %v (%v), want it given back", extra, err)
	}
	a.wantTurn(t, "after the undo", true, true)
}

// The master's own swap enters the undo too.
func TestUndoTheMastersSwap(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	e = a.passTo(t, e, "Reator")
	a.mustSwap(t, a.master, a.pens, shieldItem, false)
	a.mustUndo(t, e)
	if a.hasCarried(t, shieldItem) {
		t.Error("the shield is still stowed")
	}
}

// The sheet no longer lists the piece the swap touched: the undo is refused and writes nothing.
func TestUndoIsRefusedWhenTheSheetLostThePiece(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	e = a.passTo(t, e, "Reator")
	a.mustSwap(t, a.ana, a.pens, daggerKey, false)
	// the dagger leaves the sheet: the owner edits the weapons list.
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE characters SET sheet = jsonb_set(sheet, '{full,weapon_keys}', (sheet #> '{full,weapon_keys}') - 1) WHERE id = $1 AND sheet #>> '{full,weapon_keys,1}' = $2`, a.pens.GetId(), daggerKey); err != nil {
		t.Fatalf("remove the dagger from the sheet: %v", err)
	}
	before := a.carriedOf(t, a.pens)
	err := a.undo(t, a.master, e, a.lastEventID(t, e))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("UndoLastAction() error = %v, want failed precondition", err)
	}
	a.wantTurn(t, "after the refused undo", true, false)
	if got := a.carriedOf(t, a.pens); !slices.Equal(got, before) {
		t.Errorf("a refused undo changed the sheet: %v -> %v", before, got)
	}
}

// The encounter revision moves with the undo, so the web rereads the gear.
func TestUndoTheSwapBumpsTheEncounterRevision(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	e = a.passTo(t, e, "Reator")
	a.mustSwap(t, a.ana, a.pens, daggerKey, false)
	rev := func() int64 {
		var r int64
		if err := a.h.pool.QueryRow(t.Context(), `SELECT revision FROM encounters WHERE id = $1`, e.GetId()).Scan(&r); err != nil {
			t.Fatalf("read the revision: %v", err)
		}
		return r
	}
	before := rev()
	a.mustUndo(t, e)
	if after := rev(); after <= before {
		t.Errorf("revision %d -> %d, want it to move", before, after)
	}
}
