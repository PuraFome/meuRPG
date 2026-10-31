package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Forced movement (SRD 5.1, Thunderwave): a creature that fails the save is pushed 10 feet
// away from the caster, in whole squares of the map, stopping before a wall or another
// creature, with no opportunity attack, and the master's undo of the cast puts it back.
// These tests need the database (MEURPG_TEST_DATABASE_URL); the fixture is the cave of
// combat_move_test.go: Pensantus casts Thunderwave eastwards, the cube is the three
// columns next to him.

// waveFight is areaFight with Pensantus at (col, row) and Goblin 1 at goblin.
func (c *cave) waveFight(t *testing.T, caster, goblin [2]int32) {
	t.Helper()
	c.areaFight(t)
	c.mustMove(t, c.master, "Goblin 1", goblin[0], goblin[1])
	c.mustMove(t, c.master, "Pensantus", caster[0], caster[1])
}

func (c *cave) wave(t *testing.T) {
	t.Helper()
	c.mustCastArea(t, c.ana, "Pensantus", thunderwave, slotOfLevel(1), nil, toward(1, 0))
}

func wantSquare(t *testing.T, c *cave, label string, want [2]int32) {
	t.Helper()
	if got := c.square(t, label); got != want {
		t.Errorf("%s stands at %v, want %v", label, got, want)
	}
}

// TestThunderwavePushesAFailedSaveTenFeetAway: Goblin 1 beside the caster fails the save and
// goes two squares east; a goblin outside the cube stays; the undo of the cast brings it back.
func TestThunderwavePushesAFailedSaveTenFeetAway(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.waveFight(t, [2]int32{17, 7}, [2]int32{18, 7})

	c.wave(t)
	wantSquare(t, c, "Goblin 1", [2]int32{20, 7})
	wantSquare(t, c, "Goblin 2", [2]int32{22, 8})
	if tg := c.castTargetsOf(t, c.get(t, c.master))["Goblin 1"]; tg.GetPushedFt() != 10 || tg.GetPushStopped() {
		t.Errorf("the log says Goblin 1 was pushed %d ft, stopped %v, want 10 ft and not stopped", tg.GetPushedFt(), tg.GetPushStopped())
	}

	c.undoLast(t)
	wantSquare(t, c, "Goblin 1", [2]int32{18, 7})
}

// TestThunderwavePushStopsBeforeAWall: the cave's east wall is at column 23, so a goblin at
// column 21 can only go one square.
func TestThunderwavePushStopsBeforeAWall(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.waveFight(t, [2]int32{20, 6}, [2]int32{21, 6})

	c.wave(t)
	wantSquare(t, c, "Goblin 1", [2]int32{22, 6})
	if tg := c.castTargetsOf(t, c.get(t, c.master))["Goblin 1"]; tg.GetPushedFt() != 5 || !tg.GetPushStopped() {
		t.Errorf("the log says Goblin 1 was pushed %d ft, stopped %v, want 5 ft and stopped", tg.GetPushedFt(), tg.GetPushStopped())
	}
}

// TestThunderwavePushAgainstTheWallMovesNobodyAndSaysSo: the goblin already stands against the
// east wall, so it fails the save, keeps its square, and the log says the push was stopped.
func TestThunderwavePushAgainstTheWallMovesNobodyAndSaysSo(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.waveFight(t, [2]int32{21, 6}, [2]int32{22, 6})

	c.wave(t)
	wantSquare(t, c, "Goblin 1", [2]int32{22, 6})
	if tg := c.castTargetsOf(t, c.get(t, c.master))["Goblin 1"]; tg.GetPushedFt() != 0 || !tg.GetPushStopped() {
		t.Errorf("the log says Goblin 1 was pushed %d ft, stopped %v, want 0 ft and stopped", tg.GetPushedFt(), tg.GetPushStopped())
	}
}

// TestThunderwaveLeavesACreatureThatSavesWhereItStands: a save that holds moves nobody and the
// log has no push.
func TestThunderwaveLeavesACreatureThatSavesWhereItStands(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.waveFight(t, [2]int32{17, 7}, [2]int32{18, 7})

	c.h.roller.queue(20, 20, 20, 20)
	c.wave(t)
	wantSquare(t, c, "Goblin 1", [2]int32{18, 7})
	if tg := c.castTargetsOf(t, c.get(t, c.master))["Goblin 1"]; tg.GetPushedFt() != 0 || tg.GetPushStopped() || tg.GetPushNoMap() {
		t.Errorf("the log says Goblin 1 was pushed %d ft, stopped %v, no map %v, want no push", tg.GetPushedFt(), tg.GetPushStopped(), tg.GetPushNoMap())
	}
}

