package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// mountOptionsOf is TurnOptions.mount of the combatant as u reads it.
func (c *cave) mountOptionsOf(t *testing.T, u *user, label string) *rulesv1.MountOptions {
	t.Helper()
	res, err := u.combat.GetTurnOptions(t.Context(), connect.NewRequest(&playv1.GetTurnOptionsRequest{CampaignId: c.campaignID, EncounterId: c.get(t, c.master).GetId(), CombatantId: c.id(t, label)}))
	if err != nil {
		t.Fatalf("GetTurnOptions(%s) error = %v", label, err)
	}
	return res.Msg.GetOptions().GetMount()
}

// offered says whether the mount options list the combatant as a mount.
func (c *cave) offered(t *testing.T, o *rulesv1.MountOptions, label string) *rulesv1.MountChoice {
	t.Helper()
	for _, m := range o.GetMounts() {
		if m.GetCombatantId() == c.id(t, label) {
			return m
		}
	}
	return nil
}

// TestMountedCombat_AHiddenRiderOnAVisibleMountLeaksNothing: a player who tries to mount a pony that a hidden creature rides, or whose
// own mount rides a hidden creature, is answered as if the pair did not exist: the same refusal as for a combatant that is no mount,
// no id of the hidden one, and the options do not list the mount. The master reads the real refusal (the positive control).
func TestMountedCombat_AHiddenRiderOnAVisibleMountLeaksNothing(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		setup  func(t *testing.T, c *cave) // the hidden creature rides the Ogro, or the Ogro rides it
		reason playv1.EncounterBlockedReason
	}{
		"a hidden rider on the mount": {func(t *testing.T, c *cave) { c.sitOn(t, "Goblin 1", "Ogro") }, reasonMountTaken},
		"a mount that rides a hidden mount": {func(t *testing.T, c *cave) {
			c.mustMove(t, c.master, "Goblin 1", 8, 7)
			c.sitOn(t, "Ogro", "Goblin 1")
		}, reasonAlreadyRiding},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := newCave(t)
			c.fight(t)
			c.steed(t)
			tc.setup(t, c)
			c.hide(t, "Goblin 1")
			hidden := c.id(t, "Goblin 1")

			_, err := c.mountAs(t, c.master, "Toren", "Ogro")
			b := wantBlockedBy(t, "the master mounting the pair's mount", err, tc.reason)
			if len(b.GetCombatantIds()) != 1 {
				t.Errorf("the master's refusal names %v, want one combatant", b.GetCombatantIds())
			}
			if m := c.offered(t, c.mountOptionsOf(t, c.master, "Toren"), "Ogro"); m == nil || m.GetRefusal() == rulesv1.MountRefusal_MOUNT_REFUSAL_UNSPECIFIED {
				t.Errorf("the master's options list the Ogro as %v, want it with a refusal", m)
			}

			_, err = c.mountAs(t, c.caio, "Toren", "Ogro")
			b = wantBlockedBy(t, "a player mounting the pair's mount", err, reasonNotAMount)
			for _, id := range b.GetCombatantIds() {
				if id == hidden {
					t.Errorf("the refusal names the hidden combatant %s", id)
				}
			}
			if m := c.offered(t, c.mountOptionsOf(t, c.caio, "Toren"), "Ogro"); m != nil {
				t.Errorf("a player's options list the Ogro (%v) while a hidden creature is in its pair", m)
			}
		})
	}
}

