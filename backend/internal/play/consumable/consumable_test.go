package consumable

import (
	"errors"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

func TestTotalAddsTheFacesAndTheBonus(t *testing.T) {
	t.Parallel()
	f := rules.DiceFormula{Count: 2, Sides: 4, Bonus: 2}
	for name, tc := range map[string]struct {
		faces []int32
		want  int32
		err   bool
	}{
		"low":         {[]int32{1, 1}, 4, false},
		"high":        {[]int32{4, 4}, 10, false},
		"too few":     {[]int32{3}, 0, true},
		"too many":    {[]int32{1, 1, 1}, 0, true},
		"a zero":      {[]int32{0, 2}, 0, true},
		"off the die": {[]int32{5, 1}, 0, true},
	} {
		got, err := Total(f, tc.faces)
		if (err != nil) != tc.err || got != tc.want {
			t.Errorf("%s: Total(%v) = %d, %v; want %d, error %v", name, tc.faces, got, err, tc.want, tc.err)
		}
		if tc.err && !errors.Is(err, ErrFaces) {
			t.Errorf("%s: error %v is not ErrFaces", name, err)
		}
	}
}

func TestResistanceIsOneHourOfResistanceToTheType(t *testing.T) {
	t.Parallel()
	p := rules.PotionEffect{Kind: rules.PotionResistance, DamageType: "damage-type:fire", Rounds: rules.PotionResistanceRounds}
	m := Resistance(p)
	if m.Kind != rules.ModifierResistance || m.ResistanceKind != rules.ResistanceKindResist || len(m.DamageTypes) != 1 || m.DamageTypes[0] != "damage-type:fire" {
		t.Errorf("Resistance() = %+v", m)
	}
	if got := Seconds(p); got != 3600 {
		t.Errorf("Seconds() = %d, want 3600", got)
	}
}

func TestTextPrefersThePortugueseOne(t *testing.T) {
	t.Parallel()
	if got := Text(rules.MagicItem{Desc: []string{"en"}, DescPT: []string{"pt"}}); len(got) != 1 || got[0] != "pt" {
		t.Errorf("Text() = %v, want pt", got)
	}
	if got := Text(rules.MagicItem{Desc: []string{"en"}}); len(got) != 1 || got[0] != "en" {
		t.Errorf("Text() = %v, want en", got)
	}
}
