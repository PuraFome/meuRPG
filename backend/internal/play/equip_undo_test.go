package play

import (
	"context"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
)

// Undoing a swap of gear in a combat: the swap is an event of its own, so the master's
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

// The undo of a swap and SetEquipped lock the open session first and the character second, as every combat
// write does, so none of them holds the character while it waits for the session. Each test holds the
// session as a writer that is ahead, shows the other one waiting for it while the character is free (a
// side connection locks the character without waiting), and finishes after the commit: no abort, no retry.
// They read no PostgreSQL view, so they run on CockroachDB too.

// sessionHeld opens a side transaction that holds the campaign's open session, and returns it.
func sessionHeld(t *testing.T, ctx context.Context, a *armed) pgx.Tx {
	t.Helper()
	held, err := dbtest.SideConnection(t, a.h.pool).Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	t.Cleanup(func() { _ = held.Rollback(context.Background()) }) // rolled back unless committed
	if _, err := held.Exec(ctx, `SELECT id FROM game_sessions WHERE campaign_id = $1 AND ended_at IS NULL FOR UPDATE`, a.campaignID); err != nil {
		t.Fatalf("lock the session: %v", err)
	}
	return held
}

// waitsForTheSession waits until a statement that locks the session is waiting behind the held one, and
// checks the character is free meanwhile: the waiting call has not taken it.
func waitsForTheSession(t *testing.T, ctx context.Context, a *armed, held pgx.Tx, what string) {
	t.Helper()
	observer := dbtest.SideConnection(t, a.h.pool)
	start := time.Now()
	for {
		n, err := dbtest.WaitingStatements(ctx, observer, `%FROM game_sessions%FOR UPDATE%`)
		if err != nil {
			t.Fatalf("look for waiting statements: %v", err)
		}
		if n > 0 {
			break
		}
		if ctx.Err() != nil || time.Since(start) > 15*time.Second {
			t.Fatalf("%s never waited for the session", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := held.Exec(ctx, `SELECT id FROM characters WHERE id = $1 FOR UPDATE NOWAIT`, a.pens.GetId()); err != nil {
		t.Fatalf("%s holds the character while it waits for the session: %v", what, err)
	}
}

func TestUndoOfASwapTakesTheSessionBeforeTheCharacter(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	e = a.passTo(t, e, "Reator")
	a.mustSwap(t, a.ana, a.pens, daggerKey, false)
	id := a.lastEventID(t, e)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	held := sessionHeld(t, ctx, a)
	undone := make(chan error, 1)
	go func() { undone <- a.undo(t, a.master, e, id) }()
	waitsForTheSession(t, ctx, a, held, "the undo")
	select {
	case err := <-undone:
		t.Fatalf("UndoLastAction() returned while the session was held: %v", err)
	default:
	}
	if err := held.Commit(ctx); err != nil {
		t.Fatalf("commit the holder: %v", err)
	}
	if err := <-undone; err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	if a.hasCarried(t, daggerKey) {
		t.Error("the dagger is still stowed after the undo")
	}
	a.wantTurn(t, "after the undo", false, false)
}

func TestSetEquippedTakesTheSessionBeforeTheCharacter(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	a.passTo(t, e, "Reator")
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	held := sessionHeld(t, ctx, a)
	swapped := make(chan error, 1)
	go func() {
		_, err := a.swap(a.ana, a.pens, daggerKey, false)
		swapped <- err
	}()
	waitsForTheSession(t, ctx, a, held, "SetEquipped")
	select {
	case err := <-swapped:
		t.Fatalf("SetEquipped() returned while the session was held: %v", err)
	default:
	}
	if err := held.Commit(ctx); err != nil {
		t.Fatalf("commit the holder: %v", err)
	}
	if err := <-swapped; err != nil {
		t.Fatalf("SetEquipped() error = %v", err)
	}
	if !a.hasCarried(t, daggerKey) {
		t.Error("the swap did not change the dagger's state")
	}
	a.wantTurn(t, "after the swap", true, false)
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

// A player's armor swap is refused in a combat; the master's is allowed and enters the undo: the undo puts the armor
// back on the sheet and the armor class with it, and the line says "armor".
func TestUndoTheMastersArmorSwap(t *testing.T) {
	t.Parallel()
	a, e := equipFight(t)
	e = a.passTo(t, e, "Reator")
	ac := func() int32 { return byLabel(t, a.get(t, a.master), "Reator").GetArmorClass() }
	if got := ac(); got != 18 {
		t.Fatalf("armor class in chain mail with the shield = %d, want 18", got)
	}
	a.mustSwap(t, a.master, a.pens, chainMailKey, false)
	if got := ac(); got != 14 {
		t.Fatalf("armor class with the armor stowed = %d, want 14", got)
	}
	if line := gearLine(a.log(t, a.master, e)); line == nil || line.GetGearSwapped().GetKind() != "armor" {
		t.Errorf("the master reads %v, want a gear line of kind armor", line)
	}
	a.mustUndo(t, e)
	if a.hasCarried(t, chainMailKey) {
		t.Error("the armor is still stowed after the undo")
	}
	if got := ac(); got != 18 {
		t.Errorf("armor class after the undo = %d, want 18", got)
	}
	a.wantTurn(t, "after the undo", false, false)
}
