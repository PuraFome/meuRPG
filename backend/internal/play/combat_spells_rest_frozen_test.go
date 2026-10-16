package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// These scenarios complete the golden of combat_cast_spell_frozen_test.go for the branches of checkTargets,
// spellAttack and spellSave it leaves out: a caster-only spell aimed at another creature, a caster or a
// target off the map, a spell attack on a paralyzed target, and a save a condition fails by itself.
// testdata/spells_rest.golden holds what each scenario reads; run with UPDATE_GOLDEN=1 to write the file
// again, and read the diff.

const spellsRestGolden = "testdata/spells_rest.golden"

// spellsRestParalyze paralyzes a creature before the cast, as the master does.
func spellsRestParalyze(label string) func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
	return func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
		t.Helper()
		e, err := a.conditions(t, a.master, e, label, []string{"condition:paralyzed"}, true, false)
		if err != nil {
			t.Fatalf("SetCombatantConditions(%s paralyzed) error = %v", label, err)
		}
		return e
	}
}

// spellsRestNextTo puts a creature on a square, then the next step of the fight runs on the fight as it is.
func spellsRestNextTo(label string, col, row int32, then func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter) func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
	return func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
		t.Helper()
		a.put(t, label, col, row)
		e = a.get(t, a.master)
		if then == nil {
			return e
		}
		return then(t, a, e)
	}
}

// spellsRestOffMap takes a creature off the map before the cast.
func spellsRestOffMap(label string) func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
	return func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
		t.Helper()
		a.execSQL(t, `UPDATE combatants SET grid_col = NULL, grid_row = NULL WHERE label = $1`, label)
		return a.get(t, a.master)
	}
}

// spellsRestTableCasters is the fight of the casters with the table's spells, Pensantus holding a spell for the caster alone.
func spellsRestTableCasters(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newTableCasters(t)
	return a, a.castersFight(t, 1)
}

// spellsRestScenarios are the scenarios of the golden, in the order they are listed.
func spellsRestScenarios() []castScenario {
	return []castScenario{
		castSteps("spells rest, a caster-only spell aimed at another creature", spellsRestTableCasters, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.ana, e, "Pensantus", "spell:detect-magic", slotOfLevel(1), a.at(t, "Goblin"), noCastRoll)
		}),
		castRefused("spells rest, a spell attack by a caster off the map", brisa(guidingBolt, slotOfLevel(1), d20Face(16)).first(spellsRestOffMap("Brisa"))),
		castRefused("spells rest, a spell attack at a target off the map", brisa(guidingBolt, slotOfLevel(1), d20Face(16)).first(spellsRestOffMap("Goblin"))),
		brisa(guidingBolt, slotOfLevel(1), d20Faces(16, 3)).first(spellsRestNextTo("Brisa", 7, 6, spellsRestParalyze("Goblin"))).scenario("spells rest, a spell attack hits a paralyzed target next to the caster"),
		brisa(guidingBolt, slotOfLevel(1), d20Faces(2, 3)).first(spellsRestNextTo("Brisa", 7, 6, spellsRestParalyze("Goblin"))).scenario("spells rest, a spell attack misses a paralyzed target next to the caster"),
		brisa(guidingBolt, slotOfLevel(1), d20Faces(16, 3)).first(spellsRestParalyze("Goblin")).scenario("spells rest, a spell attack hits a paralyzed target far from the caster"),
		brisa(sacredFlame, nil, noCastRoll).first(spellsRestParalyze("Goblin")).scenario("spells rest, a dexterity save a paralyzed target fails by itself"),
	}
}

// TestSpellsRestIsFrozen runs every scenario on a fight of its own and compares what each reads with the
// golden.
func TestSpellsRestIsFrozen(t *testing.T) { //nolint:tparallel // the golden is compared once every scenario of the group has run
	t.Parallel()
	g := &castGolden{}
	t.Run("scenarios", func(t *testing.T) {
		for _, sc := range spellsRestScenarios() {
			t.Run(sc.name, func(t *testing.T) {
				t.Parallel()
				g.put(sc.name, sc.run(t))
			})
		}
	})
	g.settle(t, spellsRestGolden)
}
