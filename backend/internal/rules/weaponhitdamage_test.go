package rules

import (
	"slices"
	"strings"
	"testing"
)

// hitMod is a valid weapon_hit_damage: 1d6 fire on the next weapon hit, a Constitution save against blinded.
func hitMod(mutate func(*EffectModifier)) EffectModifier {
	m := EffectModifier{
		Kind: ModifierWeaponHitDamage, Trigger: HitTriggerNextHit, Die: 6, Dice: 1, PerSlotDice: 1, MaxDice: 4,
		DamageType: "damage-type:fire", Save: "con", OnFailConditions: []string{"condition:blinded"},
	}
	if mutate != nil {
		mutate(&m)
	}
	return m
}

func TestWeaponHitDamageEveryTriggerIsAccepted(t *testing.T) {
	t.Parallel()
	for _, trig := range []string{HitTriggerNextHit, HitTriggerNextMeleeHit, HitTriggerEachHit, HitTriggerMarkedTarget} {
		for _, die := range []int{4, 6, 8, 10, 12} {
			m := hitMod(func(m *EffectModifier) { m.Trigger, m.Die = trig, die })
			if _, err := withEffectSpell(t, oneMod(m)); err != nil {
				t.Errorf("%s d%d: %v", trig, die, err)
			}
		}
	}
	bare := EffectModifier{Kind: ModifierWeaponHitDamage, Trigger: HitTriggerNextHit, Die: 8, Dice: 2}
	if _, err := withEffectSpell(t, oneMod(bare)); err != nil {
		t.Errorf("a bare modifier: %v", err)
	}
}

func TestWeaponHitDamageRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	conds9 := make([]string, 9)
	for i := range conds9 {
		conds9[i] = "condition:" + string(rune('a'+i))
	}
	with := func(f func(*EffectModifier)) func(*TableSpell) { return oneMod(hitMod(f)) }
	noConc := func(trig string) func(*TableSpell) {
		return func(ts *TableSpell) {
			ts.Concentration = false
			ts.Effect.Modifiers = []EffectModifier{hitMod(func(m *EffectModifier) { m.Trigger = trig })}
		}
	}
	cases := []struct {
		name   string
		mutate func(*TableSpell)
		field  string
	}{
		{"empty trigger", with(func(m *EffectModifier) { m.Trigger = "" }), ".trigger"},
		{"unknown trigger", with(func(m *EffectModifier) { m.Trigger = "always" }), ".trigger"},
		{"no die", with(func(m *EffectModifier) { m.Die = 0 }), ".die"},
		{"d20", with(func(m *EffectModifier) { m.Die = 20 }), ".die"},
		{"d2", with(func(m *EffectModifier) { m.Die = 2 }), ".die"},
		{"zero dice", with(func(m *EffectModifier) { m.Dice = 0 }), ".dice"},
		{"21 dice", with(func(m *EffectModifier) { m.Dice, m.MaxDice = 21, 0 }), ".dice"},
		{"negative per slot", with(func(m *EffectModifier) { m.PerSlotDice = -1 }), ".per_slot_dice"},
		{"11 per slot", with(func(m *EffectModifier) { m.PerSlotDice = 11 }), ".per_slot_dice"},
		{"max below dice", with(func(m *EffectModifier) { m.Dice, m.MaxDice = 3, 2 }), ".max_dice"},
		{"max over 20", with(func(m *EffectModifier) { m.MaxDice = 21 }), ".max_dice"},
		{"unknown damage type", with(func(m *EffectModifier) { m.DamageType = "damage-type:boredom" }), ".damage_type"},
		{"a skill as damage type", with(func(m *EffectModifier) { m.DamageType = "skill:stealth" }), ".damage_type"},
		{"save not an ability", with(func(m *EffectModifier) { m.Save = "luck" }), ".save"},
		{"conditions without save", with(func(m *EffectModifier) { m.Save = "" }), ".on_fail_conditions"},
		{"save without conditions", with(func(m *EffectModifier) { m.OnFailConditions = nil }), ".on_fail_conditions"},
		{"unknown condition", with(func(m *EffectModifier) { m.OnFailConditions = []string{"condition:bored"} }), ".on_fail_conditions[0]"},
		{"repeated condition", with(func(m *EffectModifier) {
			m.OnFailConditions = []string{"condition:blinded", "condition:poisoned", "condition:blinded"}
		}), ".on_fail_conditions[2]"},
		{"nine conditions", with(func(m *EffectModifier) { m.OnFailConditions = conds9 }), ".on_fail_conditions"},
		{"each_hit without concentration", noConc(HitTriggerEachHit), ".trigger"},
		{"marked_target without concentration", noConc(HitTriggerMarkedTarget), ".trigger"},
		{"a stray value", with(func(m *EffectModifier) { m.Value = 2 }), ".value"},
		{"a stray applies_to", with(func(m *EffectModifier) { m.AppliesTo = []string{"attack"} }), ".applies_to"},
		{"a stray sign", with(func(m *EffectModifier) { m.Sign = 1 }), ".sign"},
		{"stray damage_types", with(func(m *EffectModifier) { m.DamageTypes = []string{"damage-type:fire"} }), ".damage_types"},
		{"stray conditions", with(func(m *EffectModifier) { m.Conditions = []string{"condition:blinded"} }), ".conditions"},
		{"an instantaneous spell", func(ts *TableSpell) {
			ts.Effect.Modifiers = []EffectModifier{hitMod(nil)}
			ts.Concentration, ts.Duration = false, TableDuration{Kind: DurationInstantaneous}
		}, ".effect"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := withEffectSpell(t, tc.mutate)
			if err == nil {
				t.Fatal("accepted")
			}
			want := "effect.modifiers[0]" + tc.field
			if tc.field == ".effect" {
				want = "effect"
			}
			if !slices.ContainsFunc(refusedFields(err), func(f string) bool { return strings.HasSuffix(f, want) }) {
				t.Errorf("fields %q, want one ending in %q (%v)", refusedFields(err), want, err)
			}
		})
	}
}

