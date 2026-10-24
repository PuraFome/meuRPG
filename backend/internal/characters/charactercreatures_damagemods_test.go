package characters

import (
	"slices"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// What play reads of a stat block's damage lists: the entries the
// SRD limits to nonmagical attacks are carried with that limit, not dropped; the ones
// limited by a material (silvered, adamantine) are left to the master.
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
	// A limit by material is the master's: nothing of it reaches the engine.
	for _, key := range []string{"monster:gargoyle", "monster:werewolf-wolf"} {
		c, _ := srd.CreatureByKey(key)
		m := typeModifiersOf(c, key)
		if len(m.ResistantNonmagical) != 0 || len(m.ImmuneNonmagical) != 0 || slices.Contains(m.Resistant, "damage-type:slashing") || slices.Contains(m.Immune, "damage-type:slashing") {
			t.Errorf("%s: an entry limited by a material was carried: %+v", key, m)
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
