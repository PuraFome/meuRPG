package play

import (
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The player who controls a creature of a stat block reads its limits, the roll of its Recharge and the reason of
// a refusal, as the master does (RN-29); every other player reads none of them, with or without the fog. These
// tests need the database (MEURPG_TEST_DATABASE_URL).

// teiaRecharge is what a turn start of Toren's spider Teia, whose Web may be spent, left on the screens.
type teiaRecharge struct {
	a            *armed
	e            *playv1.Encounter
	rise         map[string]int32                       // how much each player's revision went up
	rolls        map[string][]*playv1.CombatLogRecharge // the roll lines each person reads
	web          map[string]*playv1.LimitedAction       // the Web as each person reads it
	others       []string                               // what Ana and Bia read, as JSON
	otherRefusal error
}

// teiaTurn plays the cave (with the fog on when fog) until Teia's turn starts, her Web spent when face is not 0
// and the d6 of the server coming out as face.
func teiaTurn(t *testing.T, fog bool, face int) teiaRecharge {
	t.Helper()
	c := newCave(t)
	if fog {
		newFogCaveWith(t, c)
	}
	c.give(t, c.toren, "monster:giant-spider", "Teia")
	e := c.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: c.goblins.GetId(), Count: 1}},
		npcRolls: []int{2},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1, "Teia": 15},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {6, 7}, "Pensantus": {5, 8}, "Brisa": {4, 7}, "Teia": {7, 7}, "Goblin": {8, 7}},
	})
	if face != 0 {
		c.spendByHand(t, "Teia", spentRechargePrefix+spiderWeb)
		c.h.roller.queue(face)
	}
	people := map[string]*user{"Toren's player": c.caio, "Pensantus's player": c.ana, "Brisa's player": c.bia, "the master": c.master}
	before := map[string]int32{}
	for name, u := range people {
		before[name] = c.get(t, u).GetRevision()
	}
	e = c.ownTurnAgain(t, e, "Teia")
	out := teiaRecharge{a: c.armed, e: e, rise: map[string]int32{}, rolls: map[string][]*playv1.CombatLogRecharge{}, web: map[string]*playv1.LimitedAction{}}
	for name, u := range people {
		out.rise[name] = c.get(t, u).GetRevision() - before[name]
		out.rolls[name] = c.rechargeLines(t, u, e)
		out.web[name] = c.limitedOf(t, u, "Teia", spiderWeb)
	}
	for _, u := range []*user{c.ana, c.bia} {
		out.others = append(out.others, asJSON(t, c.get(t, u))+asJSON(t, c.log(t, u, e)))
	}
	_, out.otherRefusal = c.attack(t, c.ana, e, "Teia", spiderWeb, "Goblin", twoD20(15))
	return out
}

// Toren's player reads the Web spent (a 1 does not recharge it, a 5 does) and the roll; Ana and Bia read nothing of
// either, and the revision of every player goes up as in a turn start with no roll. The master reads both.
func TestRN29_TheControllerReadsTheLimitsAndTheRollOfItsCreature(t *testing.T) {
	t.Parallel()
	for _, fog := range []bool{false, true} {
		t.Run(map[bool]string{false: "no fog", true: "fog"}[fog], func(t *testing.T) {
			t.Parallel()
			control := teiaTurn(t, fog, 0)
			if len(control.rolls["the master"]) != 0 {
				t.Fatalf("the control rolled: %v", control.rolls["the master"])
			}
			for _, tc := range []struct {
				face      int
				recharged bool
			}{{1, false}, {5, true}} {
				got := teiaTurn(t, fog, tc.face)
				for _, who := range []string{"Toren's player", "the master"} { // the positive controls
					lines := got.rolls[who]
					if len(lines) != 1 || lines[0].GetFace() != int32(tc.face) || lines[0].GetRecharged() != tc.recharged {
						t.Errorf("face %d: %s reads %v in the log, want one roll that recharged %v", tc.face, who, lines, tc.recharged)
					}
					if w := got.web[who]; w == nil || w.GetAvailable() != tc.recharged {
						t.Errorf("face %d: %s reads the Web as %v, want available %v", tc.face, who, w, tc.recharged)
					}
				}
				for _, who := range []string{"Pensantus's player", "Brisa's player"} {
					if len(got.rolls[who]) != 0 || got.web[who] != nil {
						t.Errorf("face %d: %s reads the roll %v and the Web %v", tc.face, who, got.rolls[who], got.web[who])
					}
				}
				for _, js := range got.others {
					if strings.Contains(js, "imitedActions") || strings.Contains(js, "echarge") || strings.Contains(js, "usesLeft") {
						t.Errorf("face %d: a player who does not control Teia reads the limit:\n%s", tc.face, js)
					}
				}
				for name, n := range control.rise {
					if got.rise[name] != n {
						t.Errorf("face %d: %s: the revision went up by %d with the roll and by %d without it", tc.face, name, got.rise[name], n)
					}
				}
				if got.otherRefusal == nil || strings.Contains(strings.ToLower(got.otherRefusal.Error()), "recharg") {
					t.Errorf("face %d: a player who does not control Teia was answered %v", tc.face, got.otherRefusal)
				}
			}
		})
	}
}
