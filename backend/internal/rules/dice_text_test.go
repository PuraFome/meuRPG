package rules

import "testing"

func TestDiceFormulaText(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		d    DiceFormula
		want string
	}{
		{DiceFormula{Count: 1, Sides: 6, Bonus: 1}, "1d6+1"},
		{DiceFormula{Count: 1, Sides: 4, Bonus: -1}, "1d4-1"},
		{DiceFormula{Count: 2, Sides: 8}, "2d8"},
		{DiceFormula{}, ""},
	} {
		if got := c.d.Text(); got != c.want {
			t.Errorf("%+v = %q, want %q", c.d, got, c.want)
		}
	}
}
