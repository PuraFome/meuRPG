package play

import (
	"strings"
	"testing"
)

// The id of the creature a marked_target spell marked (#577): the caster's row of the spell carries it for the
// caster's player and the master, blank when the viewer does not see the creature (RN-10).

// markedIDOf is the marked creature's id the caster's card reads as u, "" when none.
func (a *armed) markedIDOf(t *testing.T, u *user) string {
	t.Helper()
	card := a.hitCardOf(t, u, "Pensantus")
	if card == nil {
		t.Fatalf("the caster's card is missing for the viewer")
	}
	return card.GetMarkedCombatantId()
}

func TestAMarkedTargetCardNamesTheMarkedCreatureToTheCasterAndTheMaster(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, markShape, hitMod(trigMarked))
	e := a.hitFight(t)
	a.castMark(t, e, 1, "Goblin")
	want := a.id(t, "Goblin")
	for name, u := range map[string]*user{"Ana": a.ana, "the master": a.master} {
		if got := a.markedIDOf(t, u); got != want {
			t.Errorf("%s reads the marked id %q, want the Goblin's %q", name, got, want)
		}
	}
}

func TestAnEffectThatMarksNobodyHasNoMarkedID(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigNext))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	for name, u := range map[string]*user{"Ana": a.ana, "the master": a.master} {
		if got := a.markedIDOf(t, u); got != "" {
			t.Errorf("%s reads the marked id %q on a next_hit effect, want none", name, got)
		}
	}
}

// RN-10: the id of the marked creature is read only where the viewer sees the creature. The Goblin is hidden after the
// cast: the caster's player reads the card with no id (the canary is the id itself, in no read of any player), the
// master reads it, and Ana reads it again when the Goblin is shown (the positive control).
func TestRN10_TheMarkedIDIsBlankWhenTheCasterDoesNotSeeTheCreature(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, markShape, hitMod(trigMarked))
	e := a.hitFight(t)
	a.castMark(t, e, 1, "Goblin")
	goblin := a.id(t, "Goblin")
	hideAgain(t, a, e, "Goblin")
	if got := a.markedIDOf(t, a.master); got != goblin {
		t.Errorf("the master reads the marked id %q, want %q", got, goblin)
	}
	for name, u := range map[string]*user{"Ana": a.ana, "Caio": a.caio, "Bia": a.bia} {
		if text := a.readsOf(t, u, e, "Pensantus"); strings.Contains(text, goblin) {
			t.Errorf("%s reads the id of the hidden creature", name)
		}
	}
	if got := a.markedIDOf(t, a.ana); got != "" {
		t.Errorf("Ana reads the marked id %q of a hidden creature, want none", got)
	}
	showNPC(t, a, e, "Goblin")
	if got := a.markedIDOf(t, a.ana); got != goblin {
		t.Errorf("Ana reads the marked id %q of a creature she sees, want %q", got, goblin)
	}
}

// RN-10: another player never reads the marked id, on any card, whether they see the creature or not.
func TestRN10_AnotherPlayerNeverReadsTheMarkedID(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, markShape, hitMod(trigMarked))
	e := a.hitFight(t)
	a.castMark(t, e, 1, "Goblin")
	for name, u := range map[string]*user{"Caio": a.caio, "Bia": a.bia} {
		for _, c := range a.get(t, u).GetCombatants() {
			for _, ef := range c.GetEffects() {
				if ef.GetMarkedCombatantId() != "" {
					t.Errorf("%s reads the marked id %q on the card of %s", name, ef.GetMarkedCombatantId(), c.GetLabel())
				}
			}
		}
	}
}
