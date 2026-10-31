package leaktest

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// An adult red dragon in Ana's sight: the master reads its legendary state (3 actions, 3 uses
// of Legendary Resistance, 3 options), and no player reads the field in the combat, the log or
// anywhere the matrix looks (RN-20, RN-29). The matrix flags the field as a master-only field
// in every player's answer; this test is the positive control that the field is populated.
func TestTheDragonsLegendaryStateIsTheMastersAlone(t *testing.T) {
	w := newWorld(t)
	enc := w.master.currentEncounter(w)
	var dragon *playv1.Combatant
	for _, c := range enc.GetCombatants() {
		if c.GetBestiaryCreatureKey() == dragonKey {
			dragon = c
		}
	}
	l := dragon.GetLegendary()
	if l == nil || l.GetActionsPerRound() != 3 || l.GetActionsLeft() != 3 || l.GetResistancePerDay() != 3 || l.GetResistanceLeft() != 3 || len(l.GetOptions()) != 3 {
		t.Fatalf("the master reads the dragon's legendary state as %v, want 3/3 actions, 3/3 resistance and 3 options", l)
	}
	for _, p := range []*person{w.ana, w.caio} {
		seen := false
		mustSee := p == w.ana // the dragon stands in Ana's sight, not in Caio's
		for _, c := range p.currentEncounter(w).GetCombatants() {
			if c.GetId() != dragon.GetId() {
				continue
			}
			seen = true
			if c.GetLegendary() != nil {
				t.Errorf("%s reads the dragon's legendary state %v", p.name, c.GetLegendary())
			}
		}
		if mustSee && !seen {
			t.Errorf("%s does not read the dragon, which is in plain sight: the control is missing", p.name)
		}
		for _, r := range w.reads(p) {
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
	}
}
