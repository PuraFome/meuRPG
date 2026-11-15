package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
)

// The DC 10 Dexterity saving throw of a rider (SRD 5.1, "Mounted Combat"): a rider whose mount is moved against its will, or that is
// knocked prone while it rides, owes a pending save (Encounter.mount_save) that holds the turn like a concentration save. These tests
// look at what opens the window, what does not, who reads it and what it holds. The roll is in combat_mount_save_roll_test.go.

var (
	reasonMountSavePending = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_MOUNT_SAVE_PENDING
	causePushed            = playv1.MountSaveCause_MOUNT_SAVE_CAUSE_PUSHED
	causeProne             = playv1.MountSaveCause_MOUNT_SAVE_CAUSE_PRONE
)

// askSave opens the window of a rider that sits on a mount through the same function the triggers call, for the tests of what the
// window does once it is open.
func (c *armed) askSave(t *testing.T, rider string, cause playv1.MountSaveCause) {
	t.Helper()
	m := authz.Membership{CampaignID: c.campaignID, UserID: c.master.id, Role: authz.RoleMaster}
	encounterID, riderID := c.get(t, c.master).GetId(), c.id(t, rider) // read before the transaction: the test pool has one connection
	_, err := c.h.svc.write(t.Context(), combatWrite{m: m, key: newKey(), kind: eventConditionsSet, encounterID: encounterID}, func(cx *combatTx) (any, error) {
		cs, err := listCombatants(t.Context(), cx)
		if err != nil {
			return nil, err
		}
		who, _ := combatantByID(cs, riderID)
		mount := ridingOn(cs, who)
		if mount == nil {
			t.Fatalf("%s rides nothing", rider)
		}
		if err := c.h.svc.openMountSave(t.Context(), cx, cs, who, *mount, cause); err != nil {
			return nil, err
		}
		return actionEvent{Round: cx.enc.Round, Actor: who.ID}, nil
	})
	if err != nil {
		t.Fatalf("open the mount save of %s: %v", rider, err)
	}
}

// mountSaveOf is the pending save the master reads in an armed arena.
func (c *armed) mountSaveOf(t *testing.T, u *user) *playv1.MountSavePrompt {
	t.Helper()
	return c.get(t, u).GetMountSave()
}

// wantSave checks the window the master reads: the rider, the mount, the DC and the cause.
func wantSave(t *testing.T, what string, got *playv1.MountSavePrompt, rider, mount string, cause playv1.MountSaveCause) {
	t.Helper()
	if got == nil {
		t.Errorf("%s: the master reads no mount save, want one for the rider", what)
		return
	}
	if got.GetRiderId() != rider || got.GetMountId() != mount || got.GetDc() != 10 || got.GetCause() != cause {
		t.Errorf("%s: mount_save = %v, want rider %s on %s, DC 10, cause %v", what, got, rider, mount, cause)
	}
}

// mountedWave is the area fight with Toren on the Ogro at (18,7) and Pensantus at (17,7), on her turn, to cast Thunderwave eastwards.
func (c *cave) mountedWave(t *testing.T) {
	t.Helper()
	e := c.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: c.capitao.GetId()}, {CharacterId: c.ogre.GetId()}},
		npcRolls: []int{2, 2},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Capitão Goblin", "Ogro"},
		at: map[string][2]int32{
			"Pensantus": {17, 7}, "Toren": {21, 8}, "Brisa": {4, 7}, "Capitão Goblin": {12, 13}, "Ogro": {18, 7},
		},
	})
	c.passTo(t, e, "Pensantus")
	c.mustMark(t, "Ogro", "controlled")
	c.sitOn(t, "Toren", "Ogro")
}

// A Thunderwave that pushes the mount opens the rider's save: the rider's player and the master read it, another player reads
// that the table waits and nothing else, and the push of the rider itself is not done (a rider is never pushed off its mount's square).
func TestMountedCombat_ASpellThatPushesTheMountAsksTheRidersSave(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.mountedWave(t)

	c.wave(t)
	wantSquare(t, c, "Ogro", [2]int32{20, 7})
	wantSquare(t, c, "Toren", [2]int32{20, 7})
	wantSave(t, "after the push", c.mountSaveOf(t, c.master), c.id(t, "Toren"), c.id(t, "Ogro"), causePushed)
	if got := c.mountSaveOf(t, c.caio); got.GetRiderId() != c.id(t, "Toren") {
		t.Errorf("Toren's player reads %v, want the save of Toren", got)
	}
	if got := c.mountSaveOf(t, c.ana); got != nil {
		t.Errorf("another player reads the save %v, want none", got)
	}
	if w := c.get(t, c.ana).GetReactionWait(); w == nil || w.GetTitlePt() == "" {
		t.Errorf("another player reads no wait: the turn is held")
	}
}

