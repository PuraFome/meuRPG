package play

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Who reads a rider's save and what it leaves in the log (RN-10, RN-20), and what the undo may take back.

const saveCanary = "Zorvak4321"

var reasonReactionPending = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_REACTION_PENDING

// copyOf is the encounter as u reads it, as text.
func (c *cave) copyOf(t *testing.T, u *user) string {
	t.Helper()
	return protojson.Format(c.get(t, u))
}

// logOf is the log as u reads it, as text.
func (c *cave) logOf(t *testing.T, u *user) string {
	t.Helper()
	return protojson.Format(c.log(t, u, c.get(t, u)))
}

// A hidden mount: the rider's player and the other players read nothing of the pair, the wait is the master's, the lines of the save and
// of the fall are the master's alone, and the canary is in no player's copy, log or refusal. The master reads all of it (the control).
func TestMountedCombat_AHiddenMountLeaksNothingOfTheSave(t *testing.T) {
	t.Parallel()
	c := riddenCave(t)
	c.execSQL(t, `UPDATE combatants SET label = $1 WHERE id = $2`, saveCanary, c.id(t, "Ogro"))
	for who, u := range map[string]*user{"the master": c.master, "Toren's player": c.caio, "another player": c.ana} {
		if !strings.Contains(c.copyOf(t, u), saveCanary) {
			t.Fatalf("the positive control failed: %s does not read %s while the mount is shown", who, saveCanary)
		}
	}
	c.execSQL(t, `UPDATE combatants SET hidden = true WHERE id = $1`, c.id(t, saveCanary))
	c.askSave(t, "Toren", causePushed)

	if got := c.mountSaveOf(t, c.master); got.GetMountId() != c.id(t, saveCanary) {
		t.Errorf("the master reads %v, want the save with the hidden mount (the control)", got)
	}
	for who, u := range map[string]*user{"Toren's player": c.caio, "another player": c.ana} {
		if got := c.mountSaveOf(t, u); got.GetMountId() != "" {
			t.Errorf("%s reads the hidden mount in %v", who, got)
		}
		if json := c.copyOf(t, u); strings.Contains(json, saveCanary) || strings.Contains(json, c.id(t, saveCanary)) {
			t.Errorf("%s reads the hidden mount in their copy", who)
		}
	}
	enc := c.get(t, c.master)
	_, err := c.endTurn(t, c.caio, enc, false)
	b := wantBlockedBy(t, "the rider's end of turn", err, reasonReactionPending)
	if len(b.GetCombatantIds()) != 0 || strings.Contains(err.Error(), saveCanary) {
		t.Errorf("the refusal %v names somebody", b)
	}
	_, err = c.endTurn(t, c.master, enc, false)
	wantBlockedBy(t, "the master's end of turn", err, reasonMountSavePending)

	c.mustRollSave(t, c.master, "Toren", saveFace(1)) // the master rolls for the player: a fall
	for who, u := range map[string]*user{"Toren's player": c.caio, "another player": c.ana} {
		for _, k := range []playv1.CombatLogKind{playv1.CombatLogKind_COMBAT_LOG_KIND_MOUNT_SAVE, playv1.CombatLogKind_COMBAT_LOG_KIND_RIDER_THROWN} {
			if got := c.linesOfKind(t, u, k); len(got) != 0 {
				t.Errorf("%s reads %v lines %v, want none", who, k, got)
			}
		}
		if strings.Contains(c.logOf(t, u), saveCanary) {
			t.Errorf("%s reads the hidden mount in the log", who)
		}
	}
	thrown := c.linesOfKind(t, c.master, playv1.CombatLogKind_COMBAT_LOG_KIND_RIDER_THROWN)
	if len(thrown) != 1 || !thrown[0].GetHidden() || thrown[0].GetTargetLabel() != saveCanary {
		t.Errorf("the master reads %v, want one RIDER_THROWN line with the hidden mount, marked hidden (the control)", thrown)
	}
}

