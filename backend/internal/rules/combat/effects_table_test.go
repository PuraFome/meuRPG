package combat

import (
	"slices"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

func hasSource(sources []Source, kind string, effect RollMode) bool {
	return slices.ContainsFunc(sources, func(s Source) bool { return s.Kind == kind && s.Effect == effect })
}

func TestAnEffectGivesItsAttackRollsAdvantage(t *testing.T) {
	t.Parallel()
	const key = "spell:test-keen-eye"
	r := AttackMode(AttackScene{Attacker: Creature{AttackAdvantage: key}})
	want := Source{Kind: SourceEffectAttack, Effect: ModeAdvantage, Condition: key}
	if len(r.Sources) != 1 || r.Sources[0] != want {
		t.Fatalf("sources = %+v, want %+v", r.Sources, want)
	}
	if got := Resolve(r.Sources); got != ModeAdvantage {
		t.Errorf("mode = %v, want advantage", got)
	}
	// The opposite effect cancels it, and both stay listed.
	both := AttackMode(AttackScene{Attacker: Creature{AttackAdvantage: key, AttackDisadvantage: "spell:test-jinx"}})
	if got := Resolve(both.Sources); got != ModeNormal || len(both.Sources) != 2 {
		t.Errorf("mode %v with %+v, want a normal roll listing both", got, both.Sources)
	}
	// The effect on the target changes nothing of the attacks against it.
	if got := AttackMode(AttackScene{Target: Creature{AttackAdvantage: key}}).Sources; len(got) != 0 {
		t.Errorf("an effect on the target gave %+v", got)
	}
}

func TestAnEffectGivesSavesAndChecksDisadvantage(t *testing.T) {
	t.Parallel()
	c := Creature{SaveDisadvantage: []string{"wis"}, CheckDisadvantage: []string{"str"}}
	save := SaveMode(SaveScene{Creature: c, Ability: "wis"})
	if !hasSource(save, SourceEffectSave, ModeDisadvantage) || Resolve(save) != ModeDisadvantage {
		t.Errorf("a Wisdom save: sources %+v, want the effect's disadvantage", save)
	}
	if got := SaveMode(SaveScene{Creature: c, Ability: "dex"}); len(got) != 0 {
		t.Errorf("a Dexterity save: sources %+v, want none", got)
	}
	if got := SaveMode(SaveScene{Creature: c, Ability: "str"}); len(got) != 0 {
		t.Errorf("a Strength save listed the check's disadvantage: %+v", got)
	}
	check := SaveMode(SaveScene{Creature: c, Ability: "str", Check: true})
	if !hasSource(check, SourceEffectCheck, ModeDisadvantage) || Resolve(check) != ModeDisadvantage {
		t.Errorf("a Strength check: sources %+v, want the effect's disadvantage", check)
	}
	if got := SaveMode(SaveScene{Creature: c, Ability: "wis", Check: true}); len(got) != 0 {
		t.Errorf("a Wisdom check listed the save's disadvantage: %+v", got)
	}
	// An advantage on the same save cancels it.
	c.SaveAdvantage = []string{"wis"}
	if got := Resolve(SaveMode(SaveScene{Creature: c, Ability: "wis"})); got != ModeNormal {
		t.Errorf("advantage and disadvantage: mode = %v, want normal", got)
	}
}

func TestEffectResistancesFeedTheDamageSteps(t *testing.T) {
	t.Parallel()
	const fire, slashing = "damage-type:fire", "damage-type:slashing"
	mods := EffectResistances("spell:test-ward", []rules.EffectModifier{
		{Kind: rules.ModifierResistance, ResistanceKind: rules.ResistanceKindResist, DamageTypes: []string{fire}},
		{Kind: rules.ModifierResistance, ResistanceKind: rules.ResistanceKindVulnerable, DamageTypes: []string{"damage-type:cold"}},
		{Kind: rules.ModifierResistance, ResistanceKind: rules.ResistanceKindImmune, DamageTypes: []string{"damage-type:acid"}},
		{Kind: rules.ModifierResistance, DamageTypes: []string{rules.DamageTypeNonmagicalBPS}},
		{Kind: rules.ModifierTempHP, Value: 5},
	})
	if len(mods) != 4 {
		t.Fatalf("modifiers = %+v, want the four resistance ones", mods)
	}
	for _, c := range []struct {
		dt     string
		from   Origin
		amount int
		want   int
	}{
		{fire, Origin{}, 9, 4},
		{"damage-type:cold", Origin{}, 5, 10},
		{"damage-type:acid", Origin{}, 7, 0},
		{slashing, Origin{Attack: true}, 9, 4},
		{slashing, Origin{Attack: true, Magical: true}, 9, 9},
	} {
		if got, _ := Adjust(c.amount, c.dt, mods, nil, c.from); got != c.want {
			t.Errorf("Adjust(%d, %s, %+v) = %d, want %d", c.amount, c.dt, c.from, got, c.want)
		}
	}
}

func TestAResistanceToAllCoversEveryDamageType(t *testing.T) {
	t.Parallel()
	mods := EffectResistances("spell:test-ward", []rules.EffectModifier{{Kind: rules.ModifierResistance, DamageTypes: []string{rules.DamageTypeAll}}})
	if got, steps := Adjust(10, "damage-type:necrotic", mods, nil, Origin{}); got != 5 || len(steps) != 1 {
		t.Errorf("Adjust = %d with %d steps, want 5 with one", got, len(steps))
	}
}

// Resistance and vulnerability to one type: the damage is halved, then doubled (SRD 5.1, Damage Resistance and
// Vulnerability: resistance first, so 5 lands as 4).
func TestResistanceThenVulnerabilityOnOneType(t *testing.T) {
	t.Parallel()
	mods := EffectResistances("effect:a", []rules.EffectModifier{
		{Kind: rules.ModifierResistance, ResistanceKind: rules.ResistanceKindResist, DamageTypes: []string{"damage-type:fire"}},
		{Kind: rules.ModifierResistance, ResistanceKind: rules.ResistanceKindVulnerable, DamageTypes: []string{"damage-type:fire"}},
	})
	if got, _ := Adjust(5, "damage-type:fire", mods, nil, Origin{}); got != 4 {
		t.Errorf("5 fire damage = %d, want 4 (halved to 2, doubled)", got)
	}
}
