package play

import (
	"fmt"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// measuredTable is a table whose current map has the creature and both bards within 60 feet of each other, and a scene on that map open
// to the players given (Lia is Ana's, Brisa is Bia's).
func measuredTable(t *testing.T, audience ...func(n *npcTable) *user) *npcTable {
	t.Helper()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	mapID := n.mapWith(t, gridColumns)
	n.h.placeToken(mapID, n.bri.GetId(), 250, 250)
	n.h.placeToken(mapID, n.lia.GetId(), 250, 750)
	n.h.placeToken(mapID, n.npcID, 3250, 250)
	point, _ := n.h.newScene(mapID, "A clareira", true, 1)
	var users []*user
	for _, who := range audience {
		users = append(users, who(n))
	}
	n.openFor(t, point, users...)
	return n
}

func ana(n *npcTable) *user { return n.ana }
func bia(n *npcTable) *user { return n.bia }

// TestNpcCheckOnAMeasuredMapAsksOnlyTheBardsInTheScene: both tokens are on the grid within 60 feet and the creature is seen, and the scene
// is open to one of the two bards' players: only that bard is asked, whatever the box says.
func TestNpcCheckOnAMeasuredMapAsksOnlyTheBardsInTheScene(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		audience func(n *npcTable) *user
		asked    string
	}{
		"open to Ana":      {ana, "Lia"},
		"open to Bia only": {bia, "Brisa"},
	} {
		for _, box := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s, the box %v", name, box), func(t *testing.T) {
				t.Parallel()
				n := measuredTable(t, tc.audience)
				res, err := n.roll(t, 15, box)
				if err != nil {
					t.Fatalf("RollNpcCheck() error = %v", err)
				}
				asks := res.GetCheck().GetAsks()
				if !res.GetHeld() || len(asks) != 1 || asks[0].GetCharacterName() != tc.asked {
					t.Fatalf("the roll = %v, want one bard asked: %s", res, tc.asked)
				}
				outside := n.bia
				if tc.asked == "Brisa" {
					outside = n.ana
				}
				n.mustNothing(outside)
			})
		}
	}
}

// TestNpcCheckPickerCountsOnlyTheBardsInTheScene: the map measures a creature against the bards whose player is in the scene: with the
// scene open to a player whose bard has no token, the box decides.
func TestNpcCheckPickerCountsOnlyTheBardsInTheScene(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		audience func(n *npcTable) *user
		want     bool
	}{
		"the bard with a token is in the scene":      {bia, true},
		"the bard with a token is outside the scene": {ana, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
			mapID := n.mapWith(t, gridColumns)
			n.h.placeToken(mapID, n.bri.GetId(), 250, 250)
			n.h.placeToken(mapID, n.npcID, 3250, 250)
			point, _ := n.h.newScene(mapID, "A clareira", true, 1)
			n.openFor(t, point, tc.audience(n))
			c := n.candidate(t, n.npcID)
			if c == nil {
				t.Fatal("the creature is not listed")
			}
			if c.GetMapMeasures() != tc.want {
				t.Errorf("map_measures = %v, want %v", c.GetMapMeasures(), tc.want)
			}
		})
	}
}

// bothInTheScene opens a scene to Ana and Bia and rolls the check both bards are asked about.
func (n *npcTable) bothInTheScene(t *testing.T) (hold string, first, second *user) {
	t.Helper()
	point, _ := n.h.newScene(n.h.newMap(n.campaignID, gridColumns), "A clareira", true, 1)
	n.openFor(t, point, n.ana, n.bia)
	return n.twoAsked(t)
}

// TestNpcCheckLetsPassTheBardWhosePlayerLeftTheScene: the master took the player of the bard asked now out of the scene while the question
// waited: the question is not offered to that bard, it passes to the next bard, and the master and the next bard are told.
func TestNpcCheckLetsPassTheBardWhosePlayerLeftTheScene(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	_, first, second := n.bothInTheScene(t)
	n.mustOne(first)
	master, nextEars := listen(t, n.h, n.campaignID, n.master.id, true), listen(t, n.h, n.campaignID, second.id, false)
	n.mustSetAudience(t, second)
	n.mustNothing(first)
	n.mustOne(second)
	got := n.mustLatest(t)
	if got.GetSettled() || got.GetAsks()[0].GetState() != playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_PASSED || got.GetAsks()[1].GetState() != playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_WAITING {
		t.Errorf("GetNpcCheck() = %v, want the first bard passed and the second asked, waiting", got)
	}
	if !n.changed(t, master) || !n.changed(t, nextEars) {
		t.Error("the master and the next bard were not told that the question moved")
	}
}

