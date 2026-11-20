package play

import (
	"fmt"
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
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