func TestWeaponHitDamageNextHitNeedsNoConcentration(t *testing.T) {
	t.Parallel()
	for _, trig := range []string{HitTriggerNextHit, HitTriggerNextMeleeHit} {
		_, err := withEffectSpell(t, func(ts *TableSpell) {
			ts.Concentration = false
			ts.Effect.Modifiers = []EffectModifier{hitMod(func(m *EffectModifier) { m.Trigger = trig })}
		})
		if err != nil {
			t.Errorf("%s: %v", trig, err)
		}
	}
}

func TestWeaponHitDamageRoundTripsThroughWith(t *testing.T) {
	t.Parallel()
	want := hitMod(nil)
	c, err := withEffectSpell(t, oneMod(want))
	if err != nil {
		t.Fatal(err)
	}
	d, ok := c.CombatSpellEffect(effectSpell(nil).Key)
	if !ok || len(d.Modifiers) != 1 {
		t.Fatalf("effect = %+v", d)
	}
	got := d.Modifiers[0]
	if got.Kind != want.Kind || got.Trigger != want.Trigger || got.Die != 6 || got.Dice != 1 || got.PerSlotDice != 1 || got.MaxDice != 4 ||
		got.DamageType != want.DamageType || got.Save != "con" || !slices.Equal(got.OnFailConditions, want.OnFailConditions) {
		t.Errorf("modifier = %+v, want %+v", got, want)
	}
	if wantTag := []string{"+1d6 no próximo golpe com arma, +1d6 por nível acima, até 4d6"}; !slices.Equal(d.TagsPT, wantTag) {
		t.Errorf("tags = %q, want %q", d.TagsPT, wantTag)
	}
}

func TestWeaponHitDamageIsAKindOfTheSharedVocabulary(t *testing.T) {
	t.Parallel()
	if !slices.Contains(modifierKinds, ModifierWeaponHitDamage) {
		t.Fatal("weapon_hit_damage is not in modifierKinds")
	}
	if err := checkModifier(hitMod(nil)); err != nil {
		t.Errorf("the SRD shape is refused: %v", err)
	}
}
