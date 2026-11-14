package play

import (
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"slices"
	"strings"
	"testing"

	connect "connectrpc.com/connect"
)

// An opportunity attack on a mount (SRD 5.1, "Mounted Combat"): when the mount provokes, the reactor may hit the mount or its rider. The
// offer is the mount's; the attack names either of the pair, nobody else, and a player who does not see the mount sees only the rider.

// mountLeaves is the ridden cave with the Ogro walked out of Goblin 1's reach by Toren's player: Goblin 1's offer waits, with the Ogro as
// its mover.
func mountLeaves(t *testing.T) (*cave, string) {
	t.Helper()
	c := riddenCave(t)
	if _, err := c.move(t, c.caio, "Ogro", 5, 7); err != nil {
		t.Fatalf("MoveCombatant(Ogro) error = %v", err)
	}
	offers := c.offersOf(t, c.master)
	if len(offers) != 1 || offers[0].GetMoverId() != c.id(t, "Ogro") {
		t.Fatalf("the master reads offers %v, want one with the Ogro as mover", offers)
	}
	return c, offers[0].GetId()
}

func TestMountedCombat_TheReactorMayHitTheMountOrItsRider(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"Ogro", "Toren"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			c, offer := mountLeaves(t)
			if _, err := c.offerAttack(t, c.master, "Goblin 1", "basic:0", target, offer, d20(15)); err != nil {
				t.Fatalf("RollAttack(%s) on the offer error = %v", target, err)
			}
			if got := c.offersOf(t, c.master); len(got) != 0 {
				t.Errorf("offers after the attack = %v, want it answered", got)
			}
		})
	}
	c, offer := mountLeaves(t)
	_, err := c.offerAttack(t, c.master, "Goblin 1", "basic:0", "Pensantus", offer, d20(15))
	wantCode(t, "an opportunity attack on somebody out of the pair", err, connect.CodeInvalidArgument)
}

func TestMountedCombat_TheTurnOptionsOfTheReactorListTheMountAndItsRider(t *testing.T) {
	t.Parallel()
	c, _ := mountLeaves(t)
	res := c.armed.mustOptions(t, c.master, c.get(t, c.master), "Goblin 1")
	var ids []string
	for _, a := range res.GetAttackTargets() {
		for _, tg := range a.GetTargets() {
			ids = append(ids, tg.GetCombatantId())
		}
	}
	for _, want := range []string{"Ogro", "Toren"} {
		if !slices.Contains(ids, c.id(t, want)) {
			t.Errorf("the targets of Goblin 1 = %v, want %s among them", ids, want)
		}
	}
}

func TestMountedCombat_ThePairsOwnRowsDoNotProvokeEachOther(t *testing.T) {
	t.Parallel()
	c := riddenCave(t)
	// The control: a foe in reach offers an attack when the mount walks away from it.
	if _, err := c.move(t, c.caio, "Ogro", 5, 7); err != nil {
		t.Fatalf("MoveCombatant(Ogro) error = %v", err)
	}
	for _, o := range c.offersOf(t, c.master) {
		if o.GetReactorId() == c.id(t, "Toren") || o.GetReactorId() == c.id(t, "Ogro") {
			t.Errorf("offer %v: one of the pair is the reactor against the other", o)
		}
	}
	if len(c.offersOf(t, c.master)) != 1 {
		t.Errorf("offers = %v, want the one of Goblin 1", c.offersOf(t, c.master))
	}
}

// A player's reactor that does not see the mount: the offer reads as the rider's, and no player reads the mount's name.
func TestMountedCombat_AReactorThatDoesNotSeeTheMountSeesOnlyTheRider(t *testing.T) {
	t.Parallel()
	c := riddenCave(t)
	c.execSQL(t, `UPDATE combatants SET label = $1 WHERE id = $2`, saveCanary, c.id(t, "Ogro"))
	if _, err := c.move(t, c.caio, saveCanary, 5, 7); err != nil {
		t.Fatalf("MoveCombatant(mount) error = %v", err)
	}
	if len(c.offersOf(t, c.master)) != 1 {
		t.Fatalf("the positive control failed: the master reads %v", c.offersOf(t, c.master))
	}
	c.execSQL(t, `UPDATE combatants SET hidden = true WHERE id = $1`, c.id(t, saveCanary))
	for who, u := range map[string]*user{"another player": c.ana, "a third player": c.bia} {
		if strings.Contains(c.copyOf(t, u)+c.logOf(t, u), saveCanary) {
			t.Errorf("%s reads the hidden mount", who)
		}
	}
}

// enemyPair is Goblin 1 on the Ogro, both foes of the party, next to Toren on the Ogro's turn: the master walks the Ogro away.
func enemyPair(t *testing.T) *cave {
	t.Helper()
	c := newCave(t)
	e := c.fight(t)
	c.mustMove(t, c.master, "Ogro", 7, 7)
	c.mustMove(t, c.master, "Goblin 1", 8, 8)
	c.sitOn(t, "Goblin 1", "Ogro")
	c.passTo(t, e, "Ogro")
	return c
}

func TestMountedCombat_APlayerReactorSeesTheEnemyMountOrOnlyItsRider(t *testing.T) {
	t.Parallel()
	c := enemyPair(t)
	c.execSQL(t, `UPDATE combatants SET label = $1 WHERE id = $2`, saveCanary, c.id(t, "Ogro"))
	if _, err := c.move(t, c.master, saveCanary, 9, 7); err != nil {
		t.Fatalf("MoveCombatant(mount) error = %v", err)
	}
	var toren *playv1.OpportunityOffer
	for _, o := range c.offersOf(t, c.caio) {
		if o.GetReactorId() == c.id(t, "Toren") {
			toren = o
		}
	}
	if toren == nil || toren.GetMoverId() != c.id(t, saveCanary) || !strings.Contains(c.copyOf(t, c.caio), saveCanary) {
		t.Fatalf("the positive control failed: Toren's player reads %v", c.offersOf(t, c.caio))
	}
}

func TestMountedCombat_AHiddenMountLeavesOnlyTheRiderToTheReactor(t *testing.T) {
	t.Parallel()
	c := enemyPair(t)
	c.execSQL(t, `UPDATE combatants SET label = $1, hidden = true WHERE id = $2`, saveCanary, c.id(t, "Ogro"))
	if _, err := c.move(t, c.master, saveCanary, 9, 7); err != nil {
		t.Fatalf("MoveCombatant(mount) error = %v", err)
	}
	var toren *playv1.OpportunityOffer
	for _, o := range c.offersOf(t, c.caio) {
		if o.GetReactorId() == c.id(t, "Toren") {
			toren = o
		}
	}
	if toren == nil || toren.GetMoverId() != c.id(t, "Goblin 1") {
		t.Fatalf("Toren's player reads %v, want the offer on the rider Goblin 1", c.offersOf(t, c.caio))
	}
	for who, u := range map[string]*user{"Toren's player": c.caio, "another player": c.ana} {
		if strings.Contains(c.copyOf(t, u)+c.logOf(t, u), saveCanary) {
			t.Errorf("%s reads the hidden mount", who)
		}
	}
	// The rider is a target of the offer, the hidden mount is not one the player names: the attack on the rider is allowed.
	if _, err := c.offerAttack(t, c.caio, "Toren", "equipment:battleaxe", "Goblin 1", toren.GetId(), d20(15)); err != nil {
		t.Errorf("RollAttack(rider) on the offer of its hidden mount error = %v", err)
	}
	if !strings.Contains(c.copyOf(t, c.master), saveCanary) {
		t.Error("the master does not read the mount")
	}
}