// A mount that moves on its own is no trigger: the rider's player moving the controlled mount, the master moving it by hand, and the
// master's correction of a conditions list. A push that is undone before it is answered takes the window with it.
func TestMountedCombat_WhatDoesNotAskTheSave(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.ridden(t)

	c.mustMove(t, c.caio, "Ogro", 6, 7) // the rider's player drives its controlled mount
	if got := c.mountSaveOf(t, c.master); got != nil {
		t.Errorf("the mount's own move asked %v, want no save", got)
	}
	c.mustMove(t, c.master, "Ogro", 5, 7) // the master's hand on the token
	if got := c.mountSaveOf(t, c.master); got != nil {
		t.Errorf("the master's move of the mount asked %v, want no save", got)
	}
	if got := c.rideRowOf(t, "Toren"); got.Col != 5 || got.Row != 7 || got.MountedOn == "" {
		t.Errorf("the rider after the moves = %+v, want carried to (5,7) and still on", got)
	}
	// The master's manual edit of the conditions is a correction: it asks nothing, and the positive control is the same edit by the
	// effect that knocks prone (the shove below).
	if _, err := c.conditions(t, c.master, c.get(t, c.master), "Toren", []string{condProne}, true, false); err != nil {
		t.Fatalf("SetCombatantConditions() error = %v", err)
	}
	if got := c.mountSaveOf(t, c.master); got != nil {
		t.Errorf("the master marking the rider prone asked %v, want no save", got)
	}
}

// A shove that pushes the mount asks the rider's save, with the shove's own player reading nothing of it.
func TestMountedCombat_AShovePushingTheMountAsksTheRidersSave(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	c.seat(t, "Goblin", c.hob)
	started := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
		c.h.roller.queue(18)
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), playv1.ContestPurpose_CONTEST_PURPOSE_SHOVE
	})
	view := c.mustRespond(t, a.master, started.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, 2).GetContest()
	c.mustShove(t, a.caio, view.GetId(), playv1.ShoveOutcome_SHOVE_OUTCOME_PUSH)
	wantSave(t, "after the shove", a.mountSaveOf(t, a.master), c.id(t, "Goblin"), c.id(t, c.hob), causePushed)
	if got := a.mountSaveOf(t, a.caio); got != nil {
		t.Errorf("the shover's player reads the save %v, want none (the rider is an NPC)", got)
	}
}

// A shove that only knocks the mount prone does not move it: no save (the mount knocked prone is another rule).
func TestMountedCombat_AShoveThatKnocksTheMountProneAsksNoPushSave(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	c.seat(t, "Goblin", c.hob)
	started := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
		c.h.roller.queue(18)
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), playv1.ContestPurpose_CONTEST_PURPOSE_SHOVE
	})
	view := c.mustRespond(t, a.master, started.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, 2).GetContest()
	c.mustShove(t, a.caio, view.GetId(), playv1.ShoveOutcome_SHOVE_OUTCOME_PRONE)
	if got := a.mountSaveOf(t, a.master); got != nil {
		t.Errorf("a shove that knocks the mount prone asked %v, want no save of the rider", got)
	}
}

// A shove that knocks the rider prone asks the save with the cause PRONE; the shove that would push the rider stays refused.
func TestMountedCombat_AShoveThatKnocksTheRiderProneAsksItsSave(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Hobgoblin": {5, 5}, "Goblin": {4, 3}}})
	c.seat(t, c.hob, "Goblin")
	c.h.roller.queue(18)
	shove := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), playv1.ContestPurpose_CONTEST_PURPOSE_SHOVE
	}).GetContest()
	view := c.mustRespond(t, a.master, shove.GetId(), playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, 2).GetContest()
	_, err := c.resolveShove(t, a.caio, view.GetId(), playv1.ShoveOutcome_SHOVE_OUTCOME_PUSH)
	wantContestBlocked(t, "pushing a rider", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_PUSH_BLOCKED)
	if got := a.mountSaveOf(t, a.master); got != nil {
		t.Fatalf("the refused push of the rider asked %v, want no save", got)
	}
	c.mustShove(t, a.caio, view.GetId(), playv1.ShoveOutcome_SHOVE_OUTCOME_PRONE)
	wantSave(t, "after the prone shove", a.mountSaveOf(t, a.master), c.id(t, c.hob), c.id(t, "Goblin"), causeProne)
}

