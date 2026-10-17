package play

import (
	"errors"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Misty Step (SRD 5.1): a bonus action that teleports the caster up to 30 feet to a
// free space the caster sees. The caster's player picks the square in the cast. These
// tests need the database (MEURPG_TEST_DATABASE_URL). Fixture: the cave of the area
// tests, Pensantus (level 5) in the corridor at (12,7), Goblin 1 at (18,7), and a wall
// at (12,6).

const mistyStep = "spell:misty-step"

var (
	blockedTeleportTooFar = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TELEPORT_TOO_FAR
	blockedUnavailable    = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DESTINATION_UNAVAILABLE
	blockedOccupied       = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_SQUARE_OCCUPIED
	blockedWall           = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_WALL_ON_SQUARE
)

func newMistyCave(t *testing.T) *cave {
	t.Helper()
	return newCaveWith(t, 5, []string{mistyStep, burningHands})
}

// misty casts Misty Step as u with the square chosen (nil: no square).
func (c *cave) misty(t *testing.T, u *user, dest *playv1.SpellOrigin) (*playv1.CastSpellResponse, error) {
	t.Helper()
	return c.castArea(t, u, "Pensantus", mistyStep, slotOfLevel(2), nil, nil, func(r *playv1.CastSpellRequest) { r.Destination = dest })
}

func (c *cave) squareOf(t *testing.T, u *user, label string) [2]int32 {
	t.Helper()
	w := c.who(t, u, label)
	return [2]int32{w.GetCol(), w.GetRow()}
}

func TestMistyStepThePlayerPicksTheSquareAndNobodyGetsAnOpportunityAttack(t *testing.T) {
	t.Parallel()
	c := newMistyCave(t)
	c.areaFight(t)
	c.mustMove(t, c.master, "Goblin 1", 13, 7) // next to the caster: walking away would provoke
	before := c.who(t, c.ana, "Pensantus")

	res, err := c.misty(t, c.ana, at(8, 7))
	if err != nil {
		t.Fatalf("CastSpell(Misty Step) error = %v", err)
	}
	if got := c.squareOf(t, c.ana, "Pensantus"); got != [2]int32{8, 7} {
		t.Fatalf("Pensantus is at %v, want [8 7]", got)
	}
	p := byLabel(t, res.GetEncounter(), "Pensantus")
	if !p.GetBonusActionUsed() || p.GetActionUsed() {
		t.Errorf("economy = action %v bonus %v, want only the bonus action", p.GetActionUsed(), p.GetBonusActionUsed())
	}
	if p.GetMovementUsedDft() != before.GetMovementUsedDft() {
		t.Errorf("movement used %d, was %d: a teleport spends none", p.GetMovementUsedDft(), before.GetMovementUsedDft())
	}
	if n := len(c.get(t, c.master).GetOpportunityOffers()); n != 0 {
		t.Errorf("%d opportunity offers after a teleport, want none", n)
	}

	c.undoLast(t)
	if got := c.squareOf(t, c.master, "Pensantus"); got != [2]int32{12, 7} {
		t.Errorf("after the undo Pensantus is at %v, want [12 7]", got)
	}
	if p := c.who(t, c.master, "Pensantus"); p.GetBonusActionUsed() {
		t.Error("the undo left the bonus action spent")
	}
}

func TestMistyStepWithoutASquareLeavesTheTokenToTheMaster(t *testing.T) {
	t.Parallel()
	c := newMistyCave(t)
	c.areaFight(t)
	if _, err := c.misty(t, c.ana, nil); err != nil {
		t.Fatalf("CastSpell(Misty Step) error = %v", err)
	}
	if got := c.squareOf(t, c.ana, "Pensantus"); got != [2]int32{12, 7} {
		t.Errorf("Pensantus is at %v, want [12 7]: no square, no move", got)
	}
}

func TestMistyStepRefusesTheSquaresItMay(t *testing.T) {
	t.Parallel()
	c := newMistyCave(t)
	c.areaFight(t)
	cases := []struct {
		name string
		to   *playv1.SpellOrigin
		want playv1.EncounterBlockedReason
	}{
		{"seven squares away", at(5, 7), blockedTeleportTooFar},
		{"a wall", at(12, 6), blockedUnavailable},
		{"an enemy the player sees", at(11, 7), blockedUnavailable},
	}
	c.mustMove(t, c.master, "Goblin 2", 11, 7)
	for _, tc := range cases {
		_, err := c.misty(t, c.ana, tc.to)
		wantBlockedBy(t, tc.name, err, tc.want)
	}
	if _, err := c.misty(t, c.ana, at(40, 40)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a square off the grid: error = %v, want invalid_argument", err)
	}
	if got := c.squareOf(t, c.ana, "Pensantus"); got != [2]int32{12, 7} {
		t.Errorf("a refused cast moved Pensantus to %v", got)
	}
	if p := c.who(t, c.ana, "Pensantus"); p.GetBonusActionUsed() {
		t.Error("a refused cast spent the bonus action")
	}
	// The master is told the reason, and is held to the same reach.
	_, err := c.misty(t, c.master, at(12, 6))
	wantBlockedBy(t, "master on a wall", err, blockedWall)
	_, err = c.misty(t, c.master, at(11, 7))
	wantBlockedBy(t, "master on a creature", err, blockedOccupied)
	// The field is a teleport's: another spell refuses it.
	_, err = c.castArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(1), nil, toward(1, 0), func(r *playv1.CastSpellRequest) { r.Destination = at(11, 8) })
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a destination on Burning Hands: error = %v, want invalid_argument", err)
	}
}