// A hidden rider (an NPC): only the master reads the save, and the lines of it are his alone.
func TestMountedCombat_AHiddenRiderLeaksNothingOfTheSave(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.fight(t)
	c.steed(t)
	c.mustMove(t, c.master, "Goblin 1", 6, 7)
	c.execSQL(t, `UPDATE combatants SET label = $1 WHERE id = $2`, saveCanary, c.id(t, "Goblin 1"))
	c.sitOn(t, saveCanary, "Ogro")
	for who, u := range map[string]*user{"the master": c.master, "another player": c.ana} {
		if !strings.Contains(c.copyOf(t, u), saveCanary) {
			t.Fatalf("the positive control failed: %s does not read %s while the rider is shown", who, saveCanary)
		}
	}
	c.execSQL(t, `UPDATE combatants SET hidden = true WHERE id = $1`, c.id(t, saveCanary))
	c.askSave(t, saveCanary, causePushed)

	if got := c.mountSaveOf(t, c.master); got.GetRiderId() != c.id(t, saveCanary) {
		t.Errorf("the master reads %v, want the save of the hidden rider (the control)", got)
	}
	for who, u := range map[string]*user{"Toren's player": c.caio, "another player": c.ana} {
		if got := c.mountSaveOf(t, u); got != nil {
			t.Errorf("%s reads the save %v", who, got)
		}
		if json := c.copyOf(t, u); strings.Contains(json, saveCanary) || strings.Contains(json, c.id(t, saveCanary)) {
			t.Errorf("%s reads the hidden rider in their copy", who)
		}
	}
	_, err := c.endTurn(t, c.caio, c.get(t, c.master), false)
	wantBlockedBy(t, "a player's end of turn", err, reasonReactionPending)

	c.h.roller.queue(1)
	c.mustRollSave(t, c.master, saveCanary, saveInApp)
	for who, u := range map[string]*user{"Toren's player": c.caio, "another player": c.ana} {
		for _, k := range []playv1.CombatLogKind{playv1.CombatLogKind_COMBAT_LOG_KIND_MOUNT_SAVE, playv1.CombatLogKind_COMBAT_LOG_KIND_RIDER_THROWN} {
			if got := c.linesOfKind(t, u, k); len(got) != 0 {
				t.Errorf("%s reads %v lines %v, want none", who, k, got)
			}
		}
		if strings.Contains(c.logOf(t, u), saveCanary) {
			t.Errorf("%s reads the hidden rider in the log", who)
		}
	}
	if thrown := c.linesOfKind(t, c.master, playv1.CombatLogKind_COMBAT_LOG_KIND_RIDER_THROWN); len(thrown) != 1 || !thrown[0].GetHidden() {
		t.Errorf("the master reads %v, want one RIDER_THROWN line marked hidden (the control)", thrown)
	}
}

// The push is undoable only before the save is answered: the undo before it takes the window with it, and once the roll is written
// (a closing event) the push cannot be taken back any more.
func TestMountedCombat_TheCauseIsUndoableOnlyBeforeTheAnswer(t *testing.T) {
	t.Parallel()
	t.Run("before the answer", func(t *testing.T) {
		t.Parallel()
		c := newAreaCave(t)
		c.mountedWave(t)
		c.wave(t)
		if c.mountSaveOf(t, c.master) == nil {
			t.Fatal("the push asked no save")
		}
		c.mustUndo(t, c.get(t, c.master))
		wantSquare(t, c, "Ogro", [2]int32{18, 7})
		wantSquare(t, c, "Toren", [2]int32{18, 7})
		if got := c.mountSaveOf(t, c.master); got != nil || len(c.openSaveWindows(t)) != 0 {
			t.Errorf("the save %v is still asked after the undo of its push", got)
		}
	})
	t.Run("after the answer", func(t *testing.T) {
		t.Parallel()
		c := newAreaCave(t)
		c.mountedWave(t)
		c.wave(t)
		cast := c.log(t, c.master, c.get(t, c.master)).GetUndoableEventId()
		if cast == "" {
			t.Fatal("the push has no event to undo")
		}
		c.mustRollSave(t, c.caio, "Toren", saveFace(20))
		if id := c.log(t, c.master, c.get(t, c.master)).GetUndoableEventId(); id != "" {
			t.Errorf("the log offers %s to undo after the answer, want nothing: the roll is a closing event", id)
		}
		if err := c.undo(t, c.master, c.get(t, c.master), cast); err == nil {
			t.Error("the undo of the push after the answer succeeded, want it refused")
		}
		wantSquare(t, c, "Ogro", [2]int32{20, 7})
	})
}
