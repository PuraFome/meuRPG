package combat

import "testing"

// A table's roll mode against a condition is a tagged save that names the condition: it holds on the
// saving throws against that condition only, and may be a disadvantage.
func TestTaggedSaveAgainstAConditionAppliesOnlyAgainstIt(t *testing.T) {
	t.Parallel()
	brave := TaggedSave{Source: "feat:destemido", Against: "condition:frightened", Abilities: []string{"wis"}}
	timid := TaggedSave{Source: "feat:medroso", Against: "condition:frightened", Disadvantage: true}
	scene := func(ability string, against ...string) SaveScene {
		return SaveScene{Creature: Creature{TaggedSaves: []TaggedSave{brave, timid}}, Ability: ability, Against: against}
	}
	for _, tc := range []struct {
		name  string
		scene SaveScene
		want  RollMode
	}{
		{"against being frightened, Wisdom: both sources, a normal roll", scene("wis", "magic", "condition:frightened"), ModeNormal},
		{"against being frightened, Strength: only the disadvantage", scene("str", "condition:frightened"), ModeDisadvantage},
		{"against something else", scene("wis", "magic"), ModeNormal},
		{"against nothing known", scene("wis"), ModeNormal},
	} {
		if got := Resolve(SaveMode(tc.scene)); got != tc.want {
			t.Errorf("%s: mode = %v, want %v", tc.name, got, tc.want)
		}
	}
	only := SaveScene{Creature: Creature{TaggedSaves: []TaggedSave{brave}}, Ability: "wis", Against: []string{"condition:frightened"}}
	if got := Resolve(SaveMode(only)); got != ModeAdvantage {
		t.Errorf("advantage against being frightened: mode = %v, want advantage", got)
	}
}

// The sheet's table modifiers (a race, a feat) go through Adjust as the SRD's "Damage Resistance and
// Vulnerability" says: immunity first, then resistance, then vulnerability, each source once.
func TestTableModifiersOfASheetAdjustTheDamage(t *testing.T) {
	t.Parallel()
	cold := []string{"damage-type:cold"}
	for _, tc := range []struct {
		name string
		mods []Modifier
		from Origin
		want int
	}{
		{"resistance", []Modifier{{Kind: StepResistance, Source: "trait:a", DamageTypes: cold}}, Origin{}, 3},
		{"the same resistance from two sources counts once", []Modifier{{Kind: StepResistance, Source: "trait:a", DamageTypes: cold}, {Kind: StepResistance, Source: "feat:b", DamageTypes: cold}}, Origin{}, 3},
		{"vulnerability doubles", []Modifier{{Kind: StepVulnerability, Source: "feat:b", DamageTypes: cold}}, Origin{}, 14},
		{"resistance and vulnerability: halved, then doubled", []Modifier{{Kind: StepResistance, Source: "trait:a", DamageTypes: cold}, {Kind: StepVulnerability, Source: "feat:b", DamageTypes: cold}}, Origin{}, 6},
		{"immunity beats resistance and vulnerability", []Modifier{{Kind: StepResistance, Source: "a", DamageTypes: cold}, {Kind: StepVulnerability, Source: "b", DamageTypes: cold}, {Kind: StepImmunity, Source: "c", DamageTypes: cold}}, Origin{}, 0},
		{"a resistance to nonmagical attacks halves a nonmagical one", []Modifier{{Kind: StepResistance, Source: "a", DamageTypes: cold, Qualifier: QualifierNonmagical}}, Origin{Attack: true}, 3},
		{"and not a magical one", []Modifier{{Kind: StepResistance, Source: "a", DamageTypes: cold, Qualifier: QualifierNonmagical}}, Origin{Attack: true, Magical: true}, 7},
		{"and not damage that is no attack", []Modifier{{Kind: StepResistance, Source: "a", DamageTypes: cold, Qualifier: QualifierNonmagical}}, Origin{}, 7},
	} {
		if got, _ := Adjust(7, "damage-type:cold", tc.mods, nil, tc.from); got != tc.want {
			t.Errorf("%s: 7 cold became %d, want %d", tc.name, got, tc.want)
		}
	}
}
