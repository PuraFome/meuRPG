package combat

import "testing"

// Magic Resistance (SRD 5.1): advantage on saving throws against spells and other magical effects,
// and on nothing else.
func TestMagicResistanceGivesAdvantageOnSavesAgainstMagicOnly(t *testing.T) {
	t.Parallel()
	resistant := Creature{MagicResistance: true}
	for _, tc := range []struct {
		name  string
		scene SaveScene
		want  RollMode
	}{
		{"a save against a spell", SaveScene{Creature: resistant, Ability: "dex", Against: []string{AgainstMagic}}, ModeAdvantage},
		{"a save against a spell that also charms", SaveScene{Creature: resistant, Ability: "wis", Against: []string{AgainstMagic, AgainstCharmed}}, ModeAdvantage},
		{"a save against something not magical", SaveScene{Creature: resistant, Ability: "dex", Against: []string{AgainstPoison}}, ModeNormal},
		{"a save with no known source", SaveScene{Creature: resistant, Ability: "dex"}, ModeNormal},
		{"an ability check", SaveScene{Creature: resistant, Ability: "dex", Check: true, Against: []string{AgainstMagic}}, ModeNormal},
		{"a creature without the trait", SaveScene{Creature: Creature{}, Ability: "dex", Against: []string{AgainstMagic}}, ModeNormal},
		{"disadvantage cancels it", SaveScene{Creature: Creature{MagicResistance: true, Conditions: []string{conditionRestrained}}, Ability: "dex", Against: []string{AgainstMagic}}, ModeNormal},
	} {
		if got := Resolve(SaveMode(tc.scene)); got != tc.want {
			t.Errorf("%s: mode = %v, want %v", tc.name, got, tc.want)
		}
	}
	var kinds []string
	for _, s := range SaveMode(SaveScene{Creature: resistant, Ability: "dex", Against: []string{AgainstMagic}}) {
		kinds = append(kinds, s.Kind)
	}
	if len(kinds) != 1 || kinds[0] != SourceMagicResistance {
		t.Errorf("sources = %v, want only %s", kinds, SourceMagicResistance)
	}
}