// TestThunderwavePushStopsBeforeACreature: Toren stands two squares behind the goblin, outside
// the cube, so the goblin stops in the square before him.
func TestThunderwavePushStopsBeforeACreature(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.waveFight(t, [2]int32{17, 6}, [2]int32{19, 6})
	c.mustMove(t, c.master, "Toren", 21, 6)

	c.wave(t)
	wantSquare(t, c, "Goblin 1", [2]int32{20, 6})
	wantSquare(t, c, "Toren", [2]int32{21, 6})
}

// TestThunderwavePushProvokesNothing: the goblin leaves the reach of Brisa without
// an offer of an opportunity attack.
func TestThunderwavePushProvokesNothing(t *testing.T) {
	t.Parallel()
	c := newAreaCave(t)
	c.waveFight(t, [2]int32{17, 6}, [2]int32{18, 6})
	c.mustMove(t, c.master, "Brisa", 17, 5) // beside the goblin and out of the cube: the push leaves her reach
	c.wave(t)
	wantSquare(t, c, "Goblin 1", [2]int32{20, 6})
	if got := len(c.get(t, c.master).GetOpportunityOffers()); got != 0 {
		t.Errorf("opportunity offers = %d, want none for a forced move", got)
	}
}

// TestOpenHandPushMovesTheTargetFifteenFeet: the Goblin at (4, 3) fails the Strength save and
// goes three squares away from Toren at (3, 3); a save that holds leaves it where it was.
func TestOpenHandPushMovesTheTargetFifteenFeet(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		d20  int
		want [2]int32
	}{{"failed save", 1, [2]int32{7, 3}}, {"saved", 20, [2]int32{4, 3}}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, e, offers := riderMonkFight(t)
			a.h.roller.queue(tc.d20)
			if _, err := a.useRider(t, a.caio, e, offers[playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND], playv1.HitRiderChoice_HIT_RIDER_CHOICE_PUSH); err != nil {
				t.Fatalf("UseHitRider() error = %v", err)
			}
			who := byLabel(t, a.get(t, a.master), "Goblin")
			if got := [2]int32{who.GetCol(), who.GetRow()}; got != tc.want {
				t.Errorf("the Goblin stands at %v, want %v", got, tc.want)
			}
		})
	}
}

// TestOpenHandPushInTheTheatreOfTheMindMovesNobody: with no map the log line says the push is
// left to the table.
func TestOpenHandPushInTheTheatreOfTheMindMovesNobody(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:monk", 5, "subclass:open-hand"), abilities(10, 16, 14, 14, 8), nil, nil, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, abilities(10, 14, 12, 16, 8), nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, abilities(10, 16, 14, 10, 8), []string{rapier}, nil)
	})
	e := a.start(t, plan{
		theatre:  true,
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
	})
	wantHit(t, "the Attack action", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Goblin", d20(15)), hit)
	wantHit(t, "the Extra Attack", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Goblin", d20(15)), hit)
	if _, err := a.action(t, a.caio, e, "Toren", flurry); err != nil {
		t.Fatalf("Flurry of Blows error = %v", err)
	}
	wantHit(t, "a flurry strike", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Goblin", d20(15)), hit)
	open := ridersOfKind(a.get(t, a.caio), playv1.HitRiderKind_HIT_RIDER_KIND_OPEN_HAND)
	a.h.roller.queue(1)
	if _, err := a.useRider(t, a.caio, e, open[0].GetId(), playv1.HitRiderChoice_HIT_RIDER_CHOICE_PUSH); err != nil {
		t.Fatalf("UseHitRider() error = %v", err)
	}
	line := a.lastRiderLine(t)
	if line.GetResult() != playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_FAILED || !line.GetPushNoMap() || line.GetPushedFt() != 0 {
		t.Errorf("the rider line = result %v, no map %v, pushed %d ft; want a failed save left to the table", line.GetResult(), line.GetPushNoMap(), line.GetPushedFt())
	}
}
