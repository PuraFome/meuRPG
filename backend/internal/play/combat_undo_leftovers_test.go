package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Finding fix/undo-leftovers: a rage that begins while the barbarian concentrates ends the
// concentration (SRD 5.1, Rage), and the undo of the rage gives it back.
func TestUndoKindActionTakenRageEndsAConcentrationAndTheUndoGivesItBack(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	a.concentrate(t, "Brisa", webSpell)
	w := newUndoWatch(t, a)

	w.run(t, "a rage while concentrating", nil, func() {
		if _, err := a.action(t, a.bia, e, "Brisa", "feature:rage"); err != nil {
			t.Fatalf("TakeAction(Rage) error = %v", err)
		}
		if got := byLabel(t, a.get(t, a.master), "Brisa").GetConcentrationSpell(); got != "" {
			t.Fatalf("Brisa concentrates on %q while raging, want none", got)
		}
	})
}

// newHeroismCasters is a party with a bard who knows Heroism (a concentration spell whose
// temporary hit points go with it) and Bane (another concentration spell).
func newHeroismCasters(t *testing.T) *armed {
	t.Helper()
	return newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:bard", "race:gnome", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 10, Wisdom: 10, Charisma: 16}, nil, nil,
			[]string{heroismKey, "spell:bane"}, []string{heroismKey, "spell:bane"})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:barbarian", "race:human", 3,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 14, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
	})
}

// A cast that replaces the caster's concentration on Heroism ends the effect and the temporary hit
// points it gave; the undo of that cast puts the effect and the temporary hit points back.
func TestUndoKindSpellCastReplacingHeroismGivesTheTemporaryHitPointsBack(t *testing.T) {
	t.Parallel()
	a := newHeroismCasters(t)
	e := a.castersFight(t, 1)
	a.mustCast(t, a.ana, e, "Pensantus", heroismKey, slotOfLevel(1), a.at(t, "Toren"), noCastRoll)
	e = a.passTo(t, e, "Toren") // the start of his turn gives the temporary hit points
	if got := a.vitals(t, a.toren).GetHitPointsTemporary(); got != 3 {
		t.Fatalf("Toren's temporary hit points at the start of his turn = %d, want 3", got)
	}
	e = a.passTo(t, e, "Pensantus")
	w := newUndoWatch(t, a)

	w.run(t, "Perdição replaces Heroísmo", nil, func() {
		a.mustCast(t, a.ana, e, "Pensantus", "spell:bane", slotOfLevel(1), a.at(t, "Goblin"), noCastRoll)
		if got := a.vitals(t, a.toren).GetHitPointsTemporary(); got != 0 {
			t.Fatalf("Toren's temporary hit points after Heroism ended = %d, want 0", got)
		}
	})
}

// markWithHuntersMark has the caster concentrate on Hunter's Mark and mark the target, as the
// cast leaves them.
func markWithHuntersMark(t *testing.T, a *armed, e *playv1.Encounter, caster, target string) {
	t.Helper()
	a.concentrate(t, caster, huntersMark)
	if _, err := a.h.pool.Exec(t.Context(),
		`INSERT INTO combatant_states (encounter_id, combatant_id, kind, source_id, started_round, amount, created_at) VALUES ($1, $2, 'hunters_mark_target', $3, 1, 0, now())`,
		e.GetId(), a.id(t, target), a.id(t, caster)); err != nil {
		t.Fatalf("mark %s: %v", target, err)
	}
}

// Finding fix/undo-hunters-mark: a rage that ends a Hunter's Mark concentration takes the mark off
// its target, and the undo of the rage gives the mark back with the concentration.
func TestUndoKindActionTakenRageEndsHuntersMarkAndTheUndoGivesTheMarkBack(t *testing.T) {
	t.Parallel()
	a := newHeroismCasters(t)
	e := a.passTo(t, a.castersFight(t, 1), "Brisa")
	markWithHuntersMark(t, a, e, "Brisa", "Goblin")
	w := newUndoWatch(t, a)

	w.run(t, "a rage while concentrating on Hunter's Mark", nil, func() {
		if _, err := a.action(t, a.bia, e, "Brisa", "feature:rage"); err != nil {
			t.Fatalf("TakeAction(Rage) error = %v", err)
		}
		if huntersMarkOn(t, a.get(t, a.master), "Goblin") != nil {
			t.Fatal("the Goblin is still marked after the rage ended the concentration")
		}
	})
}

// Finding fix/undo-hunters-mark: a damage that takes the caster to 0 hit points ends the Hunter's
// Mark concentration and the mark, and the undo of the damage gives the mark back.
func TestUndoKindDamageAppliedAtZeroEndsHuntersMarkAndTheUndoGivesTheMarkBack(t *testing.T) {
	t.Parallel()
	a, e, id := concentratingDamage(t, 100)
	markWithHuntersMark(t, a, e, "Pensantus", "Goblin")
	w := newUndoWatch(t, a)

	w.run(t, "a damage to 0 while concentrating on Hunter's Mark", nil, func() {
		if _, err := a.settle(t, a.master, e, id, true); err != nil {
			t.Fatalf("ApplyPendingDamage() error = %v", err)
		}
		if huntersMarkOn(t, a.get(t, a.master), "Goblin") != nil {
			t.Fatal("the Goblin is still marked after the damage ended the concentration")
		}
	})
}
