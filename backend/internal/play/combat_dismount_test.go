package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
)

// The dismount window (SRD 5.1, "Mounted Combat"): a mount knocked prone or killed under its rider lets the rider spend its reaction
// to land on its feet within 5 feet, or it falls prone.

var reactionDismount = playv1.ReactionKind_REACTION_KIND_DISMOUNT

// askDismount opens the window through the function the triggers call.
func (c *armed) askDismount(t *testing.T, rider, cause string) {
	t.Helper()
	m := authz.Membership{CampaignID: c.campaignID, UserID: c.master.id, Role: authz.RoleMaster}
	encounterID, riderID := c.get(t, c.master).GetId(), c.id(t, rider)
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
		if err := c.h.svc.openDismount(t.Context(), cx, cs, who, *mount, cause); err != nil {
			return nil, err
		}
		return actionEvent{Round: cx.enc.Round, Actor: who.ID}, nil
	})
	if err != nil {
		t.Fatalf("open the dismount of %s: %v", rider, err)
	}
}

// conditionsOf is what the table keeps of a combatant's conditions.
func (c *cave) conditionsOf(t *testing.T, label string) string {
	t.Helper()
	var s string
	if err := c.h.pool.QueryRow(t.Context(), `SELECT COALESCE(array_to_string(conditions, ','), '') FROM combatants WHERE id = $1`, c.id(t, label)).Scan(&s); err != nil {
		t.Fatalf("read the conditions of %s: %v", label, err)
	}
	return s
}

// defeatedMount is the ridden cave with the Ogro at 0 hit points under Toren.
func defeatedMount(t *testing.T) (*cave, *playv1.Encounter) {
	t.Helper()
	c := riddenCave(t)
	e := c.get(t, c.master)
	if _, err := c.adjustHP(t, e, "Ogro", damageHP(500)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints(Ogro) error = %v", err)
	}
	return c, e
}

func TestMountedCombat_ADefeatedMountOpensTheDismountWindowOfItsRider(t *testing.T) {
	t.Parallel()
	c, e := defeatedMount(t)
	w := c.windowOf(t, c.caio, reactionDismount)
	if w == nil {
		t.Fatal("Toren's player reads no dismount window")
	}
	if c.windowOf(t, c.master, reactionDismount) == nil {
		t.Error("the master reads no dismount window")
	}
	if c.windowOf(t, c.ana, reactionDismount) != nil {
		t.Error("another player reads Toren's window")
	}
	if _, err := c.answerReaction(t, c.ana, e, w.GetId(), useAnswer(0)); err == nil {
		t.Error("another player answered Toren's window")
	}
	_, err := c.endTurn(t, c.master, e, false)
	wantBlockedBy(t, "the end of the turn", err, reasonReactionPending)
	if got := c.rideRowOf(t, "Toren"); got.MountedOn == "" {
		t.Error("Toren was thrown before he answered")
	}
}

func TestMountedCombat_TheReactionLandsTheRiderOnItsFeet(t *testing.T) {
	t.Parallel()
	c, e := defeatedMount(t)
	w := c.windowOf(t, c.caio, reactionDismount)
	if w == nil {
		t.Fatal("no dismount window")
	}
	c.mustAnswer(t, c.caio, e, w.GetId(), useAnswer(0))
	got := c.rideRowOf(t, "Toren")
	if got.MountedOn != "" || (got.Col == 7 && got.Row == 7) || got.Col < 6 || got.Col > 8 || got.Row < 6 || got.Row > 8 {
		t.Errorf("Toren after the landing = %+v, want off the mount, within 5 ft of (7,7)", got)
	}
	if strings.Contains(c.conditionsOf(t, "Toren"), "prone") {
		t.Error("Toren landed prone, want on his feet")
	}
	if c.windowOf(t, c.master, reactionDismount) != nil {
		t.Error("the window is still open")
	}
	if got := c.linesOfKind(t, c.caio, playv1.CombatLogKind_COMBAT_LOG_KIND_RIDER_THROWN); len(got) != 0 {
		t.Errorf("rider thrown lines = %d, want none for a landing on the feet", len(got))
	}
	if err := c.undo(t, c.master, e, c.log(t, c.master, e).GetUndoableEventId()); err == nil && c.rideRowOf(t, "Toren").MountedOn != "" {
		t.Error("the undo put Toren back on the mount: the answer must close the chain")
	}
}