// TestMountedCombat_ADismissedMountLetsItsRiderStandOnItsOwn: the steed of a spell that ends is dismissed from the combat, and the
// rider it carried is on foot where it stood, not stranded on a mount that is not there; the master's undo of the end seats it again.
func TestMountedCombat_ADismissedMountLetsItsRiderStandOnItsOwn(t *testing.T) {
	t.Parallel()
	a := newSummoners(t)
	e := a.toSalvia(t, a.summonersFight(t))
	if _, err := a.summonWolves(t, a.bia, e, 11); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	wolf := a.id(t, "Lobo atroz 1")
	a.execSQL(t, `UPDATE combatants SET mounted_on_id = $1, grid_col = m.grid_col, grid_row = m.grid_row FROM combatants m WHERE m.id = $1 AND combatants.id = $2`, wolf, a.id(t, "Toren"))
	linked := func() *string {
		var on *string
		if err := a.h.pool.QueryRow(t.Context(), `SELECT mounted_on_id::text FROM combatants WHERE id = $1`, a.id(t, "Toren")).Scan(&on); err != nil {
			t.Fatalf("read Toren's link: %v", err)
		}
		return on
	}
	if on := linked(); on == nil || *on != wolf {
		t.Fatalf("the positive control failed: Toren rides %v, want the wolf", on)
	}
	if _, err := a.conditions(t, a.bia, e, "Sálvia", nil, false, true); err != nil {
		t.Fatalf("SetCombatantConditions(end the concentration) error = %v", err)
	}
	if on := linked(); on != nil {
		t.Errorf("Toren rides %s after the wolf was dismissed, want nothing", *on)
	}
	e = a.get(t, a.master)
	if err := a.undo(t, a.master, e, a.log(t, a.master, e).GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	if on := linked(); on == nil || *on != wolf {
		t.Errorf("Toren rides %v after the undo, want the wolf again", on)
	}
}

// TestMountedCombat_MoveOptionsAgreeWithTheMove: GetMoveOptions answers as MoveCombatant does. The rider's player gets the options of
// the controlled mount they drive and another player does not; a rider gets MOUNTED, or MOVE_BLOCKED when the mount is hidden from
// the caller, which names nothing.
func TestMountedCombat_MoveOptionsAgreeWithTheMove(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.fight(t)
	c.steed(t)
	c.mustMount(t, c.caio, "Toren", "Ogro")

	res, err := c.options(t, c.caio, "Ogro")
	if err != nil {
		t.Fatalf("the rider's player asking the options of the mount they drive = %v", err)
	}
	if res.GetMovementLeftDft() != 300 {
		t.Errorf("the options of the mount say %d left, want its own 300", res.GetMovementLeftDft())
	}
	_, err = c.options(t, c.ana, "Ogro")
	wantCode(t, "another player asking the options of the steed", err, connect.CodePermissionDenied)
	_, err = c.options(t, c.caio, "Toren")
	wantBlockedBy(t, "the options of a rider", err, reasonMounted)
	_, err = c.options(t, c.master, "Toren")
	wantBlockedBy(t, "the master's options of a rider", err, reasonMounted)

	c.nextTurnOf(t, "Pensantus") // the rider's turn is over: the mount's part of it too
	_, err = c.options(t, c.caio, "Ogro")
	wantBlockedBy(t, "the options of the mount out of its rider's turn", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_YOUR_TURN)

	c.nextTurnOf(t, "Toren")
	c.hide(t, "Ogro")
	_, err = c.options(t, c.caio, "Toren")
	wantBlockedBy(t, "the options of a rider on a hidden mount", err, reasonMoveBlocked)
}

// TestMountedCombat_ARiderIsNoFoeOfItsOwnMount: the master seats Toren, of the party, on an Ogro of the enemy side; when the Ogro
// leaves a reach, Toren, who rides it, is not offered an attack on it, while Brisa, of the party and next to it, is.
func TestMountedCombat_ARiderIsNoFoeOfItsOwnMount(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.fight(t)
	c.mustMove(t, c.master, "Ogro", 7, 7)
	c.mustMove(t, c.master, "Brisa", 6, 8)
	c.mustMount(t, c.master, "Toren", "Ogro")
	c.passTo(t, c.get(t, c.master), "Ogro")
	c.moveOffering(t, "Ogro", 10, 7)
	var reactors []string
	for _, o := range c.offersOf(t, c.master) {
		reactors = append(reactors, o.GetReactorLabel())
	}
	if len(reactors) != 1 || reactors[0] != "Brisa" {
		t.Errorf("the offers on the mount's move are from %v, want Brisa alone (Toren rides it)", reactors)
	}
}

// TestMountedCombat_TheUndoOfTheMountsMoveGivesTheRidersCoverMarkBack: the move of the mount clears the master's cover mark of its
// rider, who changes square with it, and the undo of the move puts both marks back.
func TestMountedCombat_TheUndoOfTheMountsMoveGivesTheRidersCoverMarkBack(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.fight(t)
	c.steed(t)
	c.mustMount(t, c.caio, "Toren", "Ogro")
	c.mark(t, "Toren", playv1.CoverDegree_COVER_DEGREE_HALF)
	c.mark(t, "Ogro", playv1.CoverDegree_COVER_DEGREE_THREE_QUARTERS)
	markOf := func(label string) string {
		var mark string
		if err := c.h.pool.QueryRow(t.Context(), `SELECT cover_mark FROM combatants WHERE id = $1`, c.id(t, label)).Scan(&mark); err != nil {
			t.Fatalf("read the cover mark of %s: %v", label, err)
		}
		return mark
	}
	if _, err := c.move(t, c.caio, "Ogro", 10, 7); err != nil {
		t.Fatalf("the mount's move = %v", err)
	}
	if got := markOf("Toren"); got != "none" {
		t.Fatalf("the rider's mark after the mount's move = %q, want it cleared", got)
	}
	c.undoLast(t)
	if got := markOf("Ogro"); got != "three_quarters" {
		t.Errorf("the mount's mark after the undo = %q, want three_quarters", got)
	}
	if got := markOf("Toren"); got != "half" {
		t.Errorf("the rider's mark after the undo = %q, want half", got)
	}
}