// A grappler that drags the mount moves it against its will: the rider owes the save. The grappler's own walk is not a trigger.
func TestMountedCombat_ADragOfTheMountAsksTheRidersSave(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := dragFight(t, a, arenaPlan{})
	c.seat(t, "Goblin", c.hob)
	if _, err := c.moveTo(t, a.caio, "Toren", 3, 0); err != nil {
		t.Fatalf("a three-square drag: %v", err)
	}
	wantSave(t, "after the drag", a.mountSaveOf(t, a.master), c.id(t, "Goblin"), c.id(t, c.hob), causePushed)
}

// Grease knocks the rider prone while it rides: the save is asked with the cause PRONE.
func TestMountedCombat_ASpellThatKnocksTheRiderProneAsksItsSave(t *testing.T) {
	t.Parallel()
	c := greaseCave(t)
	c.passTo(t, c.zoneFight(t), "Pensantus")
	c.mustMark(t, "Ogro", "controlled")
	c.sitOn(t, "Goblin", "Ogro") // an NPC rider: its save is rolled by the master with the queue
	c.h.roller.queue(5, 5, 5, 5) // every save of the cube fails
	c.mustCastArea(t, c.ana, "Pensantus", greaseKey, slotOfLevel(1), at(7, 7), nil)
	if !c.isProne(t, "Goblin") {
		t.Fatal("the rider in the grease is not prone: the setup does not test the trigger")
	}
	wantSave(t, "after Grease", c.mountSaveOf(t, c.master), c.id(t, "Goblin"), c.id(t, "Ogro"), causeProne)
}

// A rider already prone is not knocked prone again, and a creature that is not a rider asks nothing.
func TestMountedCombat_ProneOnFootAsksNothing(t *testing.T) {
	t.Parallel()
	c := greaseCave(t)
	c.passTo(t, c.zoneFight(t), "Pensantus")
	c.h.roller.queue(5, 5, 5, 5)
	c.mustCastArea(t, c.ana, "Pensantus", greaseKey, slotOfLevel(1), at(7, 7), nil)
	if !c.isProne(t, "Ogro") {
		t.Fatal("the ogre in the grease is not prone: the setup does not test the trigger")
	}
	if got := c.mountSaveOf(t, c.master); got != nil {
		t.Errorf("a creature on foot knocked prone asked %v, want no save", got)
	}
}

// The window holds the turn like a concentration save, for the master too: a move, an action, the end of the turn, a mount and a
// dismount are refused with MOUNT_SAVE_PENDING, and nothing is refused once it is answered. One window for each rider.
func TestMountedCombat_TheWindowHoldsTheTurn(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.ridden(t)
	c.askSave(t, "Toren", causePushed)
	c.askSave(t, "Toren", causeProne) // one pending save for each rider
	enc := c.get(t, c.master)
	if n := len(c.openSaveWindows(t)); n != 1 {
		t.Fatalf("open mount_save windows = %d, want 1 for the rider", n)
	}

	_, err := c.move(t, c.caio, "Ogro", 6, 7)
	wantBlockedBy(t, "the mount moving", err, reasonMountSavePending)
	_, err = c.action(t, c.caio, enc, "Ogro", "standard:dash")
	wantBlockedBy(t, "the mount's Dash", err, reasonMountSavePending)
	_, err = c.dismountAs(t, c.caio, "Toren", 6, 7)
	wantBlockedBy(t, "the dismount", err, reasonMountSavePending)
	_, err = c.endTurn(t, c.caio, enc, false)
	wantBlockedBy(t, "the end of the turn", err, reasonMountSavePending)
	_, err = c.endTurn(t, c.master, enc, false)
	wantBlockedBy(t, "the end of the turn by the master", err, reasonMountSavePending)
}

