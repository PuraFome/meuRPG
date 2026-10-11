package play

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The two features that change a jump (SRD 5.1): the monk's Step of the Wind doubles the
// jump distance for the turn it is used in, and the Thief's Second-Story Work adds the
// Dexterity modifier, in feet, to a running long jump. They need the database
// (MEURPG_TEST_DATABASE_URL).

// monkCave is the cave with Toren a monk of level 2 (Strength 16 with the human's +1, so the
// same jumps as the fighter of newCave: 16 ft running, 8 ft standing).
func monkCave(t *testing.T) *cave {
	t.Helper()
	return newCaveOf(t, 3, nil, func(a *armed) *charactersv1.Character {
		return a.caio.hero(t, a.campaignID, "Toren", "class:monk", "race:human", 2, abilities(15, 16, 14, 14, 8), nil, nil)
	})
}

func jumpsOf(t *testing.T, c *cave, e *playv1.Encounter) *rulesv1.JumpLimits {
	t.Helper()
	return c.mustOptions(t, c.caio, e, "Toren").GetOptions().GetJumps()
}

// Step of the Wind doubles every jump distance for the rest of the turn: a 15 ft long jump
// without a run, too far for the 8 ft a standing jump has, lands after it. Taking the action
// back gives the old distances back.
func TestMR034_StepOfTheWindDoublesTheJumpForTheTurn(t *testing.T) {
	t.Parallel()
	c := monkCave(t)
	e := c.fight(t)
	if j := jumpsOf(t, c, e); j.GetLongRunningDft() != 160 || j.GetLongStandingDft() != 80 || j.GetHighRunningDft() != 60 || j.GetHighStandingDft() != 30 {
		t.Fatalf("the monk's jumps = %v, want 16 / 8 ft long and 6 / 3 ft high", j)
	}
	c.mustMove(t, c.master, "Toren", 3, 9)
	_, err := c.move(t, c.caio, "Toren", 6, 9, jumpTo(playv1.JumpKind_JUMP_KIND_LONG))
	if b := wantEncounterBlocked(t, err, reasonTooFar); b.GetJumpLimitDft() != 80 || b.GetJumpRunningStart() {
		t.Errorf("the standing jump = limit %d, running %v; want 80 and false", b.GetJumpLimitDft(), b.GetJumpRunningStart())
	}

	if _, err := c.action(t, c.caio, e, "Toren", windDash); err != nil {
		t.Fatalf("Step of the Wind error = %v", err)
	}
	if j := jumpsOf(t, c, e); j.GetLongRunningDft() != 320 || j.GetLongStandingDft() != 160 || j.GetHighRunningDft() != 120 || j.GetHighStandingDft() != 60 {
		t.Errorf("the jumps after Step of the Wind = %v, want 32 / 16 ft long and 12 / 6 ft high", j)
	}
	c.mustMove(t, c.caio, "Toren", 6, 9, jumpTo(playv1.JumpKind_JUMP_KIND_LONG))
	if toren := c.who(t, c.caio, "Toren"); toren.GetCol() != 6 || toren.GetRow() != 9 {
		t.Errorf("after the doubled jump Toren is on (%d, %d), want (6, 9)", toren.GetCol(), toren.GetRow())
	}

	// The undo of the jump, then of the action: the distances are the old ones again.
	c.undoLast(t)
	c.undoLast(t)
	if j := jumpsOf(t, c, e); j.GetLongRunningDft() != 160 || j.GetLongStandingDft() != 80 {
		t.Errorf("after taking Step of the Wind back = %v, want 16 / 8 ft", j)
	}
	_, err = c.move(t, c.caio, "Toren", 6, 9, jumpTo(playv1.JumpKind_JUMP_KIND_LONG))
	wantEncounterBlocked(t, err, reasonTooFar)
}

// The doubling lasts for the turn: the start of the monk's next turn takes it away.
func TestMR034_TheDoublingEndsWithTheTurn(t *testing.T) {
	t.Parallel()
	c := monkCave(t)
	e := c.fight(t)
	if _, err := c.action(t, c.caio, e, "Toren", "feature:step-of-the-wind:disengage"); err != nil {
		t.Fatalf("Step of the Wind error = %v", err)
	}
	if j := jumpsOf(t, c, e); j.GetLongRunningDft() != 320 {
		t.Fatalf("the jumps after Step of the Wind = %v, want 32 ft running", j)
	}
	e = c.turnOf(t, "Toren")
	if j := jumpsOf(t, c, e); j.GetLongRunningDft() != 160 || j.GetLongStandingDft() != 80 {
		t.Errorf("on the next turn = %v, want 16 / 8 ft again", j)
	}
}

// Second-Story Work: a Thief with Dexterity 17 (+3, with the human's +1) jumps 3 ft farther with a running start, and the standing jump and the high jump are the
// usual ones.
func TestMR034_SecondStoryWorkAddsToTheRunningLongJump(t *testing.T) {
	t.Parallel()
	c := newCaveOf(t, 3, nil, func(a *armed) *charactersv1.Character {
		return a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:rogue", 3, "subclass:thief"), abilities(14, 16, 14, 12, 8), []string{rapier}, nil, nil)
	})
	e := c.fight(t)
	// Strength 15 (+2): 15 ft + 3 ft running, 7,5 ft standing; 3 + 2 = 5 ft high, 2,5 ft standing.
	if j := jumpsOf(t, c, e); j.GetLongRunningDft() != 180 || j.GetLongStandingDft() != 75 || j.GetHighRunningDft() != 50 || j.GetHighStandingDft() != 25 {
		t.Fatalf("the Thief's jumps = %v, want 18 / 7,5 ft long and 5 / 2,5 ft high", j)
	}
	// A standing jump has no bonus: 15 ft is beyond 7,5.
	c.mustMove(t, c.master, "Toren", 3, 9)
	_, err := c.move(t, c.caio, "Toren", 6, 9, jumpTo(playv1.JumpKind_JUMP_KIND_LONG))
	if b := wantEncounterBlocked(t, err, reasonTooFar); b.GetJumpLimitDft() != 75 || b.GetJumpRunningStart() {
		t.Errorf("the standing jump = limit %d, running %v; want 75 and false", b.GetJumpLimitDft(), b.GetJumpRunningStart())
	}
	// With a run, the 15 ft jump over the rubble is within the 18 ft the options show.
	c.mustMove(t, c.master, "Toren", 1, 9)
	c.mustMove(t, c.caio, "Toren", 3, 9)
	c.mustMove(t, c.caio, "Toren", 6, 9, jumpTo(playv1.JumpKind_JUMP_KIND_LONG))
}
