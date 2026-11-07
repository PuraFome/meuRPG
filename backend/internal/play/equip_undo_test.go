package play

import (
	"context"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
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
	ac := func() int32 { return byLabel(t, a.get(t, a.master), "Reator").GetArmorClass() }
	if got := ac(); got != 18 {
		t.Fatalf("armor class with the shield = %d, want 18", got)
	}
	a.mustSwap(t, a.ana, a.pens, shieldItem, false)
	a.wantTurn(t, "after doffing", false, true)
	if got := ac(); got != 16 {
		t.Fatalf("armor class without the shield = %d, want 16", got)
	}
	a.mustUndo(t, e)
	a.wantTurn(t, "after the undo", false, false)
	if got := ac(); got != 18 {
		t.Errorf("armor class after the undo = %d, want 18: the derived gear is read again", got)
	}
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

// The undo locks the session and then the character (RestoreEquipped); SetEquipped locks the
// character and then the session. When they cross, the database aborts one with 40P01 and
// db.InTx runs the undo again: the undo still succeeds, and the evidence of the retry is that
// the second attempt waits for the session's lock, which the first one took without waiting.
func TestUndoOfASwapCrossingASetEquippedTakesTurns(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	e = a.passTo(t, e, "Reator")
	a.mustSwap(t, a.ana, a.pens, daggerKey, false)
	id := a.lastEventID(t, e)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	observer := dbtest.SideConnection(t, a.h.pool)
	deadlocks := func() int64 {
		var n int64
		if err := observer.QueryRow(ctx, `SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()`).Scan(&n); err != nil {
			t.Fatalf("read the deadlock counter: %v", err)
		}
		return n
	}
	waiting := func(like string) int {
		n, err := dbtest.WaitingStatements(ctx, observer, like)
		if err != nil {
			t.Fatalf("look for waiting statements: %v", err)
		}
		return n
	}
	waitFor := func(what, like string) {
		t.Helper()
		start := time.Now()
		for waiting(like) < 1 {
			if ctx.Err() != nil || time.Since(start) > 15*time.Second {
				rows, _ := observer.Query(context.Background(), `SELECT pid, coalesce(wait_event_type, ''), left(query, 160) FROM pg_stat_activity WHERE datname = current_database() AND state <> 'idle'`)
				for rows != nil && rows.Next() {
					var pid int
					var w, q string
					_ = rows.Scan(&pid, &w, &q)
					t.Logf("active: %d [%s] %s", pid, w, q)
				}
				t.Fatalf("%s never waited", what)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	before := deadlocks()

	// SetEquipped's side: the character first.
	owner, err := dbtest.SideConnection(t, a.h.pool).Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	defer func() { _ = owner.Rollback(ctx) }() // rolled back unless committed
	if _, err := owner.Exec(ctx, `SELECT id FROM characters WHERE id = $1 FOR UPDATE`, a.pens.GetId()); err != nil {
		t.Fatalf("lock the character: %v", err)
	}
	// The undo takes the session and stops at the character: its first write to the combatant
	// asks for a share of the character's row (the foreign key), which the lock above refuses.
	undone := make(chan error, 1)
	go func() { undone <- a.undo(t, a.master, e, id) }()
	waitFor("the undo", `%UPDATE combatants%`)
	// SetEquipped's side goes on to the session: the cycle.
	locked := make(chan error, 1)
	go func() {
		_, err := owner.Exec(ctx, `SELECT id FROM game_sessions WHERE campaign_id = $1 AND ended_at IS NULL FOR UPDATE`, a.campaignID)
		locked <- err
	}()
	// The database aborts the undo (the older wait): SetEquipped's side gets the session...
	if err := <-locked; err != nil {
		t.Fatalf("the owner's side was the one aborted: %v", err)
	}
	// ...and the second attempt of the undo now waits for the session's lock.
	waitFor("the retry of the undo", `%FROM game_sessions%FOR UPDATE%`)
	select {
	case err := <-undone:
		t.Fatalf("UndoLastAction() returned while the session was held: %v", err)
	default:
	}
	if err := owner.Commit(ctx); err != nil {
		t.Fatalf("commit the owner's side: %v", err)
	}
	if err := <-undone; err != nil {
		t.Fatalf("UndoLastAction() after the retry error = %v", err)
	}
	if a.hasCarried(t, daggerKey) {
		t.Error("the dagger is still stowed after the retried undo")
	}
	a.wantTurn(t, "after the retried undo", false, false)
	deadline := time.Now().Add(10 * time.Second)
	for deadlocks() <= before && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if got := deadlocks(); got <= before {
		t.Errorf("deadlocks %d -> %d, want the database to have counted the 40P01", before, got)
	}
}

func gearLine(res *playv1.ListCombatLogResponse) *playv1.CombatLogEntry {
	for _, r := range res.GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_GEAR_SWAPPED {
				return en
			}
		}
	}
	return nil
}

// The line of a swap in the combat log: the master and the owner read the piece, the way it went and
// what it cost; a player who does not own the character reads nothing. A master's swap costs nothing
// and the line says the master made it.
func TestTheLogHasTheSwapForTheMasterAndTheOwnerAlone(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	e = a.passTo(t, e, "Reator")
	a.mustSwap(t, a.ana, a.pens, longswordKey, false)
	a.mustSwap(t, a.ana, a.pens, daggerKey, false)
	for i, u := range []*user{a.master, a.ana} {
		line := gearLine(a.log(t, u, e))
		if line == nil {
			t.Fatalf("a gear line is missing from the log of reader %d", i)
		}
		g := line.GetGearSwapped()
		if g.GetNamePt() == "" || g.GetKind() != "weapon" || g.GetEquipped() || g.GetCost() != "action" {
			t.Errorf("reader %d reads %v, want the dagger stowed for the action", i, g)
		}
	}
	if line := gearLine(a.log(t, a.bia, e)); line != nil {
		t.Errorf("another player reads %v", line)
	}
	a.mustUndo(t, e)
	for i, u := range []*user{a.master, a.ana} {
		line := gearLine(a.log(t, u, e))
		if line == nil || line.GetGearSwapped().GetCost() != "free" {
			t.Errorf("reader %d reads %v after undoing the second, want the first (free)", i, line)
		}
	}
	a.mustUndo(t, e)
	if line := gearLine(a.log(t, a.master, e)); line != nil {
		t.Errorf("the master reads %v after undoing both", line)
	}

	a.mustSwap(t, a.master, a.pens, shieldItem, false)
	line := gearLine(a.log(t, a.ana, e))
	if line == nil || line.GetGearSwapped().GetCost() != "" || !line.GetByMaster() {
		t.Errorf("the owner reads %v for the master's swap, want a free line by the master", line)
	}
}
