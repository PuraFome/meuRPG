package characters

import (
	"slices"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// What play reads of a stat block's damage lists: the entries the
// SRD limits to nonmagical attacks are carried with that limit, not dropped.
func TestTypeModifiersCarryTheNonmagicalEntries(t *testing.T) {
	t.Parallel()
	srd, err := rules.LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD: %v", err)
	}
	physical := []string{"damage-type:bludgeoning", "damage-type:piercing", "damage-type:slashing"}
	ghost, _ := srd.CreatureByKey("monster:ghost")
	got := typeModifiersOf(ghost, "monster:ghost")
	if !slices.Equal(got.ResistantNonmagical, physical) {
		t.Errorf("ghost: ResistantNonmagical = %v, want %v", got.ResistantNonmagical, physical)
	}
	if slices.Contains(got.Resistant, "damage-type:slashing") {
		t.Errorf("ghost: a nonmagical entry is in the plain resistances: %v", got.Resistant)
	}
	if !slices.Contains(got.Resistant, "damage-type:fire") || !slices.Contains(got.Immune, "damage-type:poison") {
		t.Errorf("ghost: plain entries lost: resistant %v immune %v", got.Resistant, got.Immune)
	}
	if got.Source != "monster:ghost" {
		t.Errorf("ghost: Source = %q", got.Source)
	}
	if len(got.ResistantUnlessSilvered)+len(got.ImmuneUnlessAdamantine) != 0 {
		t.Errorf("ghost: a nonmagical entry leaked into the material lists: %+v", got)
	}
}

// The entries the SRD limits by a material are carried under their own lists.
func TestTypeModifiersCarryTheMaterialEntries(t *testing.T) {
	t.Parallel()
	srd, err := rules.LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD: %v", err)
	}
	physical := []string{"damage-type:bludgeoning", "damage-type:piercing", "damage-type:slashing"}
	tests := []struct {
		key  string
		pick func(combat.TypeModifiers) []string
		want []string
	}{
		{"monster:gargoyle", func(m combat.TypeModifiers) []string { return m.ResistantUnlessAdamantine }, physical},
		{"monster:werewolf-wolf", func(m combat.TypeModifiers) []string { return m.ImmuneUnlessSilvered }, physical},
		{"monster:wight", func(m combat.TypeModifiers) []string { return m.ResistantUnlessSilvered }, physical},
		{"monster:iron-golem", func(m combat.TypeModifiers) []string { return m.ImmuneUnlessAdamantine }, physical},
	}
	for _, tt := range tests {
		c, ok := srd.CreatureByKey(tt.key)
		if !ok {
			t.Fatalf("%s is not in the SRD", tt.key)
		}
		m := typeModifiersOf(c, tt.key)
		if got := tt.pick(m); !slices.Equal(got, tt.want) {
			t.Errorf("%s: material list = %v, want %v (%+v)", tt.key, got, tt.want, m)
		}
		if slices.Contains(m.Resistant, "damage-type:slashing") || slices.Contains(m.Immune, "damage-type:slashing") || len(m.ResistantNonmagical)+len(m.ImmuneNonmagical) != 0 {
			t.Errorf("%s: a material entry is in a plain or nonmagical list: %+v", tt.key, m)
		}
	}
}

// A sheet's spell damage is magical; a weapon and the unarmed strike are not unless the
// sheet says so.
func TestCombatAttackMagical(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   rules.Attack
		want bool
	}{
		{"a cantrip", rules.Attack{Kind: "spell", Magical: true}, true},
		{"a weapon", rules.Attack{Kind: "weapon"}, false},
		{"the unarmed strike", rules.Attack{Kind: "weapon", Key: rules.UnarmedStrikeKey}, false},
		{"a magical weapon", rules.Attack{Kind: "weapon", Magical: true}, true},
	}
	for _, tt := range cases {
		if got := combatAttack(tt.in).Magical; got != tt.want {
			t.Errorf("%s: Magical = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// The weapon's material goes from the sheet's attack to the combat's.
func TestCombatAttackCarriesTheMaterial(t *testing.T) {
	t.Parallel()
	got := combatAttack(rules.Attack{Kind: "weapon", Silvered: true})
	if !got.Silvered || got.Adamantine {
		t.Errorf("silvered: %+v", got)
	}
	got = combatAttack(rules.Attack{Kind: "weapon", Adamantine: true})
	if got.Silvered || !got.Adamantine {
		t.Errorf("adamantine: %+v", got)
	}
}