// openSaveWindows are the rows of the open windows of the kind.
func (c *cave) openSaveWindows(t *testing.T) []string {
	t.Helper()
	rows, err := c.h.pool.Query(t.Context(), `SELECT id::text FROM reaction_windows WHERE kind = 'mount_save' AND status = 'open'`)
	if err != nil {
		t.Fatalf("list the mount_save windows: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, id)
	}
	return out
}

// The window clears when what it asked is gone: the mount removed from the combat, the rider removed or off the mount, the
// pushed mount back where it was (the undo of the push) or the prone taken off.
func TestMountedCombat_TheWindowClearsWhenItsPairIsGone(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		cause playv1.MountSaveCause
		clear func(t *testing.T, c *cave)
	}{
		{"the rider gets off by the master's row", causePushed, func(t *testing.T, c *cave) {
			t.Helper()
			c.execSQL(t, `UPDATE combatants SET mounted_on_id = NULL WHERE id = $1`, c.id(t, "Toren"))
			c.touchCombat(t)
		}},
		{"the mount is removed", causePushed, func(t *testing.T, c *cave) { t.Helper(); c.removeFromCombat(t, "Ogro") }},
		{"the rider leaves", causePushed, func(t *testing.T, c *cave) {
			t.Helper()
			c.execSQL(t, `DELETE FROM combatants WHERE id = $1`, c.id(t, "Toren"))
			c.touchCombat(t)
		}},
		{"the prone is taken off", causeProne, func(t *testing.T, c *cave) {
			t.Helper()
			c.execSQL(t, `UPDATE combatants SET conditions = '{}' WHERE id = $1`, c.id(t, "Toren"))
			c.touchCombat(t)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newCave(t)
			c.ridden(t)
			if tt.cause == causeProne {
				c.execSQL(t, `UPDATE combatants SET conditions = ARRAY['condition:prone'] WHERE id = $1`, c.id(t, "Toren"))
			}
			c.askSave(t, "Toren", tt.cause)
			if c.mountSaveOf(t, c.master) == nil {
				t.Fatal("the positive control failed: the window is not open")
			}
			tt.clear(t, c)
			if got := c.mountSaveOf(t, c.master); got != nil {
				t.Errorf("the window after %s = %v, want it cleared", tt.name, got)
			}
			if n := len(c.openSaveWindows(t)); n != 0 {
				t.Errorf("open mount_save windows = %d, want 0", n)
			}
		})
	}
}

// touchCombat makes the combat settle its windows, as any change does: a master's note on a combatant that changes nothing else.
func (c *cave) touchCombat(t *testing.T) {
	t.Helper()
	c.mustMark(t, "Capitão Goblin", "")
}

// removeFromCombat takes the combatant out of the combat as the master does.
func (c *cave) removeFromCombat(t *testing.T, label string) {
	t.Helper()
	if _, err := c.master.combat.RemoveCombatant(t.Context(), connect.NewRequest(&playv1.RemoveCombatantRequest{
		CampaignId: c.campaignID, EncounterId: c.get(t, c.master).GetId(), CombatantId: c.id(t, label), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("RemoveCombatant(%s) error = %v", label, err)
	}
}

// A steed that is dismissed (the concentration of its spell ends) takes the window of the rider it carried with it.
func TestMountedCombat_ADismissedMountTakesTheWindowWithIt(t *testing.T) {
	t.Parallel()
	a := newSummoners(t)
	e := a.toSalvia(t, a.summonersFight(t))
	if _, err := a.summonWolves(t, a.bia, e, 11); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	a.execSQL(t, `UPDATE combatants SET mounted_on_id = $1, grid_col = m.grid_col, grid_row = m.grid_row FROM combatants m WHERE m.id = $1 AND combatants.id = $2`, a.id(t, "Lobo atroz 1"), a.id(t, "Toren"))
	a.askSave(t, "Toren", causePushed)
	if a.mountSaveOf(t, a.master) == nil {
		t.Fatal("the positive control failed: the window is not open")
	}
	if _, err := a.conditions(t, a.bia, e, "Sálvia", nil, false, true); err != nil {
		t.Fatalf("SetCombatantConditions(end the concentration) error = %v", err)
	}
	if got := a.mountSaveOf(t, a.master); got != nil {
		t.Errorf("the window after the steed was dismissed = %v, want it cleared", got)
	}
	// A steed the caster dismisses lets the rider land on its feet with no window.
	var on *string
	var conds string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT mounted_on_id::text, COALESCE(array_to_string(conditions, ','), '') FROM combatants WHERE id = $1`, a.id(t, "Toren")).Scan(&on, &conds); err != nil {
		t.Fatalf("read the rider: %v", err)
	}
	if on != nil || strings.Contains(conds, "prone") {
		t.Errorf("the rider after the dismissal rides %v with conditions %q, want landed on its feet", on, conds)
	}
	if a.windowOf(t, a.master, reactionDismount) != nil {
		t.Error("the dismissal opened a dismount window, want none")
	}
}
