package play

import (
	"strings"
	"testing"

	connect "connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// The review of the dismount window: the creature steed, the two openings of one mount, the sources of the prone, the fall's line and its
// stream, the gate, and the opportunity attack on a pair.

// wolfRide is the summoners' fight with Toren on one of Sálvia's wolves, in the table: a creature steed.
func wolfRide(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newSummoners(t)
	e := a.toSalvia(t, a.summonersFight(t))
	if _, err := a.summonWolves(t, a.bia, e, 11); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	a.execSQL(t, `UPDATE combatants SET mounted_on_id = $1, grid_col = m.grid_col, grid_row = m.grid_row FROM combatants m WHERE m.id = $1 AND combatants.id = $2`, a.id(t, "Lobo atroz 1"), a.id(t, "Toren"))
	return a, e
}

func (a *armed) mountedOn(t *testing.T, label string) *string {
	t.Helper()
	var on *string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT mounted_on_id::text FROM combatants WHERE id = $1`, a.id(t, label)).Scan(&on); err != nil {
		t.Fatalf("read the rider %s: %v", label, err)
	}
	return on
}

// A creature steed that is killed under its rider is a defeated mount first: the rider's window opens, the steed stays linked until it
// is answered, and then it disappears. Only a steed the caster dismisses lets the rider land with no window.
func TestMountedCombat_ADefeatedCreatureSteedAsksItsRiderBeforeItGoes(t *testing.T) {
	t.Parallel()
	a, e := wolfRide(t)
	if _, err := a.adjustHP(t, e, "Lobo atroz 1", damageHP(500)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	w := a.windowOf(t, a.master, reactionDismount)
	if w == nil {
		t.Fatal("the master reads no dismount window for the killed steed")
	}
	if a.mountedOn(t, "Toren") == nil {
		t.Fatal("the rider was landed before the window was answered")
	}
	if a.windowOf(t, a.bia, reactionDismount) != nil {
		t.Error("the steed's owner reads the window of a rider that is not hers")
	}
	a.mustAnswer(t, a.master, a.get(t, a.master), w.GetId(), useAnswer(0))
	if on := a.mountedOn(t, "Toren"); on != nil {
		t.Errorf("the rider after the answer rides %v, want it landed", *on)
	}
	var dismissed bool
	if err := a.h.pool.QueryRow(t.Context(), `SELECT dismissed FROM combatants WHERE id = $1`, a.id(t, "Lobo atroz 1")).Scan(&dismissed); err != nil || !dismissed {
		t.Errorf("the steed after the answer: dismissed = %v (%v), want it gone", dismissed, err)
	}
}

// Both orders of the two openings: the window of a prone mount that is then defeated, and the window of a defeated mount that is
// then marked prone, never leave the rider mounted with no question.
func TestMountedCombat_AProneMountThatIsThenDefeatedStillAsksItsRider(t *testing.T) {
	t.Parallel()
	c := riddenCave(t)
	e := c.get(t, c.master)
	c.execSQL(t, `UPDATE combatants SET conditions = ARRAY['condition:prone'] WHERE id = $1`, c.id(t, "Ogro"))
	c.askDismount(t, "Toren", mountCauseMountProne)
	if _, err := c.adjustHP(t, e, "Ogro", damageHP(500)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	w := c.windowOf(t, c.caio, reactionDismount)
	if w == nil || c.rideRowOf(t, "Toren").MountedOn == "" {
		t.Fatalf("after the damage: window %v, rider %+v; want the window open and the rider still mounted", w, c.rideRowOf(t, "Toren"))
	}
	c.mustAnswer(t, c.caio, e, w.GetId(), useAnswer(0))
	if c.rideRowOf(t, "Toren").MountedOn != "" {
		t.Error("the rider is still mounted after the answer")
	}
}

func TestMountedCombat_ADefeatedMountThatIsThenMarkedProneKeepsItsWindow(t *testing.T) {
	t.Parallel()
	c, e := defeatedMount(t)
	if c.windowOf(t, c.caio, reactionDismount) == nil {
		t.Fatal("the positive control failed: no window")
	}
	if _, err := c.conditions(t, c.master, e, "Ogro", []string{condProne}, true, false); err != nil {
		t.Fatalf("SetCombatantConditions() error = %v", err)
	}
	if c.windowOf(t, c.caio, reactionDismount) == nil || c.rideRowOf(t, "Toren").MountedOn == "" {
		t.Error("the window of the defeated mount went away with the prone, or the rider was thrown")
	}
}

// Two mounts defeated by one change: each rider has its window.
func TestMountedCombat_TwoDefeatedMountsAskTheirTwoRiders(t *testing.T) {
	t.Parallel()
	c := riddenCave(t)
	c.sitOn(t, "Goblin 1", "Goblin 2")
	c.execSQL(t, `UPDATE combatants SET defeated = true, hp_current = 0 WHERE id = ANY($1)`, []string{c.id(t, "Ogro"), c.id(t, "Goblin 2")})
	c.touchCombat(t)
	var n int
	if err := c.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM reaction_windows WHERE kind = 'dismount' AND status = 'open'`).Scan(&n); err != nil || n != 2 {
		t.Errorf("open dismount windows = %d (%v), want 2", n, err)
	}
}