// TestNpcCheckRefusesTheAnswerOfTheBardWhosePlayerLeftTheScene: the same bard answers after the master took the player out of the scene:
// the answer is refused as for a question that is gone, and nothing is spent.
func TestNpcCheckRefusesTheAnswerOfTheBardWhosePlayerLeftTheScene(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	hold, first, second := n.bothInTheScene(t)
	before := n.uses(t, n.charOf(first))
	n.mustSetAudience(t, second)
	if _, err := n.answerAs(first, hold, newKey(), typedCut(3)); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("the bard's cut error = %v, want %v", err, connect.CodeNotFound)
	}
	if after := n.uses(t, n.charOf(first)); after != before {
		t.Errorf("the bard's uses %d -> %d, want none spent", before, after)
	}
	n.mustCut(second, hold)
}

// TestNpcCheckReadsTheMapItWasRolledOn: the keep reads the tokens of the map the check was rolled on, not the session's current one.
func TestNpcCheckReadsTheMapItWasRolledOn(t *testing.T) {
	t.Parallel()
	t.Run("the creature is hidden on the map of the roll after the master changed the map", func(t *testing.T) {
		t.Parallel()
		n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
		first := n.withTheCreatureOnTheMap(t)
		hold, bard, _ := n.twoAsked(t)
		second := n.mapWith(t, gridColumns)
		n.h.placeToken(second, n.npcID, 1750, 250)
		n.hideToken(t, first, n.npcID)
		n.mustNothing(bard)
		got := n.mustLatest(t)
		if !got.GetSettled() || got.GetHoldId() != hold || got.GetAsks()[0].GetState() != playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_CLOSED {
			t.Errorf("GetNpcCheck() = %v, want the check settled as a skip", got)
		}
		if ev := n.resolvedEvents(t); len(ev) != 1 || !ev[0].skipped {
			t.Errorf("the resolved events = %+v, want one skip", ev)
		}
	})
	t.Run("the creature is hidden on another map than the one of the roll", func(t *testing.T) {
		t.Parallel()
		n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
		n.withTheCreatureOnTheMap(t)
		_, bard, _ := n.twoAsked(t)
		second := n.mapWith(t, gridColumns)
		n.h.placeToken(second, n.npcID, 1750, 250)
		n.hideToken(t, second, n.npcID)
		n.mustOne(bard)
		if got := n.mustLatest(t); got.GetSettled() {
			t.Errorf("GetNpcCheck() = %v, want the check still waiting", got)
		}
	})
	t.Run("the check was rolled with no map", func(t *testing.T) {
		t.Parallel()
		n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
		_, bard, _ := n.twoAsked(t)
		mapID := n.mapWith(t, gridColumns)
		n.h.placeToken(mapID, n.npcID, 1750, 250)
		n.hideToken(t, mapID, n.npcID)
		n.mustOne(bard)
		if got := n.mustLatest(t); got.GetSettled() {
			t.Errorf("GetNpcCheck() = %v, want the check still waiting", got)
		}
	})
}

// TestNpcCheckTellsNoBardOutsideTheSceneOfAMove: the next bard's player is outside the scene when the question moves to it, by an answer
// or by the master's skip: the bard hears no held rolls change, and the bard after it is asked in its place.
func TestNpcCheckTellsNoBardOutsideTheSceneOfAMove(t *testing.T) {
	t.Parallel()
	t.Run("the bard asked first answers", func(t *testing.T) {
		t.Parallel()
		n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
		hold, first, second := n.bothInTheScene(t)
		ears := listen(t, n.h, n.campaignID, second.id, false)
		n.mustSetAudience(t, first)
		n.mustCut(first, hold)
		if n.changed(t, ears) {
			t.Error("the bard outside the scene heard the held rolls change, want nothing")
		}
		if got := n.mustLatest(t); !got.GetSettled() || got.GetAsks()[1].GetState() != playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_PASSED {
			t.Errorf("GetNpcCheck() = %v, want it settled with the bard outside the scene passed", got)
		}
	})
	t.Run("the master skips", func(t *testing.T) {
		t.Parallel()
		n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
		hold, first, second := n.bothInTheScene(t)
		ears := listen(t, n.h, n.campaignID, first.id, false)
		n.mustSetAudience(t, second)
		if _, err := n.skip(n.master, hold, newKey()); err != nil {
			t.Fatalf("the skip error = %v", err)
		}
		if n.changed(t, ears) {
			t.Error("the bard outside the scene heard the held rolls change, want nothing")
		}
	})
}

// TestNpcCheckRefusesThePassedBardAsGone: a bard whose question was passed for it, while its player was outside the scene, is told the
// check is not found when it answers back inside the scene.
func TestNpcCheckRefusesThePassedBardAsGone(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	hold, first, second := n.bothInTheScene(t)
	n.mustSetAudience(t, second)
	n.mustOne(second) // the keep passes the first bard's question
	n.mustSetAudience(t, first, second)
	if _, err := n.answerAs(first, hold, newKey(), typedCut(3)); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("the passed bard's cut error = %v, want %v", err, connect.CodeNotFound)
	}
}