func TestMountedCombat_PassingTheDismountThrowsTheRiderProne(t *testing.T) {
	t.Parallel()
	c, e := defeatedMount(t)
	w := c.windowOf(t, c.caio, reactionDismount)
	if w == nil {
		t.Fatal("no dismount window")
	}
	c.mustAnswer(t, c.caio, e, w.GetId(), passAnswer)
	if got := c.rideRowOf(t, "Toren"); got.MountedOn != "" {
		t.Errorf("Toren after passing = %+v, want off the mount", got)
	}
	if !strings.Contains(c.conditionsOf(t, "Toren"), "prone") {
		t.Error("Toren is not prone")
	}
	if got := c.linesOfKind(t, c.master, playv1.CombatLogKind_COMBAT_LOG_KIND_RIDER_THROWN); len(got) != 1 {
		t.Errorf("rider thrown lines = %d, want 1", len(got))
	}
}

func TestMountedCombat_ARiderWithNoReactionFallsAtOnce(t *testing.T) {
	t.Parallel()
	c := riddenCave(t)
	e := c.get(t, c.master)
	c.execSQL(t, `UPDATE combatants SET reaction_used = true, reaction_kind = 'opportunity', reaction_round = $2 WHERE id = $1`, c.id(t, "Toren"), e.GetRound())
	if _, err := c.adjustHP(t, e, "Ogro", damageHP(500)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints(Ogro) error = %v", err)
	}
	if c.windowOf(t, c.master, reactionDismount) != nil {
		t.Error("a window opened for a rider with no reaction")
	}
	if got := c.rideRowOf(t, "Toren"); got.MountedOn != "" || !strings.Contains(c.conditionsOf(t, "Toren"), "prone") {
		t.Errorf("Toren = %+v %s, want thrown prone at once", got, c.conditionsOf(t, "Toren"))
	}
}

func TestMountedCombat_AProneMountOpensTheDismountWindow(t *testing.T) {
	t.Parallel()
	c := riddenCave(t)
	c.execSQL(t, `UPDATE combatants SET conditions = ARRAY['prone'] WHERE id = $1`, c.id(t, "Ogro"))
	c.askDismount(t, "Toren", mountCauseMountProne)
	if c.windowOf(t, c.caio, reactionDismount) == nil {
		t.Fatal("Toren's player reads no dismount window")
	}
	c.execSQL(t, `UPDATE combatants SET conditions = '{}' WHERE id = $1`, c.id(t, "Ogro"))
	c.touchCombat(t)
	if c.windowOf(t, c.caio, reactionDismount) != nil {
		t.Error("the window outlived the mount's prone")
	}
}

func TestMountedCombat_HealingTheMountClosesTheDismountWindow(t *testing.T) {
	t.Parallel()
	c, e := defeatedMount(t)
	if c.windowOf(t, c.caio, reactionDismount) == nil {
		t.Fatal("the positive control failed: no window")
	}
	if _, err := c.adjustHP(t, e, "Ogro", healHP(10)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints(heal) error = %v", err)
	}
	if c.windowOf(t, c.caio, reactionDismount) != nil {
		t.Error("the window outlived the mount's defeat")
	}
}

func TestMountedCombat_ADismountOfAHiddenMountNamesNothing(t *testing.T) {
	t.Parallel()
	c := riddenCave(t)
	c.execSQL(t, `UPDATE combatants SET label = $1 WHERE id = $2`, saveCanary, c.id(t, "Ogro"))
	e := c.get(t, c.master)
	if _, err := c.adjustHP(t, e, saveCanary, damageHP(500)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	if !strings.Contains(c.copyOf(t, c.caio), saveCanary) {
		t.Fatal("the positive control failed: Toren's player does not read the mount while it is shown")
	}
	c.execSQL(t, `UPDATE combatants SET hidden = true WHERE id = $1`, c.id(t, saveCanary))
	if c.windowOf(t, c.master, reactionDismount) == nil {
		t.Fatal("the positive control failed: the master reads no dismount window")
	}
	for who, u := range map[string]*user{"Toren's player": c.caio, "another player": c.ana} {
		if strings.Contains(c.copyOf(t, u)+c.logOf(t, u), saveCanary) {
			t.Errorf("%s reads the hidden mount", who)
		}
	}
	_, err := c.endTurn(t, c.ana, c.get(t, c.ana), false)
	if err != nil && connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Logf("end turn: %v", err)
	}
}
