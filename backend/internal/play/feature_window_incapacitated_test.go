package play

import "testing"

// An incapacitated reactor is never asked, and a window that waits when the condition lands closes (#743, docs/architecture/play/
// reactions.md "The feature window"). The conditions are the ones the SRD gives that take the reaction: stunned, paralyzed,
// unconscious (and incapacitated, petrified). These tests need the database.

var incapacitatingConditions = []string{"condition:stunned", "condition:paralyzed", "condition:unconscious", "condition:incapacitated", "condition:petrified"}

// Pensantus is incapacitated when the goblin starts its turn 10 ft from her: no window opens for anybody.
func TestAnIncapacitatedCharacterIsNotAskedAboutAFeature(t *testing.T) {
	t.Parallel()
	for _, cond := range incapacitatingConditions {
		t.Run(cond, func(t *testing.T) {
			t.Parallel()
			m := haloTable(t)
			e := m.fightHalo(t)
			m.setConditions(t, e, "Pensantus", cond)
			m.passTo(t, e, "Goblin")
			if w := m.windowOf(t, m.master, kindFeature); w != nil {
				t.Errorf("the master reads %v: %s Pensantus is asked", w, cond)
			}
			if w := m.windowOf(t, m.ana, kindFeature); w != nil {
				t.Errorf("Ana reads %v: %s Pensantus is asked", w, cond)
			}
		})
	}
}

// Nor is she asked when the goblin moves near.
func TestAnIncapacitatedCharacterIsNotAskedAboutAFeatureWhenTheCreatureMoves(t *testing.T) {
	t.Parallel()
	m := haloTable(t, whenItMoves)
	e := m.fightHalo(t)
	m.put(t, "Goblin", 8, 5)
	m.setConditions(t, e, "Pensantus", "condition:stunned")
	e = m.passTo(t, e, "Goblin")
	m.moveTo(t, e, "Goblin", 7, 5)
	if w := m.windowOf(t, m.master, kindFeature); w != nil {
		t.Errorf("the master reads %v: a stunned Pensantus is asked", w)
	}
}

// The window waits for Ana and the master stuns Pensantus: the window closes, the goblin's turn can end, and Ana is not asked
// again by the same creature.
func TestTheFeatureWindowClosesWhenTheReactorIsIncapacitated(t *testing.T) {
	t.Parallel()
	for _, cond := range incapacitatingConditions {
		t.Run(cond, func(t *testing.T) {
			t.Parallel()
			m := haloTable(t)
			e, w := m.haloWindow(t)
			m.setConditions(t, e, "Pensantus", cond)
			if got := m.windowOf(t, m.master, kindFeature); got != nil {
				t.Fatalf("the window %s stands after Pensantus was made %s", w.GetId(), cond)
			}
			if m.windowOf(t, m.ana, kindFeature) != nil {
				t.Errorf("Ana still reads the window after the condition")
			}
			if _, err := m.endTurn(t, m.master, e, true); err != nil {
				t.Errorf("the goblin's turn waits after the window closed: %v", err)
			}
		})
	}
}