// The rider that cannot react, because it is surprised, falls at once, as an opportunity attack's reactor does not react.
func TestMountedCombat_ASurprisedRiderFallsAtOnce(t *testing.T) {
	t.Parallel()
	c := riddenCave(t)
	c.execSQL(t, `INSERT INTO combat_surprised (combatant_id, encounter_id, created_at) SELECT id, encounter_id, now() FROM combatants WHERE id = $1`, c.id(t, "Toren"))
	if _, err := c.adjustHP(t, c.get(t, c.master), "Ogro", damageHP(500)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	if c.windowOf(t, c.master, reactionDismount) != nil || c.rideRowOf(t, "Toren").MountedOn != "" {
		t.Error("a surprised rider was asked, or stays on the mount")
	}
}

// The master answers for a creature rider; its owner is not asked and cannot answer.
func TestMountedCombat_ACreatureRidersOwnerIsNotAsked(t *testing.T) {
	t.Parallel()
	a, e := wolfRide(t)
	a.execSQL(t, `UPDATE combatants SET mounted_on_id = NULL WHERE id = $1`, a.id(t, "Toren"))
	a.execSQL(t, `UPDATE combatants SET mounted_on_id = $1, grid_col = m.grid_col, grid_row = m.grid_row FROM combatants m WHERE m.id = $1 AND combatants.id = $2`, a.id(t, "Toren"), a.id(t, "Lobo atroz 1"))
	a.execSQL(t, `UPDATE combatants SET conditions = ARRAY['condition:prone'] WHERE id = $1`, a.id(t, "Toren"))
	a.askDismount(t, "Lobo atroz 1", mountCauseMountProne)
	w := a.windowOf(t, a.master, reactionDismount)
	if w == nil {
		t.Fatal("the master reads no window for the creature rider")
	}
	if a.windowOf(t, a.bia, reactionDismount) != nil {
		t.Error("the creature's owner reads the window")
	}
	if _, err := a.answerReaction(t, a.bia, e, w.GetId(), useAnswer(0)); err == nil {
		t.Error("the creature's owner answered its window")
	}
}

// Where the prone of a mount comes from: a spell's condition and a trap, as the rider's own prone does.
func TestMountedCombat_AGreaseThatKnocksTheMountProneAsksItsRider(t *testing.T) {
	t.Parallel()
	c := greaseCave(t)
	c.passTo(t, c.zoneFight(t), "Pensantus")
	c.mustMark(t, "Ogro", "controlled")
	c.sitOn(t, "Goblin", "Ogro")
	c.h.roller.queue(5, 5, 5, 5)
	c.mustCastArea(t, c.ana, "Pensantus", greaseKey, slotOfLevel(1), at(7, 7), nil)
	if !c.isProne(t, "Ogro") {
		t.Fatal("the mount in the grease is not prone: the setup does not test the trigger")
	}
	if c.windowOf(t, c.master, reactionDismount) == nil {
		t.Error("the master reads no dismount window after the grease")
	}
}

func TestMountedCombat_ATrapThatKnocksTheMountProneAsksItsRider(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	r.ridden(t)
	p := r.trap(t, "Fosso", 20, 12, manualTrap, func(s *mapsv1.TrapSpec) {
		s.Effect = &rulesv1.TrapEffect{Conditions: []*rulesv1.TrapCondition{{ConditionKey: "condition:prone"}}}
	})
	if _, err := r.fireRaw(t, p, func(q *playv1.FireTrapRequest) { q.TargetIds = []string{r.id(t, "Ogro")} }); err != nil {
		t.Fatalf("FireTrap() error = %v", err)
	}
	if r.windowOf(t, r.caio, reactionDismount) == nil {
		t.Error("Toren's player reads no dismount window after the trap knocked the Ogro prone")
	}
}

// The windows' gate: a rider's save wins over a plain reaction for who sees both, and the dismount alone is REACTION_PENDING.
func TestMountedCombat_TheRidersSaveWinsOverTheDismountAtTheGate(t *testing.T) {
	t.Parallel()
	c, e := defeatedMount(t)
	_, err := c.endTurn(t, c.master, e, false)
	wantBlockedBy(t, "the dismount alone", err, reasonReactionPending)
	c.askSave(t, "Toren", causePushed)
	_, err = c.endTurn(t, c.master, c.get(t, c.master), false)
	wantBlockedBy(t, "the save and the dismount", err, reasonMountSavePending)
}

// The landing and the fall reach the streams and the fog, as the fall of a failed save does.
func TestMountedCombat_ALandingIsToldToTheFog(t *testing.T) {
	t.Parallel()
	for _, answer := range []struct {
		name string
		set  func(*playv1.AnswerReactionRequest)
	}{{"landing", useAnswer(0)}, {"fall", passAnswer}} {
		t.Run(answer.name, func(t *testing.T) {
			t.Parallel()
			c, e := defeatedMount(t)
			fog := &dragFogRecorder{}
			c.h.svc.SetFog(fog)
			w := c.windowOf(t, c.caio, reactionDismount)
			if w == nil {
				t.Fatal("no window")
			}
			c.mustAnswer(t, c.caio, e, w.GetId(), answer.set)
			now := c.rideRowOf(t, "Toren")
			if !fog.told(grid.Square{Col: 7, Row: 7}, grid.Square{Col: int(now.Col), Row: int(now.Row)}) {
				t.Errorf("the fog was not told Toren moved from (7,7) to (%d,%d)", now.Col, now.Row)
			}
		})
	}
	t.Run("a rider that falls at once", func(t *testing.T) {
		t.Parallel()
		c := riddenCave(t)
		fog := &dragFogRecorder{}
		c.h.svc.SetFog(fog)
		c.execSQL(t, `UPDATE combatants SET reaction_used = true WHERE id = $1`, c.id(t, "Toren"))
		if _, err := c.adjustHP(t, c.get(t, c.master), "Ogro", damageHP(500)); err != nil {
			t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
		}
		now := c.rideRowOf(t, "Toren")
		if !fog.told(grid.Square{Col: 7, Row: 7}, grid.Square{Col: int(now.Col), Row: int(now.Row)}) {
			t.Errorf("the fog was not told Toren fell from (7,7) to (%d,%d)", now.Col, now.Row)
		}
	})
}

// A fall from a dismount is not a failed save: no save is written (no d20, no DC, no automatic failure) and the cause says none was asked.
func TestMountedCombat_ADismountFallIsNoFailedSave(t *testing.T) {
	t.Parallel()
	c, e := defeatedMount(t)
	w := c.windowOf(t, c.caio, reactionDismount)
	c.mustAnswer(t, c.caio, e, w.GetId(), passAnswer)
	for who, u := range map[string]*user{"the master": c.master, "Toren's player": c.caio} {
		ls := c.linesOfKind(t, u, playv1.CombatLogKind_COMBAT_LOG_KIND_RIDER_THROWN)
		if len(ls) != 1 {
			t.Fatalf("%s reads %d fall lines, want 1", who, len(ls))
		}
		ms := ls[0].GetMountSave()
		if ms.GetAutoFail() || ms.GetD20() != 0 || ms.GetDc() != 0 || ms.GetCause() != playv1.MountSaveCause_MOUNT_SAVE_CAUSE_UNSPECIFIED {
			t.Errorf("%s reads the fall %v, want no save in it", who, ms)
		}
	}
}

// The opportunity attack on a pair, the sides that review found.
func TestMountedCombat_AHiddenMountIsNoTargetOfAPlayerOpportunityAttack(t *testing.T) {
	t.Parallel()
	c := enemyPair(t)
	c.execSQL(t, `UPDATE combatants SET hidden = true WHERE id = $1`, c.id(t, "Ogro"))
	if _, err := c.move(t, c.master, "Ogro", 9, 7); err != nil {
		t.Fatalf("MoveCombatant(mount) error = %v", err)
	}
	var offer *playv1.OpportunityOffer
	for _, o := range c.offersOf(t, c.caio) {
		if o.GetReactorId() == c.id(t, "Toren") {
			offer = o
		}
	}
	if offer == nil {
		t.Fatal("the positive control failed: Toren's player reads no offer")
	}
	_, err := c.offerAttack(t, c.caio, "Toren", "equipment:battleaxe", "Ogro", offer.GetId(), d20(15))
	wantCode(t, "an attack on the hidden mount", err, connect.CodeNotFound)
}

func TestMountedCombat_AnOpportunityAttackOnARiderThatLandedIsRefused(t *testing.T) {
	t.Parallel()
	c, offer := mountLeaves(t)
	c.execSQL(t, `UPDATE combatants SET mounted_on_id = NULL WHERE id = $1`, c.id(t, "Toren"))
	_, err := c.offerAttack(t, c.master, "Goblin 1", "basic:0", "Toren", offer, d20(15))
	wantCode(t, "an attack on a rider that is off the mount", err, connect.CodeInvalidArgument)
}

func TestMountedCombat_TheUndoOfAnOpportunityAttackOnTheRiderGivesTheOfferBack(t *testing.T) {
	t.Parallel()
	c, offer := mountLeaves(t)
	if _, err := c.offerAttack(t, c.master, "Goblin 1", "basic:0", "Toren", offer, d20(15)); err != nil {
		t.Fatalf("RollAttack(rider) error = %v", err)
	}
	if len(c.offersOf(t, c.master)) != 0 {
		t.Fatal("the offer was not answered by the attack")
	}
	c.undoLast(t)
	if got := c.offersOf(t, c.master); len(got) != 1 || got[0].GetId() != offer {
		t.Errorf("offers after the undo = %v, want the offer back", got)
	}
}

// The opportunity window and the wait for a reactor that does not see the hidden mount are the rider's.
func TestMountedCombat_TheOfferWindowOfAHiddenMountReadsAsTheRiders(t *testing.T) {
	t.Parallel()
	c := enemyPair(t)
	c.execSQL(t, `UPDATE combatants SET label = $1, hidden = true WHERE id = $2`, saveCanary, c.id(t, "Ogro"))
	if _, err := c.move(t, c.master, saveCanary, 9, 7); err != nil {
		t.Fatalf("MoveCombatant(mount) error = %v", err)
	}
	var found bool
	for _, w := range c.windows(t, c.caio) {
		if w.GetKind() == playv1.ReactionKind_REACTION_KIND_OPPORTUNITY && w.GetReactorId() == c.id(t, "Toren") {
			found = true
		}
	}
	if !found {
		t.Error("Toren's player reads no opportunity window for the pair that left his reach")
	}
	if strings.Contains(c.copyOf(t, c.caio), saveCanary) {
		t.Error("Toren's player reads the hidden mount")
	}
}
