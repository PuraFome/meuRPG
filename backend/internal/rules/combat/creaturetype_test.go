package combat

import "testing"

func TestLifeMagicNoEffect(t *testing.T) {
	t.Parallel()
	for typ, want := range map[string]bool{"undead": true, "construct": true, "plant": false, "humanoid": false, "beast": false, "": false} {
		if got := LifeMagicNoEffect(typ); got != want {
			t.Errorf("LifeMagicNoEffect(%q) = %v, want %v", typ, got, want)
		}
	}
}

func TestBlightSaveModeIsDisadvantageForAPlantOnly(t *testing.T) {
	t.Parallel()
	for typ, want := range map[string]RollMode{"plant": ModeDisadvantage, "undead": ModeNormal, "humanoid": ModeNormal, "": ModeNormal} {
		if got := BlightSaveMode(typ); got != want {
			t.Errorf("BlightSaveMode(%q) = %v, want %v", typ, got, want)
		}
	}
}

func TestMaxRoll(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ count, sides, bonus, want int }{{8, 8, 0, 64}, {13, 8, 0, 104}, {1, 8, 3, 11}, {0, 0, 5, 5}} {
		if got := MaxRoll(tc.count, tc.sides, tc.bonus); got != tc.want {
			t.Errorf("MaxRoll(%d, %d, %d) = %d, want %d", tc.count, tc.sides, tc.bonus, got, tc.want)
		}
	}
}

func TestSaveModeListsTheTypeSourceOfASpellThatKeysOnIt(t *testing.T) {
	t.Parallel()
	got := SaveMode(SaveScene{Ability: "con", TypeMode: ModeDisadvantage, CreatureType: "plant"})
	if len(got) != 1 || got[0] != (Source{Kind: SourceTargetCreatureType, Effect: ModeDisadvantage, Condition: "plant"}) {
		t.Errorf("SaveMode with a plant = %+v, want the one source of the type", got)
	}
	if got := SaveMode(SaveScene{Ability: "con"}); len(got) != 0 {
		t.Errorf("SaveMode with no type rule = %+v, want none", got)
	}
	if got := SaveMode(SaveScene{Ability: "con", Check: true, TypeMode: ModeDisadvantage, CreatureType: "plant"}); len(got) != 0 {
		t.Errorf("SaveMode of a check = %+v, want none: a check is not a save", got)
	}
}

func TestSpellTypeRuleIsBlightsAlone(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		spell, typ string
		want       TypeRule
	}{
		{SpellBlight, "undead", TypeRule{NoEffect: true, CreatureType: "undead"}},
		{SpellBlight, "construct", TypeRule{NoEffect: true, CreatureType: "construct"}},
		{SpellBlight, "plant", TypeRule{Mode: ModeDisadvantage, CreatureType: "plant", Plant: true}},
		{SpellBlight, "humanoid", TypeRule{CreatureType: "humanoid"}},
		{SpellBlight, "", TypeRule{}},
		{"spell:fireball", "plant", TypeRule{}},
		{"spell:fireball", "undead", TypeRule{}},
	} {
		if got := SpellTypeRule(tc.spell, tc.typ); got != tc.want {
			t.Errorf("SpellTypeRule(%q, %q) = %+v, want %+v", tc.spell, tc.typ, got, tc.want)
		}
	}
}