// TestRN10_MistyStepNeverSaysAHiddenCreatureStandsOnTheSquare: a hidden NPC's square is
// refused exactly as a wall is, and the master is told what it was.
func TestRN10_MistyStepNeverSaysAHiddenCreatureStandsOnTheSquare(t *testing.T) {
	t.Parallel()
	c := newMistyCave(t)
	e := c.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: c.goblins.GetId(), Count: 1}},
		npcRolls: []int{2},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		at:       map[string][2]int32{"Pensantus": {12, 7}, "Toren": {21, 8}, "Brisa": {4, 7}, "Goblin": {10, 7}},
	})
	c.passTo(t, e, "Pensantus")
	_, hiddenErr := c.misty(t, c.ana, at(10, 7))
	wantBlockedBy(t, "hidden NPC", hiddenErr, blockedUnavailable)
	_, wallErr := c.misty(t, c.ana, at(12, 6))
	wantBlockedBy(t, "wall", wallErr, blockedUnavailable)
	var a, b *connect.Error
	if !errors.As(hiddenErr, &a) || !errors.As(wallErr, &b) || a.Message() != b.Message() {
		t.Errorf("the refusals differ: %v / %v", hiddenErr, wallErr)
	}
	_, err := c.misty(t, c.master, at(10, 7))
	wantBlockedBy(t, "master on the hidden NPC", err, blockedOccupied)
}

// TestRN10_FogMistyStepNeedsASquareTheCasterSees: on a map with the fog, a square the
// player does not see is refused like any other unavailable one.
func TestRN10_FogMistyStepNeedsASquareTheCasterSees(t *testing.T) {
	t.Parallel()
	f := newFogCaveWith(t, newMistyCave(t))
	f.fightInSeparateTurns(t)
	f.passTo(t, f.get(t, f.master), "Pensantus")
	from := f.squareOf(t, f.master, "Pensantus")
	var seen, unseen *playv1.SpellOrigin
	for dc := int32(-6); dc <= 6; dc++ {
		for dr := int32(-6); dr <= 6; dr++ {
			col, row := from[0]+dc, from[1]+dr
			if col < 0 || row < 0 || col >= 24 || row >= 16 || caveRows[row][col] == '#' || (dc == 0 && dr == 0) {
				continue
			}
			if f.seesSquare(t, f.ana, int(col), int(row)) {
				seen = at(col, row)
			} else {
				unseen = at(col, row)
			}
		}
	}
	if seen == nil || unseen == nil {
		t.Fatalf("the fixture has no seen (%v) and unseen (%v) floor square near Pensantus", seen, unseen)
	}
	_, err := f.misty(t, f.ana, unseen)
	wantBlockedBy(t, "an unseen square", err, blockedUnavailable)
	if _, err := f.misty(t, f.ana, seen); err != nil {
		t.Fatalf("CastSpell(Misty Step) to a seen square error = %v", err)
	}
	if got := f.squareOf(t, f.master, "Pensantus"); got != [2]int32{seen.GetCol(), seen.GetRow()} {
		t.Errorf("Pensantus is at %v, want %v", got, seen)
	}
}
